package commands

import (
	"artifactsmmo/internal/models"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
)

type Action int

const (
	PlayerStartedCode = -1
)

type Player interface {
	CheckInventory(code string) int
	Gather(tile models.MapTile) int
	Fight(tile models.MapTile) (bool, int)
	DepositInventory(tile models.MapTile) int
	InventoryCapacity() int
	AcceptNewTask(tile models.MapTile) int
	CompleteTask(tile models.MapTile) (*client.TaskRewardSchema, int)
	ExchangeTaskCoins(tile models.MapTile) (*client.TaskRewardSchema, int)
}

type StopStepFn func(p Player) bool
type ExecuteStepFn func(p Player) (int, error)

type Command struct {
	Steps []Step
}

type Step interface {
	Stop(p Player) bool
	Execute(p Player) (int, error)
}

type Stepper struct {
	StopFn    StopStepFn
	ExecuteFn ExecuteStepFn
}

func (s *Stepper) Execute(p Player) (int, error) {
	return s.ExecuteFn(p)
}

func (s *Stepper) Stop(p Player) bool {
	return s.StopFn(p)
}

type CommandResponse struct {
	Name  string
	Error error
	Code  int
}
