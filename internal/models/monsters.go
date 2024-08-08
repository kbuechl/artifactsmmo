package models

import (
	"github.com/promiseofcake/artifactsmmo-go-client/client"
)

type Monster struct {
	Name        string
	Code        string
	Level       int
	Hp          int
	AttackType  AttackType
	AttackDmg   int
	Resistances map[AttackType]int
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
		Resistances: map[AttackType]int{
			Fire:  monster.ResFire,
			Water: monster.ResWater,
			Earth: monster.ResEarth,
			Air:   monster.ResAir,
		},
	}

	switch {
	case monster.AttackWater != 0:
		m.AttackType = Water
		m.AttackDmg = monster.AttackWater
	case monster.AttackFire != 0:
		m.AttackType = Fire
		m.AttackDmg = monster.AttackFire
	case monster.AttackAir != 0:
		m.AttackType = Air
		m.AttackDmg = monster.AttackAir
	case monster.AttackEarth != 0:
		m.AttackType = Earth
		m.AttackDmg = monster.AttackEarth
	}

	return m
}
