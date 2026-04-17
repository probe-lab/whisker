package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/sui"
	"github.com/probe-lab/whisker/walrus"
)

func main() {
	app := &cli.Command{
		Name:  "whisker-watch",
		Usage: "Watch Walrus events on Sui and print them to stdout as newline-delimited JSON",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "rpc-url",
				Usage:    "Sui JSON-RPC endpoint URL",
				Sources:  cli.EnvVars("WHISKER_WATCH_RPC_URL"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "package",
				Usage:    "Walrus package ID on Sui",
				Sources:  cli.EnvVars("WHISKER_WATCH_PACKAGE_ID"),
				Required: true,
			},
			&cli.DurationFlag{
				Name:    "poll-interval",
				Usage:   "how often to poll for new events when caught up",
				Value:   5 * time.Second,
				Sources: cli.EnvVars("WHISKER_WATCH_POLL_INTERVAL"),
			},
			&cli.StringFlag{
				Name:    "cursor",
				Usage:   "JSON-encoded EventCursor to resume from (omit to start from latest)",
				Sources: cli.EnvVars("WHISKER_WATCH_CURSOR"),
			},
			&cli.BoolFlag{
				Name:    "human",
				Usage:   "print events in human-readable format instead of JSON",
				Sources: cli.EnvVars("WHISKER_WATCH_HUMAN"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error)",
				Value:   "info",
				Sources: cli.EnvVars("WHISKER_WATCH_LOG_LEVEL"),
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
	human := cmd.Bool("human")

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
			if human {
				printHuman(envelope)
			} else {
				if err := enc.Encode(envelope); err != nil {
					return fmt.Errorf("encode event: %w", err)
				}
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

func printHuman(env *walrus.EventEnvelope) {
	ts := "-"
	if env.TimestampMs != "" {
		if ms, err := strconv.ParseInt(env.TimestampMs, 10, 64); err == nil {
			ts = time.UnixMilli(ms).UTC().Format("2006-01-02 15:04:05 UTC")
		}
	}

	switch e := env.Event.(type) {
	case *walrus.BlobRegistered:
		fmt.Printf("%s  %-15s  %s  epoch=%d end_epoch=%d size=%s\n",
			ts, env.EventType, formatBlobID(e.BlobID), e.Epoch, e.EndEpoch, e.Size)
	case *walrus.BlobCertified:
		fmt.Printf("%s  %-15s  %s  epoch=%d end_epoch=%d\n",
			ts, env.EventType, formatBlobID(e.BlobID), e.Epoch, e.EndEpoch)
	case *walrus.BlobDeleted:
		fmt.Printf("%s  %-15s  %s  epoch=%d end_epoch=%d\n",
			ts, env.EventType, formatBlobID(e.BlobID), e.Epoch, e.EndEpoch)
	}
}

// formatBlobID returns the base64url form of a blob ID for display.
// Falls back to the original string if conversion fails.
func formatBlobID(id string) string {
	b64, err := walrus.BlobIDBase64(id)
	if err != nil {
		return id
	}
	return b64
}
