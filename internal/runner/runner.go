package runner

import (
	"context"
	"github.com/promiseofcake/artifactsmmo-cli/client"
	"time"
)

const (
	WoodcuttingSkill      = "woodcutting"
	MiningSkill           = "mining"
	FishingSkill          = "fishing"
	WeaponCraftingSkill   = "weaponcrafting"
	JeweleryCraftingSkill = "jewelerycrafting"
	CookingSkill          = "cooking"
	GearcraftingSkill     = "gearcrafting"
)

type MapContentType int

type Runner struct {
	Client *client.ClientWithResponses
}

type CharacterStatus struct {
	Cooldown int
	Stamina  int
	Hp       int
	Skills   map[string]int
}
type MoveResponse struct {
	*Cooldown
	Content any
}

type Cooldown struct {
	Seconds    int
	Expiration time.Time
}

type GenericContent struct {
	Type string
	Code string
}

func newCooldown(c client.CooldownSchema) *Cooldown {
	return &Cooldown{
		Seconds:    c.RemainingSeconds,
		Expiration: c.Expiration,
	}
}

func (r *Runner) GetPlayerStatus(ctx context.Context, name string) (*CharacterStatus, error) {
	resp, err := r.Client.GetCharacterCharactersNameGetWithResponse(ctx, name)
	if err != nil {
		return nil, err
	}

	return &CharacterStatus{
		Cooldown: resp.JSON200.Data.Cooldown,
		Hp:       resp.JSON200.Data.Hp,
		Stamina:  resp.JSON200.Data.Stamina,
		Skills: map[string]int{
			WoodcuttingSkill:      resp.JSON200.Data.WoodcuttingLevel,
			FishingSkill:          resp.JSON200.Data.FishingLevel,
			MiningSkill:           resp.JSON200.Data.MiningLevel,
			GearcraftingSkill:     resp.JSON200.Data.GearcraftingLevel,
			CookingSkill:          resp.JSON200.Data.CookingLevel,
			WeaponCraftingSkill:   resp.JSON200.Data.WeaponcraftingLevel,
			JeweleryCraftingSkill: resp.JSON200.Data.JewelrycraftingLevel,
		},
	}, nil

}
func (r *Runner) Move(ctx context.Context, name string, x int, y int) (*MoveResponse, error) {
	resp, err := r.Client.ActionMoveMyNameActionMovePostWithResponse(ctx, name, client.ActionMoveMyNameActionMovePostJSONRequestBody{
		X: x,
		Y: y,
	})
	if err != nil {
		return nil, err
	}

	return &MoveResponse{
		Cooldown: newCooldown(resp.JSON200.Data.Cooldown),
		Content:  resp.JSON200.Data.Destination.Content,
	}, nil
}

func (r *Runner) Gather(ctx context.Context, name string) (*Cooldown, error) {
	resp, err := r.Client.ActionGatheringMyNameActionGatheringPostWithResponse(ctx, name)
	if err != nil {
		return nil, err
	}
	return newCooldown(resp.JSON200.Data.Cooldown), nil
}
