package player

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/models"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
)

const maxFightRounds = 100

type PlayerNotFound struct{}

func (e PlayerNotFound) Error() string {
	return "player not found"
}

type Player struct {
	Name        string
	data        PlayerData
	mu          sync.RWMutex
	ctx         context.Context
	client      *client.ClientWithResponses
	engineChan  chan commands.CommandResponse
	In          chan commands.Command
	logger      *slog.Logger
	bankChannel chan models.BankResponse
	errChan     chan error
}

type PlayerPosition struct {
	X int
	Y int
}

type PlayerTask struct {
	Code     string
	Type     string
	Progress int
	Total    int
}

type PlayerData struct {
	Stamina      int
	Hp           int
	Skills       map[string]int
	Pos          PlayerPosition
	Task         *PlayerTask
	MaxInventory int
	Inventory    []client.InventorySlot
	Level        int
	AttackStats  map[models.AttackType]int
	DefenseStats map[models.AttackType]int
}

type playerResponse struct {
	Code  int
	Error error
}

// Player is the character abstraction from the engine.
func NewPlayer(ctx context.Context, name string, client *client.ClientWithResponses, rc chan commands.CommandResponse, bc chan models.BankResponse, errChan chan error) *Player {
	logger := slog.Default().With("source", name)
	p := &Player{
		Name:        name,
		client:      client,
		ctx:         ctx,
		engineChan:  rc,
		In:          make(chan commands.Command),
		logger:      logger,
		bankChannel: bc,
		errChan:     errChan,
	}

	go p.start()

	return p
}

func (p *Player) start() {
	if err := p.getData(); err != nil {
		var playerNotFound PlayerNotFound
		if errors.As(err, &playerNotFound) {
			if pErr := p.createCharacter(); pErr != nil {
				p.errChan <- fmt.Errorf("error creating character: %s", pErr)
			}
		} else {
			p.errChan <- fmt.Errorf("error getting data in character %s: %s", p.Name, err)
			return
		}
	}

	//send initial command to tell eng online
	p.engineChan <- commands.CommandResponse{Name: p.Name, Code: commands.PlayerStartedCode}

	for {
		select {
		case <-p.ctx.Done():
			return
		case cmd := <-p.In:
			r := p.processCommand(cmd)
			p.engineChan <- commands.CommandResponse{Name: p.Name, Error: r.Error, Code: r.Code}
		default:
			//loop
		}
	}
}

func (p *Player) processCommand(cmd commands.Command) *playerResponse {
	for _, s := range cmd.Steps {
	loop:
		for {
			if code, err := s.Execute(p); err != nil {
				return &playerResponse{
					Code:  code,
					Error: err,
				}
			}
			if s.Stop(p) {
				break loop
			}
		}
	}
	return nil
}

func (p *Player) Data() PlayerData {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data
}

