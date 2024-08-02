package internal

import (
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
	"math/rand"
	"net/http"
	"time"
)

type GameEngine struct {
	player *Player
	world  *WorldDataCollector
	ctx    context.Context
	cancel context.CancelFunc
	Out    chan error
}

type GameConfig struct {
	Token string `yaml:"token"`
	URL   string `yaml:"url"`
	Name  string `yaml:"name"`
}

func NewGameEngine(ctx context.Context, cfg GameConfig) (*GameEngine, error) {
	gameCtx, cancel := context.WithCancel(ctx)

	c, err := client.NewClientWithResponses(cfg.URL, client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+cfg.Token)
		return nil
	}))

	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create client for %s: %w", cfg.Name, err)
	}

	p, err := NewPlayer(ctx, cfg.Name, c)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create player for %s: %w", cfg.Name, err)
	}

	wc, err := NewWorldCollector(ctx, c)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create world collector for %s: %w", cfg.Name, err)
	}

	return &GameEngine{
		player: p,
		world:  wc,
		ctx:    gameCtx,
		cancel: cancel,
		Out:    make(chan error),
	}, nil
}

func (e *GameEngine) MonitorForError() {
	for {
		select {
		case err := <-e.world.Out:
			e.Out <- fmt.Errorf("%s error in collector:%w", e.player.Name, err)
			fmt.Printf("stopping game for %s\n", e.player.Name)
			e.cancel()
		case err := <-e.player.Out:
			e.Out <- fmt.Errorf("error in player %s :%w", e.player.Name, err)
			fmt.Printf("stopping game for %s\n", e.player.Name)
			e.cancel()
		case <-e.ctx.Done():
			fmt.Printf("game for %s stopped due to context\n", e.player.Name)
			return
		}
	}
}

func (e *GameEngine) Start() {
	fmt.Printf("Starting game for %s\n", e.player.Name)
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			//using the character data find a resource we are allowed to gather
			fmt.Println("filtering map sections based on options")
			next, err := e.getNextGatherLocation()
			if curX, curY := e.player.CurrentPos(); curX != next.X && curY != next.Y {
				fmt.Println("moving to space to gather resource")
				// move to that space
				resp, err := e.player.Move(next.X, next.Y)
				e.exitOnError(err)
				fmt.Printf("moved to gather %s, cooldown %d \n", resp.Content.Code, resp.Cooldown.Seconds)
				time.Sleep(time.Second * time.Duration(resp.Cooldown.Seconds))
			} else {
				fmt.Println("current location selected, gathering")
			}
			gatherResp, err := e.player.Gather()
			e.exitOnError(err)
			fmt.Printf("finished gathering, cooldown %d\n", gatherResp.Seconds)
			time.Sleep(time.Second * time.Duration(gatherResp.Seconds))
		}

	}
}
func (e *GameEngine) getNextGatherLocation() (*MapData, error) {
	pData := e.player.Data()
	tiles := e.world.GetGatherableMapSections(pData.Skills)
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no gather locations found")
	}

	var currentTile *MapData
	otherTiles := make([]MapData, 0, len(tiles))
	for _, m := range tiles {
		if m.Y == pData.Pos.Y && m.X == pData.Pos.X {
			currentTile = &m
		} else {
			otherTiles = append(otherTiles, m)
		}
	}

	if currentTile == nil {
		return pickRandomMapSection(tiles)
	}

	//we only want resource tiles for this method
	if currentTile.Type != ResourceMapContentType.String() {
		return pickRandomMapSection(otherTiles)
	}

	resource, err := e.world.GetResourceByName(currentTile.Code)
	if err != nil {
		fmt.Printf("could not get resource for current location %s\n", currentTile.Code)
		return pickRandomMapSection(otherTiles)
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
		return currentTile, nil
	}

	return pickRandomMapSection(otherTiles)
}

func pickRandomMapSection(md []MapData) (*MapData, error) {
	if len(md) == 0 {
		return nil, fmt.Errorf("no gather locations found")
	}
	rand.NewSource(time.Now().UnixNano())
	return &md[rand.Intn(len(md))], nil
}

// todo: is this how we want to handle game loop errors?
func (e *GameEngine) exitOnError(err error) {
	if err != nil {
		fmt.Printf("error in game loop: %s\n", err)
		e.Out <- err
		e.cancel()
	}
}
