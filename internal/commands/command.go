package commands

import (
	"artifactsmmo/internal/models"
	"strings"

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
	WithdrawItem(code string, qty int) int
}

type StopStepFn func(p Player) bool
type ExecuteStepFn func(p Player) (int, error)

type Command struct {
	Steps []Step
}

func (c Command) String() string {
	response := make([]string, len(c.Steps))
	for _, step := range c.Steps {
		response = append(response, step.String())
	}
	return strings.Join(response, ",")
}

type Step interface {
	Stop(p Player) bool
	Execute(p Player) (int, error)
	String() string
}

type Stepper struct {
	StopFn      StopStepFn
	ExecuteFn   ExecuteStepFn
	Description string
}

func (s *Stepper) Execute(p Player) (int, error) {
	return s.ExecuteFn(p)
}

func (s *Stepper) Stop(p Player) bool {
	return s.StopFn(p)
}
func (s *Stepper) String() string {
	return s.Description
}

type CommandResponse struct {
	Name  string
	Error error
	Code  int
}
