package internal

import (
	"artifactsmmo/internal/runner"
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	worldRefreshInterval time.Duration = time.Second * 2
)

type WorldDataCollector struct {
	Resources runner.ResourceMap
	mapData   []runner.MapData
	mapMu     sync.RWMutex
	ctx       context.Context
	client    *runner.Runner
	Out       chan error
}

func NewWorldCollector(ctx context.Context, client *runner.Runner) (*WorldDataCollector, error) {
	rData, err := client.GetAllResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all resources: %w", err)
	}

	collector := &WorldDataCollector{
		Resources: rData,
		ctx:       ctx,
		client:    client,
	}

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

func (w *WorldDataCollector) MapData() []runner.MapData {
	w.mapMu.RLock()
	defer w.mapMu.RUnlock()
	return w.mapData
}

func (w *WorldDataCollector) updateWorldData() error {
	resp, err := w.client.GetMap(w.ctx)
	if err != nil {
		return fmt.Errorf("get all resources: %w", err)
	}
	w.mapMu.Lock()
	w.mapData = resp
	w.mapMu.Unlock()
	return nil
}

func (w *WorldDataCollector) GetGatherableMapSections(playerSkills map[string]int) []runner.MapData {
	var mapData []runner.MapData
	resourceTypeString := runner.ResourceMapContentType.String()

	for _, m := range w.MapData() {
		if m.Type != resourceTypeString {
			continue
		}
		resourceData, foundResource := w.Resources[m.Code]
		if !foundResource {
			continue
		}
		playerLevel, foundPlayer := playerSkills[resourceData.Code]
		if !foundPlayer || playerLevel < resourceData.Level {
			continue
		}
		mapData = append(mapData, m)
	}

	return mapData
}
