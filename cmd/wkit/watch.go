package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/pkg/sui"
	"github.com/probe-lab/whisker/pkg/walrus"
)

func watchCommand() *cli.Command {
	return &cli.Command{
		Name:  "watch",
		Usage: "Watch Walrus events on Sui and print them to stdout as newline-delimited JSON",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "rpc-url",
				Usage:    "Sui JSON-RPC endpoint URL",
				Sources:  cli.EnvVars("WKIT_WATCH_RPC_URL"),
				Required: true,
			},
			&cli.StringFlag{
				Name:     "package",
				Usage:    "Walrus package ID on Sui",
				Sources:  cli.EnvVars("WKIT_WATCH_PACKAGE_ID"),
				Required: true,
			},
			&cli.DurationFlag{
				Name:    "poll-interval",
				Usage:   "how often to poll for new events when caught up",
				Value:   5 * time.Second,
				Sources: cli.EnvVars("WKIT_WATCH_POLL_INTERVAL"),
			},
			&cli.StringFlag{
				Name:    "cursor",
				Usage:   "JSON-encoded EventCursor to resume from (omit to start from latest)",
				Sources: cli.EnvVars("WKIT_WATCH_CURSOR"),
			},
			&cli.BoolFlag{
				Name:    "human",
				Usage:   "print events in human-readable format instead of JSON",
				Sources: cli.EnvVars("WKIT_WATCH_HUMAN"),
			},
		},
		Action: runWatch,
	}
}

func runWatch(ctx context.Context, cmd *cli.Command) error {
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

	err := client.WatchEvents(ctx, filter, cursor, pollInterval, func(ev sui.Event) error {
		envelope, err := walrus.ParseEvent(ev)
		if err != nil {
			slog.Warn("skipping unrecognised event", "type", ev.Type, "err", err)
			return nil
		}
		if human {
			printHuman(envelope)
		} else {
			if err := enc.Encode(envelope); err != nil {
				return fmt.Errorf("encode event: %w", err)
			}
		}
		return nil
	})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
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

func formatBlobID(id string) string {
	b64, err := walrus.BlobIDBase64(id)
	if err != nil {
		return id
	}
	return b64
}