func (p *Player) move(x, y int) int {
	curX, curY := p.Pos()
	if curX == x && curY == y {
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

func (p *Player) Gather(tile models.MapTile) int {
	if code := p.move(tile.X, tile.Y); code != http.StatusOK {
		p.logger.Warn("Could not move to gather", slog.Group("code", code))
		return code
	}

	p.logger.Debug("gathering", "resource", tile.Code)
	resp, err := p.client.ActionGatheringMyNameActionGatheringPostWithResponse(p.ctx, p.Name)
	if err != nil {
		p.logger.Debug("error gathering characters: %v", err)
		return resp.StatusCode()
	}

	if resp.StatusCode() == http.StatusOK {
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
	if resp.StatusCode() == http.StatusNotFound {
		return PlayerNotFound{}
	}
	p.UpdateData(resp.JSON200.Data)

	return nil
}

func (p *Player) DepositInventory(tile models.MapTile) int {
	if code := p.move(tile.X, tile.Y); code != http.StatusOK {
		p.logger.Warn("Could not move to deposit inventory", slog.Group("code", code))
		return code
	}
	p.logger.Debug("depositing inventory")
	var code int
	for _, i := range p.Data().Inventory {
		if i.Quantity > 0 {
			code = p.depositItem(i.Code, i.Quantity)
		}
	}
	p.logger.Debug("depositing inventory complete")
	return code
}

// depositItem is meant to be called when the player is already at the bank, If a use case comes up where the player needs to deposit a single item we will need to refactor
func (p *Player) depositItem(code string, qty int) int {
	resp, err := p.client.ActionDepositBankMyNameActionBankDepositPostWithResponse(p.ctx, p.Name, client.ActionDepositBankMyNameActionBankDepositPostJSONRequestBody{
		Code:     code,
		Quantity: qty,
	})
	if err != nil {
		p.logger.Debug("deposit inventory: %v", err)
		return resp.StatusCode()
	}

	if resp.HTTPResponse.StatusCode == 200 {
		p.bankChannel <- models.BankResponse{
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

	var task *PlayerTask

	if s.Task != "" {
		task = &PlayerTask{
			Code:     s.Task,
			Type:     s.TaskType,
			Progress: s.TaskProgress,
			Total:    s.TaskTotal,
		}
	}

	p.data = PlayerData{
		Hp:           s.Hp,
		Stamina:      s.Stamina,
		Level:        s.Level,
		MaxInventory: s.InventoryMaxItems,
		Inventory:    *s.Inventory,
		Task:         task,
		Pos: PlayerPosition{
			X: s.X,
			Y: s.Y,
		},
		Skills: map[string]int{
			models.WoodcuttingSkill:      s.WoodcuttingLevel,
			models.FishingSkill:          s.FishingLevel,
			models.MiningSkill:           s.MiningLevel,
			models.GearcraftingSkill:     s.GearcraftingLevel,
			models.CookingSkill:          s.CookingLevel,
			models.WeaponCraftingSkill:   s.WeaponcraftingLevel,
			models.JeweleryCraftingSkill: s.JewelrycraftingLevel,
		},
		AttackStats: map[models.AttackType]int{
			models.Fire:  calculateElementDamage(s.AttackFire, s.DmgFire),
			models.Air:   calculateElementDamage(s.AttackAir, s.DmgAir),
			models.Water: calculateElementDamage(s.AttackWater, s.DmgWater),
			models.Earth: calculateElementDamage(s.AttackEarth, s.DmgEarth),
		},
		DefenseStats: map[models.AttackType]int{
			models.Fire:  s.ResFire,
			models.Air:   s.ResAir,
			models.Water: s.ResWater,
			models.Earth: s.ResEarth,
		},
	}
	p.mu.Unlock()

	//temporary while we cant use expiration for fighting due to early timeout
	// if cd, err := s.CooldownExpiration.AsCharacterSchemaCooldownExpiration0(); err != nil {
	// 	waitForCooldownSeconds(s.Cooldown)
	// } else if cd.After(time.Now()) {
	waitForCooldownSeconds(s.Cooldown)
	// }
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

func (p *Player) Fight(tile models.MapTile) (bool, int) {
	if code := p.move(tile.X, tile.Y); code != http.StatusOK {
		p.logger.Warn("Could not move to fight", slog.Group("code", code))
		return false, code
	}
	resp, err := p.client.ActionFightMyNameActionFightPostWithResponse(p.ctx, p.Name)
	if err != nil {
		p.logger.Debug("fight error", "error", err)
	}
	if resp.StatusCode() != 200 {
		p.logger.Debug("got non 200 status from fight", "code", resp.StatusCode())
		return false, resp.StatusCode()
	}

	p.logger.Debug("fight result", "result", resp.JSON200.Data.Fight.Result, "turns", resp.JSON200.Data.Fight.Turns, "monster", tile.Code)
	p.UpdateData(resp.JSON200.Data.Character)

	return resp.JSON200.Data.Fight.Result == "win", resp.StatusCode()
}

func (p *Player) CanWinFight(attackType models.AttackType, monster models.Monster) bool {
	// Calculate the damage each entity deals
	monsterDmg := calculateAttackDamage(monster.AttackDmg, p.Data().DefenseStats[monster.AttackType])
	playerDmg := calculateAttackDamage(p.Data().AttackStats[monster.AttackType], monster.Resistances[monster.AttackType])

	// Calculate how many turns each entity can take
	playerTurns := float64(monster.Hp) / float64(playerDmg)
	monsterTurns := float64(p.Data().Hp) / float64(monsterDmg)

	// Determine the outcome
	if playerTurns >= float64(maxFightRounds) || playerTurns >= monsterTurns {
		return false
	}
	return true
}

func (p *Player) CheckInventory(code string) int {
	count := 0

	for _, i := range p.Data().Inventory {
		if i.Code == code {
			count += i.Quantity
		}
	}

	return count
}

func waitForCooldown(expiration time.Time) {
	time.Sleep(time.Until(expiration))
}

func waitForCooldownSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
