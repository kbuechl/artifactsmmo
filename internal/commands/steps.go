package commands

import (
	"artifactsmmo/internal/models"
	"fmt"
	"net/http"
)

func NewGatherStep(qty int, tile models.MapTile) Step {
	g := struct{ *Stepper }{
		Stepper: &Stepper{},
	}
	g.Description = fmt.Sprintf("Gather %d %s", qty, tile.Code)
	g.StopFn = func(p Player) bool { return p.CheckInventory(tile.Code) >= qty }
	g.ExecuteFn = func(p Player) (int, error) {
		//todo: it would be great if we could deposit when gather returns a 497
		code := p.Gather(tile)
		return code, nil
	}

	return g
}

func NewFightStep(qty int, tile models.MapTile) Step {
	f := struct {
		count int
		*Stepper
	}{
		count:   0,
		Stepper: &Stepper{},
	}
	f.Description = fmt.Sprintf("Fight %d %s", qty, tile.Code)
	f.StopFn = func(p Player) bool { return f.count >= qty }
	f.ExecuteFn = func(p Player) (int, error) {
		win, code := p.Fight(tile)
		if win {
			fmt.Printf("won, increasing count from %d to %d\n", qty, f.count)
			f.count += 1
		}

		return code, nil
	}

	return f
}

func NewAcceptTaskStep(tile models.MapTile) Step {
	s := struct {
		*Stepper
	}{
		Stepper: &Stepper{},
	}

	s.Description = "Accept Task"
	s.StopFn = func(p Player) bool { return true }
	s.ExecuteFn = func(p Player) (int, error) {
		code := p.AcceptNewTask(tile)

		if code != http.StatusOK {
			return code, fmt.Errorf("accept task failed with code %d", code)
		}

		return code, nil
	}

	return s
}

func NewCompleteTaskStep(tile models.MapTile) Step {
	s := struct {
		*Stepper
	}{
		Stepper: &Stepper{},
	}
	s.Description = "Complete Task"
	s.StopFn = func(p Player) bool { return true }
	s.ExecuteFn = func(p Player) (int, error) {
		_, code := p.CompleteTask(tile)
		if code != http.StatusOK {
			return code, fmt.Errorf("complete task failed with code %d", code)
		}
		return code, nil
	}

	return s
}

func NewDepositInventoryStep(tile models.MapTile) Step {
	s := struct {
		*Stepper
	}{
		Stepper: &Stepper{},
	}
	s.Description = "Deposit Inventory"
	s.StopFn = func(p Player) bool { return true }
	s.ExecuteFn = func(p Player) (int, error) {
		code := p.DepositInventory(tile)
		if code != http.StatusOK {
			return code, fmt.Errorf("deposit inventory failed with code %d", code)
		}
		return code, nil
	}

	return s
}
