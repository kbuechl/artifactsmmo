package main

import (
	"artifactsmmo/internal"
	"artifactsmmo/internal/runner"
	"context"
	"fmt"
	"github.com/promiseofcake/artifactsmmo-cli/client"
	"github.com/spf13/viper"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	tokenKey = "token"
	urlKey   = "url"
)

type config struct {
	Token string `yaml:"token"`
	URL   string `yaml:"url"`
	Name  string `yaml:"name"`
}

func init() {
	dir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	viper.SetConfigFile(dir + "/.artifactsmmo/config.yaml")
	viper.SetDefault(urlKey, "https://api.artifactsmmo.com")

	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	exitOnError := errorHandler(cancel)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	cfg := &config{}
	if err := viper.Unmarshal(cfg); err != nil {
		exitOnError(err)
	}

	if cfg.Token == "" {
		panic(fmt.Errorf("token not found in config"))
	}

	c, err := client.NewClientWithResponses(cfg.URL, client.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+cfg.Token)
		return nil
	}))
	exitOnError(err)

	mmoClient := &runner.Runner{
		Client: c,
	}

	dataCollector, err := internal.NewWorldCollector(ctx, mmoClient)
	exitOnError(err)
	player, err := internal.NewPlayer(ctx, cfg.Name, mmoClient)
	exitOnError(err)

	go func() {
		for {
			select {
			case err := <-dataCollector.Out:
				exitOnError(fmt.Errorf("error in collector:%w", err))
			case err := <-player.Out:
				exitOnError(fmt.Errorf("error in player:%w", err))
			case <-sigChan:
				cancel()
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			//using the character data find a resource we are allowed to gather
			playerData := player.CharacterData()
			fmt.Println("filtering map sections based on options")
			sections := dataCollector.GetGatherableMapSections(playerData.Skills)
			next := pickRandomMapSection(sections)

			fmt.Println("moving to space to gather resource")
			// move to that space
			resp, err := player.Move(next.X, next.Y)
			exitOnError(err)

			fmt.Printf("waiting for cooldown \n", resp.Cooldown.Seconds)
			time.Sleep(time.Second * time.Duration(resp.Cooldown.Seconds))
			//gather
			fmt.Println("moving to space to gather resource")
			gatherResp, err := player.Gather()
			exitOnError(err)
			fmt.Println("waiting for cooldown \n")
			time.Sleep(time.Second * time.Duration(gatherResp.Seconds))
		}

	}

}
func errorHandler(cancel context.CancelFunc) func(err error) {
	return func(err error) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			cancel()
			os.Exit(1)
		}
	}
}

func pickRandomMapSection(md []runner.MapData) runner.MapData {
	rand.NewSource(time.Now().UnixNano())
	return md[rand.Intn(len(md))]
}
