package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type CommandAction int

const (
	PlayerStartedCode = -1
)

type GameEngine struct {
	players map[string]*Player
	In      chan PlayerResponse
	world   *WorldDataCollector
	ctx     context.Context
	cancel  context.CancelFunc
	Out     chan error
	errChan chan error
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

	wc, err := NewWorldCollector(ctx, c)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create world collector: %w", err)
	}

	engine := &GameEngine{
		In:      make(chan PlayerResponse),
		world:   wc,
		ctx:     gameCtx,
		cancel:  cancel,
		errChan: make(chan error),
		Out:     make(chan error),
		players: map[string]*Player{},
	}

	for _, name := range cfg.PlayerNames {
		p, playerError := NewPlayer(ctx, name, c, engine.In)
		if playerError != nil {
			cancel()
			return nil, fmt.Errorf("cannot create player for %s: %w", name, err)
		}
		engine.players[name] = p
	}

	go engine.Start()
	return engine, nil
}

func (e *GameEngine) MonitorForError() {
	for {
		select {
		case err := <-e.world.Out:
			e.Out <- fmt.Errorf("error in collector: %w", err)
			log.Println("stopping game due to error")
			e.cancel()
		case err := <-e.errChan:
			log.Printf("stopping game due to error: %v", err)
			e.cancel()
		case <-e.ctx.Done():
			log.Println("game stopped due to context")
			return
		}
	}
}

func (e *GameEngine) handlePlayerResponse(resp PlayerResponse) error {
	player, ok := e.players[resp.Name]

	if !ok {
		return fmt.Errorf("player %s not found", resp.Name)
	}

	if resp.Code == 497 {
		//player needs to deposit at the bank now
		cmd, err := e.newDepositCommand(player)
		if err != nil {
			return fmt.Errorf("cannot create deposit command: %w", err)
		}

		player.In <- cmd
	} else if resp.Code != 200 && resp.Code != PlayerStartedCode {
		log.Printf("player %s responded with %d", resp.Name, resp.Code)
	} else {
		//determine what we should do next
		//todo: make decisions on fight vs craft vs gather
		gatherCmd, err := e.newGatherCommand(player)
		if err != nil {
			return fmt.Errorf("cannot create gather command: %w", err)
		}

		player.In <- gatherCmd
	}

	return nil
}

func (e *GameEngine) Start() {
	for {
		select {
		case playerResponse := <-e.In:
			if err := e.handlePlayerResponse(playerResponse); err != nil {
				e.exitOnError(err)
			}
		default:
			//loop
		}
	}
}

func (e *GameEngine) newDepositCommand(player *Player) (*Command, error) {
	//find bank
	tiles := e.world.GetMapByContentType(BankMapContentType)

	if len(tiles) == 0 {
		return nil, fmt.Errorf("could not find bank")
	}

	return &Command{
		Steps: []Step{
			{Action: MoveAction, Data: tiles[0]},
			{Action: DepositAction},
		},
	}, nil
}

func (e *GameEngine) newGatherCommand(player *Player) (*Command, error) {
	log.Println("filtering gather location")
	pData := player.Data()
	tiles := e.world.GetGatherableMapSections(pData.Skills)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no gather locations found")
	}

	var currentTile *MapTile
	otherTiles := make([]MapTile, 0, len(tiles))
	for _, m := range tiles {
		if m.Y == pData.Pos.Y && m.X == pData.Pos.X {
			currentTile = &m
		} else {
			otherTiles = append(otherTiles, m)
		}
	}

	//we only want to use current tile if it's a resource tile for gather
	if currentTile == nil || currentTile.Type != ResourceMapContentType.String() {
		return gatherRandomTile(otherTiles)
	}

	resource, err := e.world.GetResourceByName(currentTile.Code)
	if err != nil {
		log.Printf("could not get resource for current location %s\n", currentTile.Code)
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
		return newCommand(GatherAction, nil), nil
	}

	return gatherRandomTile(otherTiles)
}

func gatherRandomTile(md []MapTile) (*Command, error) {
	if len(md) == 0 {
		return nil, fmt.Errorf("no gather locations found")
	}
	rand.NewSource(time.Now().UnixNano())
	tile := &md[rand.Intn(len(md))]

	return &Command{
		Steps: []Step{
			{
				Action: MoveAction,
				Data:   tile,
			}, {
				Action: GatherAction,
			},
		},
	}, nil
}

// todo: is this how we want to handle game loop errors?
func (e *GameEngine) exitOnError(err error) {
	if err != nil {
		log.Printf("error in game loop: %s\n", err)
		e.Out <- err
		e.cancel()
	}
}
