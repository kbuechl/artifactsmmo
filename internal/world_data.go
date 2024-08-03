package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"sync"
	"time"
)

const (
	worldRefreshInterval time.Duration = time.Second * 1
)

type WorldDataCollector struct {
	Resources ResourceMap
	tiles     []MapTile
	mu        sync.RWMutex
	ctx       context.Context
	client    *client.ClientWithResponses
	Out       chan error
}

func NewWorldCollector(ctx context.Context, client *client.ClientWithResponses) (*WorldDataCollector, error) {
	collector := &WorldDataCollector{
		ctx:    ctx,
		client: client,
	}

	rData, err := collector.getAllResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all resources: %w", err)
	}
	collector.Resources = rData

	err = collector.updateWorldData()
	if err != nil {
		return nil, err
	}

	collector.start()
	return collector, nil
}

// start will start checking world data on some interval
func (w *WorldDataCollector) start() {
	//collect world data
	go func() {
		ticker := time.NewTicker(worldRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				if err := w.updateWorldData(); err != nil {
					//todo: if we get timed out we should just just continue
					w.Out <- err
				}
			}
		}
	}()
}

func (w *WorldDataCollector) MapTiles() []MapTile {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.tiles
}

func (w *WorldDataCollector) updateWorldData() error {
	resp, err := w.updateMap(w.ctx)
	if err != nil {
		return fmt.Errorf("get all resources: %w", err)
	}
	w.mu.Lock()
	w.tiles = resp
	w.mu.Unlock()
	return nil
}

func (w *WorldDataCollector) GetGatherableMapSections(playerSkills map[string]int) []MapTile {
	var mapData []MapTile
	resourceTypeString := ResourceMapContentType.String()

	for _, m := range w.MapTiles() {
		if m.Type != resourceTypeString {
			continue
		}
		resourceData, foundResource := w.Resources[m.Code]
		if !foundResource {
			continue
		}
		playerLevel, foundPlayer := playerSkills[resourceData.Skill]
		if !foundPlayer || playerLevel < resourceData.Level {
			continue
		}
		mapData = append(mapData, m)
	}

	return mapData
}

func (w *WorldDataCollector) GetResourceByName(name string) (*Resource, error) {
	if r, ok := w.Resources[name]; ok {
		return &r, nil
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

func (w *WorldDataCollector) GetMapByContentType(contentType MapContentType) []MapTile {
	cString := contentType.String()

	res := make([]MapTile, 0)
	for _, m := range w.MapTiles() {
		if m.Type == cString {
			res = append(res, m)
		}
	}
	return res
}
