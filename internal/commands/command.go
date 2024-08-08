package commands

type Action int

const (
	PlayerStartedCode = -1
)

const (
	MoveAction Action = iota
	GatherAction
	FightAction
	DepositAction
	AcceptTask
	CompleteTask
)

// todo: turn this into a closure to execute on the player, we can then chain them together and handle a bit cleaner in a loop
type Response struct {
	Name   string
	Action Action
	Code   int
}

type Command struct {
	Steps []Step
}

type Step struct {
	Action Action
	Data   any
}

func NewCommand(action Action, data any) *Command {
	return &Command{Steps: []Step{{Action: action, Data: data}}}
}

func (c *Command) AddStep(action Action, data any) {
	c.Steps = append(c.Steps, Step{Action: action, Data: data})
}
