package engine

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/player"

	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"golang.org/x/exp/slices"
)

//given a single item, find out what it needs to be crafted
//generate commands to gather the right resources (check bank too)
//generate commands to craft the item

func (e *GameEngine) CraftItem(item client.ItemSchema, p *player.Player) {
	c, err := item.Craft.AsCraftSchema()
	steps := make([]commands.Step, 0)

	if err != nil {
		e.logger.Error("error getting craft schema", "error", err)
		return
	}
	for _, i := range *c.Items {
		needed := i.Quantity
		bIdx := slices.IndexFunc(e.world.BankItems, func(a client.SimpleItemSchema) bool {
			return a.Code == i.Code
		})

		if bIdx != -1 {
			needed -= e.world.BankItems[bIdx].Quantity
			commands.new
		}
		if needed > 0 {
			x, y := p.Pos()
			tile := e.world.FindClosestTile(i.Code, x, y)
			steps = append(steps, commands.NewGatherStep(needed, *tile))
		}

	}
	//generate commands to gather the right resources (check bank too)
	//generate commands to craft the item
}
