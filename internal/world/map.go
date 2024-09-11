package world

import (
	"artifactsmmo/internal/models"
	"context"
	"fmt"
	"math"
	"net/http"

	"github.com/promiseofcake/artifactsmmo-go-client/client"
)

type mapContentType int

const (
	MonsterMapContentType mapContentType = iota
	ResourceMapContentType
	WorkshopMapContentType
	BankMapContentType
	TaskMasterContentType
	GrandExchangeContentType
)

func (m mapContentType) String() string {
	switch m {
	case MonsterMapContentType:
		return "monster"
	case ResourceMapContentType:
		return "resource"
	case WorkshopMapContentType:
		return "workshop"
	case BankMapContentType:
		return "bank"
	case TaskMasterContentType:
		return "tasks_master"
	case GrandExchangeContentType:
		return "grand_exchange"
	default:
		return ""
	}
}

func (w *Collector) updateMap(ctx context.Context) ([]models.MapTile, error) {
	size := 100
	data := make([]client.MapSchema, 0)

	for page := 1; ; page++ {
		resp, err := w.client.GetAllMapsMapsGetWithResponse(ctx, &client.GetAllMapsMapsGetParams{
			ContentType: nil,
			ContentCode: nil,
			Page:        &page,
			Size:        &size,
		})

		if err != nil {
			return nil, fmt.Errorf("error fetching map resources: %w", err)
		}
		if resp.StatusCode() != http.StatusOK {
			return nil, fmt.Errorf("error fetching map resources: %s", resp.Status())
		}
		data = append(data, resp.JSON200.Data...)

		if p, pErr := resp.JSON200.Pages.AsDataPageMapSchemaPages0(); pErr != nil {
			return nil, err
		} else if page >= p {
			break
		}
	}

	resources := make([]models.MapTile, 0, len(data))

	for _, d := range data {
		if contentMap, cErr := d.Content.AsMapContentSchema(); cErr == nil {
			resources = append(resources, models.MapTile{
				X:    d.X,
				Y:    d.Y,
				Type: contentMap.Type,
				Code: contentMap.Code,
			})
		}
	}
	return resources, nil
}

func (w *Collector) MapTiles() []models.MapTile {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.tiles
}

func (w *Collector) loadMapTiles() error {
	w.logger.Info("Loading Map")
	resp, err := w.updateMap(w.ctx)
	if err != nil {
		return fmt.Errorf("get all resources: %w", err)
	}
	w.mu.Lock()
	w.tiles = resp
	w.mu.Unlock()
	return nil
}

func (w *Collector) FindClosestTile(code string, x int, y int) *models.MapTile {
	var closest *models.MapTile
	distance := math.MaxInt
	for _, t := range w.MapTiles() {
		if t.Code == code {
			d := getDistance(x, y, t.X, t.Y)
			if d < distance {
				closest = &t
				distance = d
			}
		}
	}

	return closest
}

func getDistance(x1, y1, x2, y2 int) int {
	return int(math.Abs(float64(x1)-float64(x2)) + math.Abs(float64(y1)-float64(y2)))
}
