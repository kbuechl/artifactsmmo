package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
	"sync"
	"time"
)

const (
	characterRefreshInterval time.Duration = time.Second * 5
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

type Player struct {
	Name   string
	data   PlayerStatus
	mu     sync.RWMutex
	ctx    context.Context
	client *client.ClientWithResponses
	Out    chan error
}

type PlayerPosition struct {
	X int
	Y int
}
type PlayerStatus struct {
	Cooldown int
	Stamina  int
	Hp       int
	Skills   map[string]int
	Pos      PlayerPosition
}

type GenericContent struct {
	Type string
	Code string
}

type Cooldown struct {
	Seconds    int
	Expiration time.Time
}

type MoveResponse struct {
	Cooldown
	Content GenericContent
}

func NewPlayer(ctx context.Context, name string, client *client.ClientWithResponses) (*Player, error) {
	p := &Player{
		Name:   name,
		client: client,
		ctx:    ctx,
	}

	if err := p.updateStatus(); err != nil {
		return nil, err
	}

	p.start()

	return p, nil
}

func (p *Player) start() {
	go func() {
		ticker := time.NewTicker(characterRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if err := p.updateStatus(); err != nil {
					//todo: if we get timed out we should just just continue
					p.Out <- err
				}
			}
		}
	}()
}

func (p *Player) CharacterData() PlayerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data
}

func (p *Player) CanGather(resource Resource) bool {
	for skill, level := range p.data.Skills {
		if skill == resource.Skill && resource.Level <= level {
			return true
		}
	}
	return false
}

func (p *Player) Move(x, y int) (*MoveResponse, error) {
	curX, curY := p.CurrentPos()
	if curX == x && curY == y {
		//todo: return data from map data for this spot?
		fmt.Println("character already at position")
		return &MoveResponse{
			Cooldown: Cooldown{
				Seconds: 0,
			},
			Content: GenericContent{},
		}, nil
	}

	resp, err := p.client.ActionMoveMyNameActionMovePostWithResponse(p.ctx, p.Name, client.ActionMoveMyNameActionMovePostJSONRequestBody{
		X: x,
		Y: y,
	})

	if err != nil {
		return nil, err
	}

	if resp.HTTPResponse.StatusCode != 200 {
		return nil, fmt.Errorf("action move returned status code %v", resp.HTTPResponse.StatusCode)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.data.Pos = PlayerPosition{
		X: x,
		Y: y,
	}

	return &MoveResponse{
		Cooldown: newCooldown(resp.JSON200.Data.Cooldown),
		Content:  convertMapInterfaceToGeneric(resp.JSON200.Data.Destination.Content),
	}, nil
}

func (p *Player) Gather() (*Cooldown, error) {
	resp, err := p.client.ActionGatheringMyNameActionGatheringPostWithResponse(p.ctx, p.Name)
	if err != nil {
		return nil, err
	}
	cd := newCooldown(resp.JSON200.Data.Cooldown)
	return &cd, nil
}

func (p *Player) CurrentPos() (x, y int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data.Pos.X, p.data.Pos.Y
}

func (p *Player) updateStatus() error {
	resp, err := p.client.GetCharacterCharactersNameGetWithResponse(p.ctx, p.Name)
	if err != nil {
		return fmt.Errorf("get character status: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.data = PlayerStatus{
		Cooldown: resp.JSON200.Data.Cooldown,
		Hp:       resp.JSON200.Data.Hp,
		Stamina:  resp.JSON200.Data.Stamina,
		Pos: PlayerPosition{
			X: resp.JSON200.Data.X,
			Y: resp.JSON200.Data.Y,
		},
		Skills: map[string]int{
			WoodcuttingSkill:      resp.JSON200.Data.WoodcuttingLevel,
			FishingSkill:          resp.JSON200.Data.FishingLevel,
			MiningSkill:           resp.JSON200.Data.MiningLevel,
			GearcraftingSkill:     resp.JSON200.Data.GearcraftingLevel,
			CookingSkill:          resp.JSON200.Data.CookingLevel,
			WeaponCraftingSkill:   resp.JSON200.Data.WeaponcraftingLevel,
			JeweleryCraftingSkill: resp.JSON200.Data.JewelrycraftingLevel,
		},
	}

	return nil
}
