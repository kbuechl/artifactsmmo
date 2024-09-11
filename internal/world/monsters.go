package world

import (
	"artifactsmmo/internal/models"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"net/http"
)

func (w *Collector) loadMonsters() error {
	w.logger.Info("Loading Monsters")
	data := make([]models.Monster, 0)

	for page := 1; ; page++ {
		resp, err := w.client.GetAllMonstersMonstersGetWithResponse(w.ctx, &client.GetAllMonstersMonstersGetParams{
			MinLevel: nil,
			MaxLevel: nil,
			Drop:     nil,
			Page:     &page,
			Size:     nil,
		})
		if err != nil {
			return fmt.Errorf("get all monsters: %w", err)
		}
		if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("get all monsters: %d", resp.StatusCode())
		}
		for _, m := range resp.JSON200.Data {
			data = append(data, models.MonsterFromSchema(m))
		}

		if p, pErr := resp.JSON200.Pages.AsDataPageMonsterSchemaPages0(); pErr != nil {
			return fmt.Errorf("get all monsters: %w", pErr)
		} else if page >= p {
			break
		}
	}

	w.Monsters = data
	return nil
}

func (w *Collector) GetMonster(code string) *models.Monster {
	for _, m := range w.Monsters {
		if m.Code == code {
			return &m
		}
	}
	return nil
}
