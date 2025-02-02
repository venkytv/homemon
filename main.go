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
	var natsAddress string
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
			&cli.StringFlag{
				Name:        "nats-address",
				Usage:       "NATS address",
				Value:       "localhost:4222",
				Destination: &natsAddress,
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
							config, err := initialize(ctx, configDir, redisAddress, natsAddress, debug)
							if err != nil {
								log.Fatal(err)
							}
							netatmo.RecordMetrics(ctx, config)
							return nil
						},
					},
				},
			},
			{
				Name:  "metrics",
				Usage: "Metrics commands",
				Subcommands: []*cli.Command{
					{
						Name:  "publish",
						Usage: "Publish a metric",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Aliases:  []string{"n"},
								Usage:    "Name of the metric",
								Required: true,
							},
							&cli.IntFlag{
								Name:     "priority",
								Aliases:  []string{"p"},
								Usage:    "Priority of the metric",
								Required: true,
							},
							&cli.StringFlag{
								Name:     "colour",
								Aliases:  []string{"c"},
								Usage:    "Colour of the metric",
								Required: true,
							},
							&cli.DurationFlag{
								Name:     "ttl",
								Aliases:  []string{"t"},
								Usage:    "Time to live of the metric",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							config, err := initialize(ctx, configDir, redisAddress, natsAddress, debug)
							if err != nil {
								log.Fatal(err)
							}
							metric := backend.Metric{
								Name:     c.String("name"),
								Priority: c.Int("priority"),
								Colour:   c.String("colour"),
								TTL:      time.Now().Add(c.Duration("ttl")),
							}
							slog.Debug("Publishing metric", "metric", metric)
							if err := config.Publisher.Publish(ctx, metric); err != nil {
								log.Fatal(err)
							}
							return nil
						},
					},
					{
						Name:  "list",
						Usage: "List metrics",
						Action: func(c *cli.Context) error {
							config, err := initialize(ctx, configDir, redisAddress, natsAddress, debug)
							if err != nil {
								log.Fatal(err)
							}
							metrics, err := config.Publisher.ListMetrics(ctx, config)
							if err != nil {
								log.Fatal(err)
							}
							for _, metric := range metrics {
								fmt.Printf("%s: priority: %d, colour: %s, ttl: %s\n", metric.Name, metric.Priority, metric.Colour, metric.TTL)
							}
							return nil
						},
					},
					{
						Name:  "delete",
						Usage: "Delete a metric",
						Action: func(c *cli.Context) error {
							name := c.Args().First()
							if name == "" {
								log.Fatal("Name of the metric is required")
							}
							config, err := initialize(ctx, configDir, redisAddress, natsAddress, debug)
							if err != nil {
								log.Fatal(err)
							}
							if err := config.Publisher.DeleteMetric(ctx, name); err != nil {
								log.Fatal(err)
							}
							return nil
						},
					},
				},
			},
			{
				Name:  "cleanup",
				Usage: "Cleanup commands",
				Subcommands: []*cli.Command{
					{
						Name:  "metrics",
						Usage: "Cleanup metrics in redis",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "dry-run",
								Usage: "Dry run",
							},
						},
						Action: func(c *cli.Context) error {
							config, err := initialize(ctx, configDir, redisAddress, natsAddress, debug)
							if err != nil {
								log.Fatal(err)
							}
							backend.CleanupMetrics(ctx, config, c.Bool("dry-run"))
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

func initialize(_ context.Context, configDir string, redisAddr, natsAddr string, debug bool) (*backend.Config, error) {
	// Configure the logger
	var programLevel = new(slog.LevelVar)
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: programLevel})
	slog.SetDefault(slog.New(h))
	if debug {
		programLevel.Set(slog.LevelDebug)
		slog.Debug("Debug mode enabled")
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

	// Initialize the NATS client
	natsPublisher, err := backend.NewNATSPublisher(natsAddr)
	if err != nil {
		return nil, err
	}
	config.RawPublisher = natsPublisher

	return config, nil
}
