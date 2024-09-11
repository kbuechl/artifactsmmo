package engine

import (
	"artifactsmmo/internal/models"
	"artifactsmmo/internal/player"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/promiseofcake/artifactsmmo-go-client/client"
)

const maxFightRounds = 100

func (e *GameEngine) findMinGear(m models.Monster, p *player.Player) ([]client.ItemSchema, error) {
	//todo: we should check if we even need to get gear first

	//find min combo that can win the fight
	gearMap := map[string][]client.ItemSchema{}

	slices.SortFunc(e.world.Items, func(a, b client.ItemSchema) int {
		return a.Level - b.Level
	})

	for _, i := range e.world.Items {
		if i.Type == "consumable" || i.Type == "resource" || i.Type == "currency" || i.Craft == nil {
			continue
		}
		craft, err := i.Craft.AsCraftSchema()
		if err != nil {
			return nil, err
		}
		if *craft.Level > p.Data().Skills[string(*craft.Skill)] {
			continue
		}

		if _, ok := gearMap[i.Type]; !ok {
			gearMap[i.Type] = []client.ItemSchema{i}
		} else {
			gearMap[i.Type] = append(gearMap[i.Type], i)
		}

	}

	//go through each slot and find the first thing that can help us win. If we can win without other slots we should just return the item
	gear := e.checkGear(gearMap, m, p)
	if gear == nil {
		e.logger.Info("no gear found for %s", p.Name)
		return nil, nil
	}

	e.logger.Info("gear determined for fight against monster", "monster", m.Name, "gear", gear)
	return gear, nil
}

// returns nil if no combo works
func (e *GameEngine) checkGear(gearMap map[string][]client.ItemSchema, m models.Monster, p *player.Player) []client.ItemSchema {
	//todo: can we check if we can get away just the weapon?

	// Prepare to test weapons with full armor
	helmets := gearMap["helmet"]
	shields := gearMap["shield"]
	boots := gearMap["boots"]

	var minGear []client.ItemSchema
	//we are assuming gear is ordered by lowest level gear first
	// Test each weapon combined with all possible full armor sets
	for _, weapon := range gearMap["weapon"] {
		for _, helmet := range helmets {
			for _, shield := range shields {
				for _, boot := range boots {
					combination := []client.ItemSchema{weapon, helmet, shield, boot}
					if canWinFight(combination, m, p) {
						return combination
					}
				}
			}
		}
	}

	if len(minGear) == 0 {
		fmt.Println("no gear combination found that can win the fight")
	}
	return nil
}

func (e *GameEngine) canPlayerWinFight(m models.Monster, p *player.Player) bool {
	eq := make([]client.ItemSchema, 0)

	for _, code := range p.Data().Equipment {
		for _, i := range e.world.Items {
			if i.Code == code {
				eq = append(eq, i)
			}
		}
	}

	return canWinFight(eq, m, p)
}

func canWinFight(eq []client.ItemSchema, m models.Monster, p *player.Player) bool {
	attack := map[models.AttackType]int{models.Earth: 0, models.Air: 0, models.Water: 0, models.Fire: 0}
	dmg := map[models.AttackType]int{}
	res := map[models.AttackType]int{}
	playerHpBonus := 0
	for _, e := range eq {
		for _, effect := range *e.Effects {
			if effect.Name == "hp" {
				playerHpBonus += effect.Value
			} else if effect.Name == "haste" {
				//todo: calculate haste
				continue
			} else {
				s := strings.Split(effect.Name, "_")
				attackType := models.AttackType(s[1])
				switch s[0] {
				case "attack":
					attack[attackType] += effect.Value
				case "dmg":
					dmg[attackType] += effect.Value
				case "res":
					res[attackType] += effect.Value
				default:
					fmt.Println("unknown effect: %s", s[0])
				}
			}
		}
	}

	playerDmg := 0
	monsterDmg := 0
	for attackType, damage := range attack {
		modifiedDmg := calculateElementDamage(damage, dmg[attackType])
		playerDmg += calculateAttackDamage(modifiedDmg, m.Resistances[attackType])
		mElement := calculateElementDamage(m.Attacks[attackType], p.Data().DefenseStats[attackType])
		monsterDmg += calculateAttackDamage(mElement, p.Data().DefenseStats[attackType])
	}
	// Calculate how many turns each entity can take
	playerTurns := float64(m.Hp) / float64(playerDmg)
	monsterTurns := float64(p.Data().Hp+playerHpBonus) / float64(monsterDmg)

	// Determine the outcome
	if playerTurns >= float64(maxFightRounds) || playerTurns >= monsterTurns {
		return false
	}
	return true
}

const dmgModifier = .1

func calculateElementDamage(attackBase int, dmg int) int {
	return int(math.Floor(float64(attackBase) + (float64(dmg) * dmgModifier)))
}

func calculateAttackDamage(attack int, resistance int) int {
	damageTaken := float64(attack) * (1 - float64(resistance)*0.01)
	return int(math.Round(damageTaken))
}
