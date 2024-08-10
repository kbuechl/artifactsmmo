package player

import (
	"artifactsmmo/internal/models"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"net/http"
)

func (p *Player) AcceptNewTask(tile models.MapTile) int {
	if code := p.move(tile.X, tile.Y); code != http.StatusOK {
		return code
	}

	p.logger.Debug("getting new task")
	resp, err := p.client.ActionAcceptNewTaskMyNameActionTaskNewPostWithResponse(p.ctx, p.Name)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != 200 {
		return resp.StatusCode()
	}

	p.logger.Info("got new task", "task", resp.JSON200.Data.Task)
	p.UpdateData(resp.JSON200.Data.Character)

	return resp.StatusCode()
}

func (p *Player) CompleteTask(tile models.MapTile) (*client.TaskRewardSchema, int) {
	if code := p.move(tile.X, tile.Y); code != http.StatusOK {
		return nil, code
	}
	p.logger.Debug("completing task")
	resp, err := p.client.ActionCompleteTaskMyNameActionTaskCompletePostWithResponse(p.ctx, p.Name)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode() != 200 {
		return nil, resp.StatusCode()
	}
	p.logger.Info("completed task", "reward", resp.JSON200.Data.Reward)
	p.UpdateData(resp.JSON200.Data.Character)

	return &resp.JSON200.Data.Reward, resp.StatusCode()
}

func (p *Player) ExchangeTaskCoins(tile models.MapTile) (*client.TaskRewardSchema, int) {
	if code := p.move(tile.X, tile.Y); code != http.StatusOK {
		return nil, code
	}
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
