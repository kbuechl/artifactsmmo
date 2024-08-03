package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"net/http"
)

type MapContentType int

const (
	MonsterMapContentType MapContentType = iota
	ResourceMapContentType
	WorkshopMapContentType
	BankMapContentType
	TaskMasterContentType
	GrandExchangeContentType
)

func (m MapContentType) String() string {
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
		return "task_master"
	case GrandExchangeContentType:
		return "grand_exchange"
	default:
		return ""
	}
}

type MapTile struct {
	X    int
	Y    int
	Type string
	Code string
}

func (w *WorldDataCollector) updateMap(ctx context.Context) ([]MapTile, error) {
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
			return nil, fmt.Errorf("error fetching map resources: %w", resp.Status())
		}
		data = append(data, resp.JSON200.Data...)

		if p, pErr := resp.JSON200.Pages.AsDataPageMapSchemaPages0(); pErr != nil {
			return nil, err
		} else if p == page {
			break
		}
	}

	resources := make([]MapTile, 0, len(data))

	for _, d := range data {
		if contentMap, cErr := d.Content.AsMapContentSchema(); cErr == nil {
			resources = append(resources, MapTile{
				X:    d.X,
				Y:    d.Y,
				Type: contentMap.Type,
				Code: contentMap.Code,
			})
		}
	}
	return resources, nil
}
