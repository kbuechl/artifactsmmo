package main

import (
	"artifactsmmo/internal"
	"context"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
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

	//todo: refactor to have more character names in the config and start one game per
	game, err := internal.NewGameEngine(ctx, internal.GameConfig{
		Token: cfg.Token,
		URL:   cfg.URL,
		Name:  cfg.Name,
	})
	exitOnError(err)

	go game.Start()

	for {
		select {
		case <-sigChan:
			fmt.Println("signal caught, stopping game")
			cancel()
			break
		default:

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
