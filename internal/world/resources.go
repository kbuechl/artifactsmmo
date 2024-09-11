package world

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/promiseofcake/artifactsmmo-go-client/client"
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

func (w *Collector) getAllResources(ctx context.Context) (ResourceMap, error) {
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

func (w *Collector) loadItems(ctx context.Context) error {
	for page := 1; ; page++ {
		resp, err := w.client.GetAllItemsItemsGetWithResponse(ctx, &client.GetAllItemsItemsGetParams{
			Page: &page,
		})
		if err != nil {
			return err
		}
		if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("error getting items, status code: %d", resp.StatusCode())
		}
		w.mu.Lock()
		w.Items = append(w.Items, resp.JSON200.Data...)
		w.mu.Unlock()

		if p, err := resp.JSON200.Pages.AsDataPageItemSchemaPages0(); err != nil {
			return err
		} else if p >= page {
			break
		}
	}

	return nil
}
