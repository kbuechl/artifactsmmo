package world

import (
	"artifactsmmo/internal/models"
	"artifactsmmo/internal/player"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"net/http"
)

func (w *Collector) loadMonsters() error {
	data := make([]models.Monster, 0)

	for page := 1; ; page++ {
		resp, err := w.client.GetAllMonstersMonstersGetWithResponse(w.ctx, &client.GetAllMonstersMonstersGetParams{
			MinLevel: nil,
			MaxLevel: nil,
			Drop:     nil,
			Page:     nil,
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
		} else if p == page {
			break
		}
	}

	w.Monsters = data
	return nil
}

// FilterMonsters gets a slice of monsters the player can kill
func (w *Collector) FilterMonsters(p *player.Player) []models.Monster {
	//todo: later we can select the right weapons and stuff for now lets use what we have
	var skill models.AttackType
	skillDmg := 0
	for s, dmg := range p.Data().AttackStats {
		if dmg > skillDmg {
			skill = s
			skillDmg = dmg
		}
	}

	w.logger.Debug("top skill for monster filter", skill, skillDmg)

	monsters := make([]models.Monster, 0)
	for _, m := range w.Monsters {
		if p.CanWinFight(skill, m) {
			monsters = append(monsters, m)
		}
	}

	return monsters
}

func (w *Collector) GetMonster(name string) *models.Monster {
	for _, m := range w.Monsters {
		if m.Name == name {
			return &m
		}
	}
	return nil
}
