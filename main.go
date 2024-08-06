package main

import (
	"artifactsmmo/internal/engine"
	"context"
	"fmt"
	"github.com/sagikazarmark/slog-shim"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const (
	urlKey = "url"
)

type config struct {
	Token   string   `yaml:"token"`
	URL     string   `yaml:"url"`
	Players []string `yaml:"players"`
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	exitOnError := errorHandler(cancel)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic caught: %v", r)
			sigChan <- syscall.SIGTERM
		}
	}()

	cfg := &config{}
	if err := viper.Unmarshal(cfg); err != nil {
		exitOnError(err)
	}

	if cfg.Token == "" {
		panic(fmt.Errorf("token not found in config"))
	}

	game, err := engine.NewGameEngine(ctx, engine.GameConfig{
		Token:       cfg.Token,
		URL:         cfg.URL,
		PlayerNames: cfg.Players,
	})

	exitOnError(err)

mainloop:
	for {
		select {
		case <-sigChan:
			log.Println("signal caught, stopping game")
			cancel()
			break mainloop
		case gErr := <-game.Out:
			exitOnError(gErr)
			break mainloop
		default:

		}
	}
	log.Println("game stopped")
}

func errorHandler(cancel context.CancelFunc) func(err error) {
	return func(err error) {
		if err != nil {
			cancel()
			log.Fatalln(err)
		}
	}
}
