package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/redis/go-redis/v9"
	"github.com/urfave/cli/v2"

	"github.com/venkytv/homemon/backend"
	"github.com/venkytv/homemon/netatmo"
)

const (
	Prefix = "homemon"
)

func main() {

	ctx := context.Background()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	var configDir string
	var redisAddress string
	var debug bool

	app := &cli.App{
		Name:  "homemon",
		Usage: "Monitor your home",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config-dir",
				Usage:       "Configuration directory",
				Value:       homeDir + "/.config/homemon",
				TakesFile:   true,
				Destination: &configDir,
			},
			&cli.StringFlag{
				Name:        "redis-address",
				Usage:       "Redis address",
				Value:       "localhost:6379",
				Destination: &redisAddress,
			},
			&cli.BoolFlag{
				Name:        "debug",
				Usage:       "Enable debug mode",
				Value:       false,
				Destination: &debug,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "netatmo",
				Usage: "Netatmo commands",
				Subcommands: []*cli.Command{
					{
						Name:  "record-metrics",
						Usage: "Start metrics recording service",
						Action: func(c *cli.Context) error {
							config, err := initialize(ctx, configDir, redisAddress, debug)
							if err != nil {
								log.Fatal(err)
							}
							netatmo.RecordMetrics(ctx, config)
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func initialize(_ context.Context, configDir string, redisAddr string, debug bool) (*backend.Config, error) {
	// Configure the logger
	var programLevel = new(slog.LevelVar)
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel})
	slog.SetDefault(slog.New(h))
	if debug {
		fmt.Println("Setting log level to debug")
		programLevel.Set(slog.LevelDebug)
	}

	// Initialize the configuration directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	config := &backend.Config{}
	config.ConfigDir = configDir

	// Initialize the resty client
	restyClient := resty.New()
	restyClient.SetContentLength(true)
	restyClient.SetHeader("Content-Type", "application/json")
	restyClient.SetDebug(debug)
	restyClient.SetTimeout(30 * time.Second)
	config.RestyClient = restyClient

	// Initialize the redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	config.RedisClient = redisClient

	// Initialize the publisher
	config.Publisher = backend.NewPublisher(redisAddr, Prefix)

	return config, nil
}
