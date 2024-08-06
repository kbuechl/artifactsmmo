package engine

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/player"
	world2 "artifactsmmo/internal/world"
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"math/rand"
	"net/http"
	"time"
)

const (
	PlayerStartedCode = -1
)

type GameEngine struct {
	players map[string]*player.Player
	In      chan commands.Response
	world   *world2.Collector
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

	wc, err := world2.NewCollector(ctx, c)
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
		logger:  slog.Default().With("engine"),
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
	} else if resp.Code != 200 && resp.Code != PlayerStartedCode {
		e.logger.Debug("player response", resp.Code, "player", player.Name)
		return nil, fmt.Errorf("player %s responded with %d", resp.Name, resp.Code)
	} else {
		//will this be an issue for crafting?
		if player.InventoryCapacity() == 0 {
			return e.newDepositCommand(player)
		}

		//determine what we should do next
		//todo: make decisions on fight vs craft vs gather
		return e.newGatherCommand(player)
	}
}

func (e *GameEngine) Start() {
	for {
		select {
		case cr := <-e.In:
			player, ok := e.players[cr.Name]
			if !ok {
				e.exitOnError(fmt.Errorf("player %s not found", cr.Name))
			}
			e.logger.Debug("code", cr.Code, "player", cr.Name)
			if cmd, err := e.generatePlayerCommand(cr, player); err != nil {
				e.exitOnError(err)
			} else {
				player.In <- cmd
			}
		default:
			//loop
		}
	}
}

func (e *GameEngine) newDepositCommand(player *player.Player) (*commands.Command, error) {
	//find bank
	tiles := e.world.GetMapByContentType(world2.BankMapContentType)

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

func (e *GameEngine) newGatherCommand(player *player.Player) (*commands.Command, error) {
	e.logger.Debug("filtering gather location")
	pData := player.Data()
	tiles := e.world.GetGatherableMapTiles(pData.Skills)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no gather locations found")
	}

	var currentTile *world2.MapTile
	otherTiles := make([]world2.MapTile, 0, len(tiles))
	for _, m := range tiles {
		if m.Y == pData.Pos.Y && m.X == pData.Pos.X {
			currentTile = &m
		} else {
			otherTiles = append(otherTiles, m)
		}
	}

	//we only want to use current tile if it's a resource tile for gather
	if currentTile == nil || currentTile.Type != world2.ResourceMapContentType.String() {
		return gatherRandomTile(otherTiles)
	}

	resource, err := e.world.GetResourceByName(currentTile.Code)
	if err != nil {
		e.logger.Debug("could not get resource for current location", "code", currentTile.Code)
		return gatherRandomTile(otherTiles)
	}

	skill := pData.Skills[resource.Skill]
	skillLimited := false
	for _, lvl := range pData.Skills {
		if lvl*3 < skill {
			skillLimited = true
			break
		}
	}

	if !skillLimited {
		return commands.NewCommand(commands.GatherAction, nil), nil
	}

	return gatherRandomTile(otherTiles)
}

func gatherRandomTile(md []world2.MapTile) (*commands.Command, error) {
	if len(md) == 0 {
		return nil, fmt.Errorf("no gather locations found")
	}
	rand.NewSource(time.Now().UnixNano())
	tile := &md[rand.Intn(len(md))]

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
		e.logger.Error("error in game loop", err)
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

	//get monsters we can fight on the map

	//find monster but prioritize the current square

	for _, t := range e.world.MapTiles() {
		if t.Code == monster {

		}
	}
	//move to square

	//fight

	return cmd, nil
}
