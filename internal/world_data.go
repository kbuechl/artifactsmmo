package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
	"sync"
	"time"
)

const (
	worldRefreshInterval time.Duration = time.Second * 2
)

type WorldDataCollector struct {
	Resources ResourceMap
	mapData   []MapData
	mapMu     sync.RWMutex
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

func (w *WorldDataCollector) MapData() []MapData {
	w.mapMu.RLock()
	defer w.mapMu.RUnlock()
	return w.mapData
}

func (w *WorldDataCollector) updateWorldData() error {
	resp, err := w.updateMap(w.ctx)
	if err != nil {
		return fmt.Errorf("get all resources: %w", err)
	}
	w.mapMu.Lock()
	w.mapData = resp
	w.mapMu.Unlock()
	return nil
}

func (w *WorldDataCollector) GetGatherableMapSections(playerSkills map[string]int) []MapData {
	var mapData []MapData
	resourceTypeString := ResourceMapContentType.String()

	for _, m := range w.MapData() {
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
