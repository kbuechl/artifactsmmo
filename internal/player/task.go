package player

import (
	"github.com/promiseofcake/artifactsmmo-go-client/client"
)

func (p *Player) AcceptNewTask() (*PlayerTask, int) {
	p.logger.Debug("getting new task for player" + p.Name)
	resp, err := p.client.ActionAcceptNewTaskMyNameActionTaskNewPostWithResponse(p.ctx, p.Name)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != 200 {
		return nil, resp.StatusCode()
	}

	p.logger.Info("got new task", "task", resp.JSON200.Data.Task)
	p.UpdateData(resp.JSON200.Data.Character)

	return p.Data().Task, resp.StatusCode()
}

func (p *Player) CompleteTask() (*client.TaskRewardSchema, int) {
	p.logger.Debug("completing task for player" + p.Name)
	resp, err := p.client.ActionCompleteTaskMyNameActionTaskCompletePostWithResponse(p.ctx, p.Name)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != 200 {
		return nil, resp.StatusCode()
	}
	p.logger.Info("completed task for player"+p.Name, "reward", resp.JSON200.Data.Reward)
	p.UpdateData(resp.JSON200.Data.Character)

	return &resp.JSON200.Data.Reward, resp.StatusCode()
}

func (p *Player) ExchangeTaskCoins() (*client.TaskRewardSchema, int) {
	resp, err := p.client.ActionTaskExchangeMyNameActionTaskExchangePostWithResponse(p.ctx, p.Name)

	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != 200 {
		return nil, resp.StatusCode()
	}

	p.UpdateData(resp.JSON200.Data.Character)
	return &resp.JSON200.Data.Reward, resp.StatusCode()
}
