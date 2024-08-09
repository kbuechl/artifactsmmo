package engine

import (
	"artifactsmmo/internal/commands"
	"artifactsmmo/internal/models"
	"artifactsmmo/internal/player"
	"fmt"
	"net/http"
)

func newGatherStep(qty int, tile models.MapTile) commands.Step {
	g := struct{ *commands.Stepper }{}
	g.StopFn = func(p *player.Player) bool { return p.CheckInventory(tile.Code) >= qty }
	g.ExecuteFn = func(p *player.Player) (int, error) {
		//todo: it would be great if we could deposit when gather returns a 497
		code := p.Gather(tile)
		return code, nil
	}

	return g
}

func newFightStep(qty int, tile models.MapTile) commands.Step {
	f := &struct {
		count int
		*commands.Stepper
	}{}
	f.StopFn = func(p *player.Player) bool { return f.count >= qty }
	f.ExecuteFn = func(p *player.Player) (int, error) {
		win, code := p.Fight(tile)
		if win {
			fmt.Printf("won, increasing count from %d to %d\n", qty, f.count)
			f.count += 1
		}

		return code, nil
	}

	return f
}

func newAcceptTaskStep(tile models.MapTile) commands.Step {
	s := struct {
		*commands.Stepper
	}{}

	s.StopFn = func(p *player.Player) bool { return true }
	s.ExecuteFn = func(p *player.Player) (int, error) {
		_, code := p.AcceptNewTask(tile)

		if code != http.StatusOK {
			return code, fmt.Errorf("accept task failed with code %d", code)
		}

		return code, nil
	}

	return s
}

func newCompleteTaskStep(tile models.MapTile) commands.Step {
	s := struct {
		*commands.Stepper
	}{}

	s.StopFn = func(p *player.Player) bool { return true }
	s.ExecuteFn = func(p *player.Player) (int, error) {
		_, code := p.CompleteTask(tile)
		if code != http.StatusOK {
			return code, fmt.Errorf("complete task failed with code %d", code)
		}
		return code, nil
	}

	return s
}

func newDepositInventoryStep(tile models.MapTile) commands.Step {
	s := struct {
		*commands.Stepper
	}{}
	s.StopFn = func(p *player.Player) bool { return true }
	s.ExecuteFn = func(p *player.Player) (int, error) {
		code := p.DepositInventory(tile)
		if code != http.StatusOK {
			return code, fmt.Errorf("deposit inventory failed with code %d", code)
		}
		return code, nil
	}

	return s
}
