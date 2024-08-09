package commands

import (
	"artifactsmmo/internal/player"
)

type Action int

const (
	PlayerStartedCode = -1
)

type StopStepFn func(p *player.Player) bool
type ExecuteStepFn func(p *player.Player) (int, error)

type Command struct {
	Steps []Step
}

type Step interface {
	Stop(p *player.Player) bool
	Execute(p *player.Player) (int, error)
}

type Stepper struct {
	StopFn    StopStepFn
	ExecuteFn ExecuteStepFn
}

func (s *Stepper) Execute(p *player.Player) (int, error) {
	return s.ExecuteFn(p)
}

func (s *Stepper) Stop(p *player.Player) bool {
	return s.StopFn(p)
}

type CommandResponse struct {
	Name  string
	Error error
	Code  int
}
