package player

import (
	"artifactsmmo/internal"
	"artifactsmmo/internal/engine"
	"artifactsmmo/internal/world"
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"net/http"
	"sync"
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

type AttackType string

const (
	Air   AttackType = "air"
	Earth AttackType = "earth"
	Water AttackType = "water"
	Fire  AttackType = "fire"
)

const maxFightRounds = 100

type Player struct {
	Name        string
	data        PlayerData
	mu          sync.RWMutex
	ctx         context.Context
	client      *client.ClientWithResponses
	engineChan  chan internal.CommandResponse
	In          chan *internal.Command
	logger      *slog.Logger
	bankChannel chan world.BankResponse
}

type PlayerPosition struct {
	X int
	Y int
}

type PlayerData struct {
	Stamina      int
	Hp           int
	Skills       map[string]int
	Pos          PlayerPosition
	MaxInventory int
	Inventory    []client.InventorySlot
	Level        int
	AttackStats  map[AttackType]int
	DefenseStats map[AttackType]int
}

// Player is the character abstraction from the engine.
func NewPlayer(ctx context.Context, name string, client *client.ClientWithResponses, rc chan internal.CommandResponse, bc chan world.BankResponse) (*Player, error) {
	logger := slog.Default().With("player", name)
	p := &Player{
		Name:        name,
		client:      client,
		ctx:         ctx,
		engineChan:  rc,
		In:          make(chan *internal.Command),
		logger:      logger,
		bankChannel: bc,
	}

	if err := p.getData(); err != nil {
		return nil, err
	}

	go p.start()

	return p, nil
}

func (p *Player) start() {
	//send initial command to tell eng online
	p.engineChan <- internal.CommandResponse{Name: p.Name, Code: engine.PlayerStartedCode}

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

// handleCommand will handle a command from the engine, the steps are followed in order unless a non 200 response is given, when that happens
// we return the action and the code so the engine can decide the next steps.
// Players are responsible for handling cooldowns before moving onto the next step
func (p *Player) handleCommand(cmd *internal.Command) internal.CommandResponse {
	resp := internal.CommandResponse{Name: p.Name}
	for _, step := range cmd.Steps {
		switch step.Action {
		case internal.MoveAction:
			tile, ok := step.Data.(*world.MapTile)
			if !ok {
				//todo: need to handle errors better
				panic("could not cast tile to MapTile")
			}
			resp.Code = p.Move(tile.X, tile.Y)
		case internal.GatherAction:
			resp.Code = p.Gather()
		case internal.DepositAction:
			resp.Code = p.DepositInventory()
		default:
			//todo: dont panic
			panic(fmt.Sprintf("unknown action: %v", step.Action))
		}
		resp.Action = step.Action
		if resp.Code != http.StatusOK {
			break //stop processing and report it back to the engine for next steps
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
		p.logger.Debug("character already at position")
		return 200
	}
	p.logger.Debug("moving character to position")
	resp, err := p.client.ActionMoveMyNameActionMovePostWithResponse(p.ctx, p.Name, client.ActionMoveMyNameActionMovePostJSONRequestBody{
		X: x,
		Y: y,
	})

	if err != nil {
		p.logger.Debug("error moving character to position: %v\n", err)
		return resp.StatusCode()
	}

	if resp.StatusCode() == 200 {
		p.UpdateData(resp.JSON200.Data.Character)
	}

	return resp.StatusCode()
}

func (p *Player) Gather() int {
	p.logger.Debug("gathering")
	resp, err := p.client.ActionGatheringMyNameActionGatheringPostWithResponse(p.ctx, p.Name)
	if err != nil {
		p.logger.Debug("error gathering characters: %v", err)
		return resp.StatusCode()
	}

	if resp.StatusCode() == 200 {
		p.UpdateData(resp.JSON200.Data.Character)
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

	return nil
}

func (p *Player) DepositInventory() int {
	p.logger.Debug("depositing inventory")
	var code int
	for _, i := range p.Data().Inventory {
		if i.Quantity > 0 {
			code = p.DepositItem(i.Code, i.Quantity)
		}
	}
	p.logger.Debug("depositing inventory complete")
	return code
}

func (p *Player) DepositItem(code string, qty int) int {

	resp, err := p.client.ActionDepositBankMyNameActionBankDepositPostWithResponse(p.ctx, p.Name, client.ActionDepositBankMyNameActionBankDepositPostJSONRequestBody{
		Code:     code,
		Quantity: qty,
	})
	if err != nil {
		p.logger.Debug("deposit inventory: %v", err)
		return resp.StatusCode()
	}

	if resp.HTTPResponse.StatusCode == 200 {
		p.bankChannel <- world.BankResponse{
			Gold:  nil,
			Items: &resp.JSON200.Data.Bank,
		}
		p.UpdateData(resp.JSON200.Data.Character)

	}

	p.logger.Debug("deposit complete")
	return resp.StatusCode()
}

// UpdateData updates the player data and wait for the cooldown
func (p *Player) UpdateData(s client.CharacterSchema) {
	p.mu.Lock()

	p.data = PlayerData{
		Hp:           s.Hp,
		Stamina:      s.Stamina,
		Level:        s.Level,
		MaxInventory: s.InventoryMaxItems,
		Inventory:    *s.Inventory,
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
		AttackStats: map[AttackType]int{
			Fire:  calculateElementDamage(s.AttackFire, s.DmgFire),
			Air:   calculateElementDamage(s.AttackAir, s.DmgAir),
			Water: calculateElementDamage(s.AttackWater, s.DmgWater),
			Earth: calculateElementDamage(s.AttackEarth, s.DmgEarth),
		},
		DefenseStats: map[AttackType]int{
			Fire:  s.ResFire,
			Air:   s.ResAir,
			Water: s.ResWater,
			Earth: s.ResEarth,
		},
	}
	p.mu.Unlock()

	if cooldown, err := s.CooldownExpiration.AsCharacterSchemaCooldownExpiration0(); err != nil {
		panic(err) //this union type stuff is dumb
	} else {
		waitForCooldown(cooldown)
	}
}

func (p *Player) InventoryCapacity() int {
	c := p.Data().MaxInventory

	for _, i := range p.Data().Inventory {
		if i.Quantity > 0 && i.Code != "" {
			c -= i.Quantity
		}
	}

	return c
}

func (p *Player) Fight() int {
	resp, err := p.client.ActionFightMyNameActionFightPostWithResponse(p.ctx, p.Name)
	if err != nil {
		p.logger.Debug("fight inventory: %v", err)
	}
	if resp.StatusCode() != 200 {
		p.logger.Debug("got non 200 status from fight", resp.StatusCode())
	} else {
		p.logger.Debug("fight result", resp.JSON200.Data.Fight.Result, "turns", resp.JSON200.Data.Fight.Turns)
		p.UpdateData(resp.JSON200.Data.Character)
	}

	return resp.StatusCode()
}

func (p *Player) CanWinFight(attackType AttackType, monster internal.Monster) bool {
	//check equipment and consumables to determine health + attack
	monsterDmg := calculateAttackDamage(monster.AttackDmg, p.Data().DefenseStats[monster.AttackType])
	playerDmg := calculateAttackDamage(p.Data().AttackStats[attackType], monster.Resistances[attackType])

	//if we pass 100 turns we auto lose
	if monster.Hp/playerDmg > maxFightRounds || monster.Hp/playerDmg < p.Data().Hp/monsterDmg {
		return false
	}
	return true
}

func waitForCooldown(expiration time.Time) {
	time.Sleep(time.Until(expiration))
}
