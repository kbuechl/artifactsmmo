package player

import "math"

const dmgModifier = .1

func calculateElementDamage(attackBase int, dmg int) int {
	return int(math.Floor(float64(attackBase) + (float64(dmg) * dmgModifier)))
}

func calculateAttackDamage(attack int, resistance int) int {
	damageTaken := float64(attack) * (1 - float64(resistance)*0.01)
	return int(math.Round(damageTaken))
}
