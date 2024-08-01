package internal

import (
	"artifactsmmo/internal/runner"
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	characterRefreshInterval time.Duration = time.Second * 5
)

type Player struct {
	Name   string
	data   *runner.CharacterStatus
	mu     sync.RWMutex
	ctx    context.Context
	client *runner.Runner
	Out    chan error
}

func NewPlayer(ctx context.Context, name string, client *runner.Runner) (*Player, error) {
	p := &Player{
		Name:   name,
		client: client,
		ctx:    ctx,
	}

	if err := p.updateCharacterData(); err != nil {
		return nil, err
	}

	p.start()

	return p, nil
}

func (p *Player) start() {
	go func() {
		ticker := time.NewTicker(characterRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				if err := p.updateCharacterData(); err != nil {
					//todo: if we get timed out we should just just continue
					p.Out <- err
				}
			}
		}
	}()
}

func (p *Player) updateCharacterData() error {
	status, err := p.client.GetPlayerStatus(p.ctx, p.Name)
	if err != nil {
		return fmt.Errorf("get player status: %w", err)
	}
	p.mu.Lock()
	p.data = status
	p.mu.Unlock()
	return nil
}

func (p *Player) CharacterData() *runner.CharacterStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.data
}

func (p *Player) CanGather(resource runner.Resource) bool {
	for skill, level := range p.data.Skills {
		if skill == resource.Skill && resource.Level <= level {
			return true
		}
	}
	return false
}

func (p *Player) Move(x, y int) (*runner.MoveResponse, error) {
	return p.client.Move(p.ctx, p.Name, x, y)
}

func (p *Player) Gather() (*runner.Cooldown, error) {
	return p.client.Gather(p.ctx, p.Name)
}
