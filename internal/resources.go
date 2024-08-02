package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
)

type ResourceMap map[string]Resource
type ResourceResponse struct {
	Data []Resource `json:"data"`
}
type Resource struct {
	Skill string          `json:"skill"`
	Name  string          `json:"name"`
	Code  string          `json:"code"`
	Level int             `json:"level"`
	Drops []ResourceDrops `json:"drops"`
}

type ResourceDrops struct {
	Code string `json:"code"`
	Rate int    `json:"rate"`
	//min and max quantity
}

func (w *WorldDataCollector) getAllResources(ctx context.Context) (ResourceMap, error) {
	resp, err := w.client.GetAllResourcesResourcesGetWithResponse(ctx, &client.GetAllResourcesResourcesGetParams{
		MinLevel: nil,
		MaxLevel: nil,
		Skill:    nil,
		Drop:     nil,
		Page:     nil,
		Size:     nil,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting all resources: %w", err)
	}
	var rr ResourceResponse
	if err := json.Unmarshal(resp.Body, &rr); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	result := make(ResourceMap)
	for _, d := range rr.Data {
		result[d.Code] = d
	}

	return result, nil
}
