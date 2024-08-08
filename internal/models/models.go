package models

import "github.com/promiseofcake/artifactsmmo-go-client/client"

// todo: this is hacky but temporarily fixing import cycles

const (
	WoodcuttingSkill      = "woodcutting"
	MiningSkill           = "mining"
	FishingSkill          = "fishing"
	WeaponCraftingSkill   = "weaponcrafting"
	JeweleryCraftingSkill = "jewelerycrafting"
	CookingSkill          = "cooking"
	GearcraftingSkill     = "gearcrafting"
)

type AttackType string

const (
	Air   AttackType = "air"
	Earth AttackType = "earth"
	Water AttackType = "water"
	Fire  AttackType = "fire"
)

type BankResponse struct {
	Gold  *int
	Items *[]client.SimpleItemSchema
}

type MapTile struct {
	X    int
	Y    int
	Type string
	Code string
}
