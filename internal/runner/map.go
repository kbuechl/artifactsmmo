package runner

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
)

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

func (r *Runner) GetMap(ctx context.Context) ([]MapData, error) {
	ct := client.GetAllMapsMapsGetParamsContentType("resource")
	resp, err := r.Client.GetAllMapsMapsGetWithResponse(ctx, &client.GetAllMapsMapsGetParams{
		//todo: this is paginated we need to loop through all the pages instead of using resources only
		ContentType: &ct,
		ContentCode: nil,
		Page:        nil,
		Size:        nil,
	})

	if err != nil {
		return nil, fmt.Errorf("error fetching map resources: %w", err)
	}
	fmt.Printf("num pages: %d\n", resp.JSON200.Pages)
	resources := make([]MapData, 0, len(resp.JSON200.Data))

	for _, d := range resp.JSON200.Data {
		if content, ok := d.Content.(GenericContent); ok {
			resources = append(resources, MapData{
				X:    d.X,
				Y:    d.Y,
				Type: content.Type,
				Code: content.Code,
			})
		}
	}
	return resources, nil
}
