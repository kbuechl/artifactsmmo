package engine

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/models"
	"artifactsmmo/internal/player"
	"artifactsmmo/internal/world"
	"context"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"math/rand"
	"net/http"
)

type GameEngine struct {
	players   map[string]*player.Player
	In        chan commands.CommandResponse
	playerErr chan error
	world     *world.Collector
	ctx       context.Context
	cancel    context.CancelFunc
	Out       chan error
	errChan   chan error
	logger    *slog.Logger
}

type GameConfig struct {
	Token       string
	URL         string
	PlayerNames []string
}

func NewGameEngine(ctx context.Context, cfg GameConfig) (*GameEngine, error) {
	gameCtx, cancel := context.WithCancel(ctx)

	retryClient := retryablehttp.NewClient()

	retryClient.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		shouldRetry, checkErr := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
		if shouldRetry || checkErr != nil {
			return shouldRetry, err
		}

		switch resp.StatusCode {
		case 461:
			//transaction already in progress with this at bank
			return true, nil
		case 486:
			//character locked, action in progress
			return true, nil
		case 499:
			//character in cooldown, shouldnt happen because we wait but can be retried
			return true, nil
		}
		return false, nil
	}

	c, err := client.NewClientWithResponses(cfg.URL,
		client.WithRequestEditorFn(client.NewBearerAuthorizationRequestFunc(cfg.Token)),
		client.WithHTTPClient(retryClient.HTTPClient),
	)

	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create client: %w", err)
	}

	wc, err := world.NewCollector(ctx, c)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create world collector: %w", err)
	}

	engine := &GameEngine{
		In:        make(chan commands.CommandResponse),
		world:     wc,
		ctx:       gameCtx,
		cancel:    cancel,
		errChan:   make(chan error),
		Out:       make(chan error),
		players:   map[string]*player.Player{},
		logger:    slog.Default().With("source", "engine"),
		playerErr: make(chan error),
	}

	for _, name := range cfg.PlayerNames {
		engine.logger.Debug(fmt.Sprintf("starting player %s", name))
		p := player.NewPlayer(ctx, name, c, engine.In, wc.BankChannel, engine.playerErr)
		engine.players[name] = p
	}

	go engine.Start()
	go engine.MonitorForError()

	return engine, nil
}

func (e *GameEngine) MonitorForError() {
	for {
		select {
		case err := <-e.errChan:
			e.exitOnError(err)
		case err := <-e.world.Out:
			e.exitOnError(err)
		case err := <-e.Out:
			e.logger.Error(err.Error())
		case <-e.ctx.Done():
			return
		}
	}
}

