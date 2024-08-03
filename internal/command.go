package internal

const (
	MoveAction CommandAction = iota
	GatherAction
	FightAction
	DepositAction
)

type PlayerResponse struct {
	Name string
	Code int
}

type Command struct {
	Steps []Step
}

type Step struct {
	Action CommandAction
	Data   interface{}
}

func newCommand(action CommandAction, data any) *Command {
	return &Command{Steps: []Step{{Action: action, Data: data}}}
}

func (c *Command) AddStep(action CommandAction, data any) {
	c.Steps = append(c.Steps, Step{Action: action, Data: data})
}
