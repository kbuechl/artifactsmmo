package world

import (
	"artifactsmmo/internal/player"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"net/http"
)

type Monster struct {
	Name        string
	Code        string
	Level       int
	Hp          int
	AttackType  player.AttackType
	AttackDmg   int
	Resistances map[player.AttackType]int
	MinGold     int
	MaxGold     int
	Drops       []client.DropRateSchema
}

func MonsterFromSchema(monster client.MonsterSchema) Monster {
	m := Monster{
		Name:    monster.Name,
		Code:    monster.Code,
		Level:   monster.Level,
		Hp:      monster.Hp,
		MinGold: monster.MinGold,
		MaxGold: monster.MaxGold,
		Drops:   monster.Drops,
		Resistances: map[player.AttackType]int{
			player.Fire:  monster.ResFire,
			player.Water: monster.ResWater,
			player.Earth: monster.ResEarth,
			player.Air:   monster.ResAir,
		},
	}

	switch {
	case monster.AttackWater != 0:
		m.AttackType = player.Water
		m.AttackDmg = monster.AttackWater
	case monster.AttackFire != 0:
		m.AttackType = player.Fire
		m.AttackDmg = monster.AttackFire
	case monster.AttackAir != 0:
		m.AttackType = player.Air
		m.AttackDmg = monster.AttackAir
	case monster.AttackEarth != 0:
		m.AttackType = player.Earth
		m.AttackDmg = monster.AttackEarth
	}

	return m
}

func (w *Collector) loadMonsters() error {
	data := make([]Monster, 0)

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
			data = append(data, MonsterFromSchema(m))
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

func (w *Collector) FilterMonsters(p *player.Player) []Monster {
	//todo: later we can select the right weapons and stuff for now lets use what we have
	var skill player.AttackType
	skillDmg := 0
	for s, dmg := range p.Data().AttackStats {
		if dmg > skillDmg {
			skill = s
			skillDmg = dmg
		}
	}

	w.logger.Debug("top skill for monster filter", skill, skillDmg)

	monsters := make([]Monster, 0)
	for _, m := range w.Monsters {
		if p.CanWinFight(skill, m) {
			monsters = append(monsters, m)
		}
	}

	return monsters
}
