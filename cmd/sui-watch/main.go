package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/sui"
	"github.com/probe-lab/whisker/walrus"
)

func main() {
	app := &cli.Command{
		Name:  "sui-watch",
		Usage: "Watch Walrus events on Sui and print them to stdout as newline-delimited JSON",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "rpc-url",
				Usage:    "Sui JSON-RPC endpoint URL",
				Sources:  cli.EnvVars("SUI_RPC_URL"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "package",
				Usage:    "Walrus package ID on Sui",
				Sources:  cli.EnvVars("WALRUS_PACKAGE_ID"),
				Required: true,
			},
			&cli.DurationFlag{
				Name:    "poll-interval",
				Usage:   "how often to poll for new events when caught up",
				Value:   5 * time.Second,
				Sources: cli.EnvVars("POLL_INTERVAL"),
			},
			&cli.StringFlag{
				Name:    "cursor",
				Usage:   "JSON-encoded EventCursor to resume from (omit to start from latest)",
				Sources: cli.EnvVars("START_CURSOR"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error)",
				Value:   "info",
				Sources: cli.EnvVars("LOG_LEVEL"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			level := slog.LevelInfo
			if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
				return ctx, err
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
			return ctx, nil
		},
		Action: run,
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	client := sui.NewClient(cmd.String("rpc-url"))
	filter := sui.MoveEventModuleFilter(cmd.String("package"), "events")
	pollInterval := cmd.Duration("poll-interval")

	var cursor *sui.EventCursor
	if raw := cmd.String("cursor"); raw != "" {
		cursor = new(sui.EventCursor)
		if err := json.Unmarshal([]byte(raw), cursor); err != nil {
			return fmt.Errorf("parse cursor: %w", err)
		}
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if cursor == nil {
		var err error
		cursor, err = client.LatestEventCursor(ctx, filter)
		if err != nil {
			return fmt.Errorf("fetch latest cursor: %w", err)
		}
		slog.Debug("starting from latest cursor", "cursor", cursor)
	}

	enc := json.NewEncoder(os.Stdout)

	slog.Info("watching walrus events", "rpc", cmd.String("rpc-url"), "package", cmd.String("package"))

	for {
		page, err := client.QueryEvents(ctx, filter, cursor, 50)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("query events failed", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pollInterval):
			}
			continue
		}

		for _, ev := range page.Data {
			envelope, err := walrus.ParseEvent(ev)
			if err != nil {
				slog.Warn("skipping unrecognised event", "type", ev.Type, "err", err)
				continue
			}
			if err := enc.Encode(envelope); err != nil {
				return fmt.Errorf("encode event: %w", err)
			}
		}

		if page.NextCursor != nil {
			cursor = page.NextCursor
		}

		if !page.HasNextPage {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(pollInterval):
			}
		}
	}
}
