package player

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/models"
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"net/http"
	"sync"
	"time"
)

const maxFightRounds = 100

type Player struct {
	Name        string
	data        PlayerData
	mu          sync.RWMutex
	ctx         context.Context
	client      *client.ClientWithResponses
	engineChan  chan commands.Response
	In          chan *commands.Command
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

// Player is the character abstraction from the engine.
func NewPlayer(ctx context.Context, name string, client *client.ClientWithResponses, rc chan commands.Response, bc chan models.BankResponse, errChan chan error) *Player {
	logger := slog.Default().With("player", name)
	p := &Player{
		Name:        name,
		client:      client,
		ctx:         ctx,
		engineChan:  rc,
		In:          make(chan *commands.Command),
		logger:      logger,
		bankChannel: bc,
		errChan:     errChan,
	}

	go p.start()

	return p
}

func (p *Player) start() {
	if err := p.getData(); err != nil {
		p.errChan <- fmt.Errorf("error getting data in character %s: %s", p.Name, err)
		return
	}

	//send initial command to tell eng online
	p.engineChan <- commands.Response{Name: p.Name, Code: commands.PlayerStartedCode}

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
func (p *Player) handleCommand(cmd *commands.Command) commands.Response {
	resp := commands.Response{Name: p.Name}
	for _, step := range cmd.Steps {
		switch step.Action {
		case commands.MoveAction:
			tile, ok := step.Data.(*models.MapTile)
			if !ok {
				//todo: need to handle errors better
				panic("could not cast tile to MapTile")
			}
			resp.Code = p.Move(tile.X, tile.Y)
		case commands.GatherAction:
			resp.Code = p.Gather()
		case commands.DepositAction:
			resp.Code = p.DepositInventory()
		case commands.AcceptTask:
			_, resp.Code = p.AcceptNewTask()
		case commands.CompleteTask:
			_, resp.Code = p.CompleteTask()
		case commands.FightAction:
			resp.Code = p.Fight()
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

	if expiration, err := s.CooldownExpiration.AsCharacterSchemaCooldownExpiration0(); err != nil {
		panic(err)
	} else if expiration.After(time.Now()) {
		//waitForCooldown(expiration) //todo: there is an issue here with expiration expiring slightly before
		waitForCooldownSeconds(s.Cooldown)
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
		p.logger.Debug("fight error", "error", err)
	}
	if resp.StatusCode() != 200 {
		p.logger.Debug("got non 200 status from fight", "code", resp.StatusCode())
		return resp.StatusCode()
	}

	p.logger.Debug("fight result", "result", resp.JSON200.Data.Fight.Result, "turns", resp.JSON200.Data.Fight.Turns)
	p.UpdateData(resp.JSON200.Data.Character)

	return resp.StatusCode()
}

func (p *Player) CanWinFight(attackType models.AttackType, monster models.Monster) bool {
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

func waitForCooldownSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}
