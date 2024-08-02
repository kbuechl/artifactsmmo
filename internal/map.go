package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
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

type MapData struct {
	X    int
	Y    int
	Type string
	Code string
}

func (w *WorldDataCollector) updateMap(ctx context.Context) ([]MapData, error) {
	ct := client.GetAllMapsMapsGetParamsContentType("resource")
	resp, err := w.client.GetAllMapsMapsGetWithResponse(ctx, &client.GetAllMapsMapsGetParams{
		//todo: this is paginated we need to loop through all the pages instead of using resources only
		ContentType: &ct,
		ContentCode: nil,
		Page:        nil,
		Size:        nil,
	})

	if err != nil {
		return nil, fmt.Errorf("error fetching map resources: %w", err)
	}

	resources := make([]MapData, 0, len(resp.JSON200.Data))

	for _, d := range resp.JSON200.Data {
		if contentMap, ok := d.Content.(map[string]interface{}); ok {
			// Extract values for "type" and "code"
			contentType, typeOk := contentMap["type"].(string)
			contentCode, codeOk := contentMap["code"].(string)

			if typeOk && codeOk {
				// Append to resources
				resources = append(resources, MapData{
					X:    d.X,
					Y:    d.Y,
					Type: contentType,
					Code: contentCode,
				})
			}
		}
	}
	return resources, nil
}
