package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"log"
	"sync"
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
	Name       string
	data       PlayerData
	mu         sync.RWMutex
	ctx        context.Context
	client     *client.ClientWithResponses
	engineChan chan PlayerResponse
	In         chan *Command
}

type PlayerPosition struct {
	X int
	Y int
}

type PlayerData struct {
	Stamina   int
	Hp        int
	Skills    map[string]int
	Pos       PlayerPosition
	Inventory []client.InventorySlot
	Level     int
}

func NewPlayer(ctx context.Context, name string, client *client.ClientWithResponses, rc chan PlayerResponse) (*Player, error) {
	p := &Player{
		Name:       name,
		client:     client,
		ctx:        ctx,
		engineChan: rc,
		In:         make(chan *Command),
	}

	if err := p.getData(); err != nil {
		return nil, err
	}

	go p.start()

	return p, nil
}

func (p *Player) start() {
	//send initial command to tell eng online
	p.engineChan <- PlayerResponse{Name: p.Name, Code: PlayerStartedCode}

	for {
		select {
		case <-p.ctx.Done():
			return
		case cmd := <-p.In:
			p.engineChan <- p.handleCommand(cmd)
		default:
			//loop
		}
	}
}

func (p *Player) handleCommand(cmd *Command) PlayerResponse {
	resp := PlayerResponse{Name: p.Name}
	for _, step := range cmd.Steps {
		switch step.Action {
		case MoveAction:
			tile, ok := step.Data.(*MapTile)
			if !ok {
				//todo: need to handle errors better
				panic("could not cast tile to MapTile")
			}
			resp.Code = p.Move(tile.X, tile.Y)
		case GatherAction:
			resp.Code = p.Gather()
		case DepositAction:
			resp.Code = p.DepositInventory()
		default:
			//todo: dont panic
			panic(fmt.Sprintf("unknown action: %v", step.Action))
		}
	}

	return resp
}

func (p *Player) Data() PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data
}

func (p *Player) Move(x, y int) int {
	curX, curY := p.Pos()
	if curX == x && curY == y {
		log.Println("character already at position")
		return 200
	}
	log.Println("moving character to position")
	resp, err := p.client.ActionMoveMyNameActionMovePostWithResponse(p.ctx, p.Name, client.ActionMoveMyNameActionMovePostJSONRequestBody{
		X: x,
		Y: y,
	})

	if err != nil {
		log.Printf("error moving character to position: %v\n", err)
		return resp.StatusCode()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.data.Pos = PlayerPosition{
		X: x,
		Y: y,
	}

	waitForCooldown(resp.JSON200.Data.Cooldown)
	return resp.StatusCode()
}

func (p *Player) Gather() int {
	log.Println("gathering")
	resp, err := p.client.ActionGatheringMyNameActionGatheringPostWithResponse(p.ctx, p.Name)
	if err != nil {
		log.Printf("error gathering characters: %v\n", err)
		return resp.StatusCode()
	}

	if resp.StatusCode() == 200 {
		p.UpdateData(resp.JSON200.Data.Character)
		waitForCooldown(resp.JSON200.Data.Cooldown)
	}

	return resp.StatusCode()
}

func (p *Player) Pos() (x, y int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data.Pos.X, p.data.Pos.Y
}

func (p *Player) getData() error {
	resp, err := p.client.GetCharacterCharactersNameGetWithResponse(p.ctx, p.Name)
	if err != nil {
		return fmt.Errorf("get character status: %w", err)
	}
	p.UpdateData(resp.JSON200.Data)

	//if we are in a cooldown for any reason we should sleep it off (like when I restart the engine)
	if resp.JSON200.Data.Cooldown > 0 {
		waitForCooldownSeconds(resp.JSON200.Data.Cooldown)
	}
	return nil
}

func (p *Player) DepositInventory() int {
	log.Println("depositing inventory")
	var code int
	for _, i := range p.Data().Inventory {
		if i.Quantity > 0 {
			code = p.DepositItem(i.Code, i.Quantity)
		}
	}
	log.Println("depositing inventory complete")
	return code
}

func (p *Player) DepositItem(code string, qty int) int {

	resp, err := p.client.ActionDepositBankMyNameActionBankDepositPostWithResponse(p.ctx, p.Name, client.ActionDepositBankMyNameActionBankDepositPostJSONRequestBody{
		Code:     code,
		Quantity: qty,
	})
	if err != nil {
		log.Printf("deposit inventory: %v", err)
		return resp.StatusCode()
	}

	if resp.HTTPResponse.StatusCode == 200 {
		p.UpdateData(resp.JSON200.Data.Character)
		waitForCooldown(resp.JSON200.Data.Cooldown)
	}

	log.Println("deposit complete")
	return resp.StatusCode()
}

func (p *Player) UpdateData(s client.CharacterSchema) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data = PlayerData{
		Hp:      s.Hp,
		Stamina: s.Stamina,
		Level:   s.Level,
		Pos: PlayerPosition{
			X: s.X,
			Y: s.Y,
		},
		Skills: map[string]int{
			WoodcuttingSkill:      s.WoodcuttingLevel,
			FishingSkill:          s.FishingLevel,
			MiningSkill:           s.MiningLevel,
			GearcraftingSkill:     s.GearcraftingLevel,
			CookingSkill:          s.CookingLevel,
			WeaponCraftingSkill:   s.WeaponcraftingLevel,
			JeweleryCraftingSkill: s.JewelrycraftingLevel,
		},
		Inventory: *s.Inventory,
	}
}

func (p *Player) updatePos(x int, y int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.data.Pos.X = x
	p.data.Pos.Y = y
}
