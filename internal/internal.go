package internal

import "github.com/promiseofcake/artifactsmmo-cli/client"

func newCooldown(c client.CooldownSchema) Cooldown {
	return Cooldown{
		Seconds:    c.RemainingSeconds,
		Expiration: c.Expiration,
	}
}

func convertMapInterfaceToGeneric(d any) GenericContent {
	if m, ok := d.(map[string]interface{}); ok {
		return GenericContent{
			Type: m["type"].(string),
			Code: m["code"].(string),
		}
	} else {
		return GenericContent{}
	}
}
