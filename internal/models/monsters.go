package models

import (
	"github.com/promiseofcake/artifactsmmo-go-client/client"
)

type Monster struct {
	Name        string
	Code        string
	Level       int
	Hp          int
	Attacks     map[AttackType]int
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
		Attacks: map[AttackType]int{
			Fire:  monster.AttackFire,
			Water: monster.AttackWater,
			Earth: monster.AttackEarth,
			Air:   monster.AttackAir,
		},
	}

	return m
}
