package internal

import (
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"time"
)

func waitForCooldown(cooldown client.CooldownSchema) {
	time.Sleep(cooldown.Expiration.Sub(time.Now()))
}

func waitForCooldownSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
