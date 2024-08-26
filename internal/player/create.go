package player

import (
	"fmt"
	"github.com/promiseofcake/artifactsmmo-go-client/client"
	"github.com/sagikazarmark/slog-shim"
	"math/rand"
	"net/http"
)

func (p *Player) createCharacter() error {
	skins := []string{"men1", "men2", "men3", "women1", "women2", "women3"}

	skin := skins[rand.Intn(len(skins)-1)]
	p.logger.Info("Create character", slog.Group("skin", skin))
	resp, err := p.client.CreateCharacterCharactersCreatePostWithResponse(p.ctx, client.CreateCharacterCharactersCreatePostJSONRequestBody{
		Name: p.Name,
		Skin: client.AddCharacterSchemaSkin(skin),
	})

	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("CreateCharactersCreatePost returned %s", resp.Status)
	}

	p.UpdateData(resp.JSON200.Data)

	return nil
}
