package engine

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/player"
	"artifactsmmo/internal/world"
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"net/http"
)

type GameEngine struct {
	players map[string]*player.Player
	In      chan commands.Response
	world   *world.Collector
	ctx     context.Context
	cancel  context.CancelFunc
	Out     chan error
	errChan chan error
	logger  *slog.Logger
}

type GameConfig struct {
	Token       string
	URL         string
	PlayerNames []string
}

func NewGameEngine(ctx context.Context, cfg GameConfig) (*GameEngine, error) {
	gameCtx, cancel := context.WithCancel(ctx)

	c, err := client.NewClientWithResponses(cfg.URL, client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+cfg.Token)
		return nil
	}))

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
		In:      make(chan commands.Response),
		world:   wc,
		ctx:     gameCtx,
		cancel:  cancel,
		errChan: make(chan error),
		Out:     make(chan error),
		players: map[string]*player.Player{},
		logger:  slog.Default().With("runner", "engine"),
	}

	for _, name := range cfg.PlayerNames {
		engine.logger.Debug(fmt.Sprintf("starting player %s", name))
		p, playerError := player.NewPlayer(ctx, name, c, engine.In, wc.BankChannel)
		if playerError != nil {
			cancel()
			return nil, fmt.Errorf("cannot create player for %s: %w", name, err)
		}
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
		case <-e.ctx.Done():
			return
		}
	}
}

// generatePlayerCommand determines the next step for a character given the character's state and previous instructions response
func (e *GameEngine) generatePlayerCommand(resp commands.Response, player *player.Player) (*commands.Command, error) {
	// todo: Does this need to be a receiver?

	if resp.Code == 497 {
		//player needs to deposit at the bank now
		return e.newDepositCommand(player)
	} else if resp.Code != 200 && resp.Code != commands.PlayerStartedCode {
		e.logger.Debug("got response from player", "code", resp.Code, "player", player.Name)
		return nil, fmt.Errorf("player %s responded with %d", resp.Name, resp.Code)
	} else {
		//will this be an issue for crafting?
		if player.InventoryCapacity() == 0 {
			return e.newDepositCommand(player)
		}

		//does the player have an active task?
		if player.Data().Task == nil {
			return e.newAcceptTaskCommand()
		}

		if player.Data().Task.Progress == player.Data().Task.Total {
			return e.newCompleteTaskCommand()
		}

		taskCode := player.Data().Task.Code

		switch player.Data().Task.Type {
		case "monsters":
			return e.newFightCommand(taskCode, player)
		case "resources":
			return e.newGatherCommand(taskCode, player)
		default:
			//todo: crafting
			e.logger.Warn("unmapped task type", "type", player.Data().Task.Type)
			//for now just prioritize lowest skill to mine
			pData := player.Data()
			minSkill := ""
			for skill, lvl := range pData.Skills {
				if minSkill == "" {
					minSkill = skill
					continue
				}
				if lvl < pData.Skills[minSkill] {
					minSkill = skill
				}
			}
			resources := e.world.GetResourcesBySkill(minSkill, pData.Skills[minSkill])

			if len(resources) == 0 {
				panic(fmt.Sprintf("no resources found for skill %s", minSkill))
			}

			return e.newGatherCommand(resources[0].Name, player)
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
			e.logger.Debug(fmt.Sprintf("received code %d for p %s", cr.Code, cr.Name))
			if cmd, err := e.generatePlayerCommand(cr, p); err != nil {
				e.exitOnError(err)
			} else {
				p.In <- cmd
			}
		default:
			//loop
		}
	}
}

func (e *GameEngine) newDepositCommand(player *player.Player) (*commands.Command, error) {
	//find bank
	tiles := e.world.GetMapByContentType(world.BankMapContentType)

	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find bank")
	}

	return &commands.Command{
		Steps: []commands.Step{
			{Action: commands.MoveAction, Data: tiles[0]},
			{Action: commands.DepositAction},
		},
	}, nil
}

func (e *GameEngine) newGatherCommand(resourceCode string, player *player.Player) (*commands.Command, error) {
	e.logger.Debug("filtering gather location")
	pData := player.Data()

	tile := e.world.FindClosestTile(resourceCode, pData.Pos.X, pData.Pos.Y)
	return &commands.Command{
		Steps: []commands.Step{
			{
				Action: commands.MoveAction,
				Data:   tile,
			}, {
				Action: commands.GatherAction,
			},
		},
	}, nil
}

// todo: is this how we want to handle game loop errors?
func (e *GameEngine) exitOnError(err error) {
	if err != nil {
		e.logger.Error("error in game loop", "error", err)
		e.Out <- err
		e.cancel()
	}
}

func (e *GameEngine) newFightCommand(monster string, player *player.Player) (*commands.Command, error) {
	cmd := &commands.Command{
		Steps: []commands.Step{},
	}

	//todo: for now lets shortcut
	//determine the best equipment we can use for this fight
	//find closest tile with the monster on it
	//add fight step

	tile := e.world.FindClosestTile(monster, player.Data().Pos.X, player.Data().Pos.Y)

	cmd.AddStep(commands.MoveAction, tile)
	cmd.AddStep(commands.FightAction, nil)
	return cmd, nil
}

func (e *GameEngine) newAcceptTaskCommand() (*commands.Command, error) {
	tiles := e.world.GetMapByContentType(world.TaskMasterContentType)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find task master")
	}

	cmd := commands.NewCommand(commands.MoveAction, tiles[0])
	cmd.AddStep(commands.AcceptTask, nil)
	return cmd, nil
}

func (e *GameEngine) newCompleteTaskCommand() (*commands.Command, error) {
	tiles := e.world.GetMapByContentType(world.TaskMasterContentType)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find task master")
	}

	cmd := commands.NewCommand(commands.MoveAction, tiles[0])
	cmd.AddStep(commands.CompleteTask, nil)
	return cmd, nil
}