// generatePlayerCommand determines the next step for a character given the character's state and previous instructions response
func (e *GameEngine) generatePlayerCommand(resp commands.CommandResponse, player *player.Player) (commands.Step, error) {
	// todo: eventually we should return a slice or chain of steps to follow

	if resp.Code == 497 {
		//player needs to deposit at the bank now
		return e.newDepositStep()
	} else if resp.Code != 200 && resp.Code != commands.PlayerStartedCode {
		e.logger.Debug("got response from player", "code", resp.Code, "player", player.Name)
		return nil, fmt.Errorf("player %s responded with %d", resp.Name, resp.Code)
	} else {
		//will this be an issue for crafting?
		if player.InventoryCapacity() == 0 {
			return e.newDepositStep()
		}

		//does the player have an active task?
		if player.Data().Task == nil {
			return e.newAcceptTaskStep()
		}

		if player.Data().Task.Progress >= player.Data().Task.Total {
			return e.newCompleteTaskStep()
		}

		task := player.Data().Task

		//todo: crafting
		switch task.Type {
		case "resources":
			return e.newGatherStep(task.Code, task.Total-task.Progress, player)
		case "monsters":
			monster := e.world.GetMonster(task.Code)
			if monster == nil {
				return nil, fmt.Errorf("cannot find monster for: %s", task.Code)
			}

			if player.CanWinFight(models.Earth, *monster) {
				//todo: can we add in the turn in task step here after the fight step since we know the task is done after this?
				return e.newFightStep(task.Code, task.Total-task.Progress, player)
			}
			e.logger.Info("cannot win fight for given task, skipping task", "player", player.Name, "monster", task.Code)
			fallthrough
		default:
			//todo: this default logic is temporary
			//temporarily use 50/50 chance to fight random or gather some resource
			if rand.Int()%2 == 0 { //resource gather
				e.logger.Warn("unmapped task type", "type", player.Data().Task.Type)
				//for now just prioritize lowest skill to mine
				pData := player.Data()

				skill := []string{models.WoodcuttingSkill, models.FishingSkill, models.MiningSkill}[rand.Intn(2)]
				resources := e.world.GetResourcesBySkill(skill, pData.Skills[skill])

				if len(resources) == 0 {
					panic(fmt.Sprintf("no resources found for skill %s", skill))
				}

				return e.newGatherStep(resources[0].Code, rand.Intn(9)+1, player)
			} else {
				//temp code, fight random monster
				monsters := e.world.FilterMonsters(player)
				if len(monsters) == 0 {
					return nil, fmt.Errorf("no fightable monsters for %s", player.Name)
				}

				i := rand.Intn(len(monsters))
				if len(monsters) == 1 {
					i = 0
				}
				m := monsters[i]

				return e.newFightStep(m.Code, rand.Intn(9)+1, player)
			}

		}
	}
}

func (e *GameEngine) Start() {
	for {
		select {
		case cr := <-e.In:
			p, ok := e.players[cr.Name]
			if !ok {
				e.exitOnError(fmt.Errorf("p %s not found", cr.Name))
			}
			e.logger.Debug(fmt.Sprintf("received code %d for player %s", cr.Code, cr.Name))
			if cmd, err := e.generatePlayerCommand(cr, p); err != nil {
				e.exitOnError(err)
			} else {
				p.In <- commands.Command{Steps: []commands.Step{cmd}}
			}
		default:
			//loop
		}
	}
}

func (e *GameEngine) newDepositStep() (commands.Step, error) {
	//find bank
	tiles := e.world.GetMapByContentType(world.BankMapContentType)

	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find bank")
	}
	return commands.NewDepositInventoryStep(*tiles[0]), nil
}

func (e *GameEngine) newGatherStep(resourceCode string, qty int, player *player.Player) (commands.Step, error) {
	e.logger.Debug("filtering gather location")
	pData := player.Data()

	tile := e.world.FindClosestTile(resourceCode, pData.Pos.X, pData.Pos.Y)
	if tile == nil {
		return nil, fmt.Errorf("could not find tile for resource code %s", resourceCode)
	}

	return commands.NewGatherStep(qty, *tile), nil
}

// todo: is this how we want to handle game loop errors?
func (e *GameEngine) exitOnError(err error) {
	if err != nil {
		e.logger.Error("error in game loop", "error", err)
		e.Out <- err
		e.cancel()
	}
}

func (e *GameEngine) newFightStep(monster string, qty int, player *player.Player) (commands.Step, error) {
	//todo: for now lets shortcut
	//determine the best equipment we can use for this fight
	//find closest tile with the monster on it
	//add fight step

	tile := e.world.FindClosestTile(monster, player.Data().Pos.X, player.Data().Pos.Y)
	if tile == nil {
		return nil, fmt.Errorf("could not find tile for monster %s", monster)
	}

	return commands.NewFightStep(qty, *tile), nil
}

func (e *GameEngine) newAcceptTaskStep() (commands.Step, error) {
	tiles := e.world.GetMapByContentType(world.TaskMasterContentType)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find task master")
	}
	return commands.NewAcceptTaskStep(*tiles[0]), nil
}

func (e *GameEngine) newCompleteTaskStep() (commands.Step, error) {
	tiles := e.world.GetMapByContentType(world.TaskMasterContentType)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find task master")
	}

	return commands.NewCompleteTaskStep(*tiles[0]), nil
}
