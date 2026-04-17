package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/walrus"
)

func main() {
	app := &cli.Command{
		Name:      "whisker-publish",
		Usage:     "Upload a file to a Walrus publisher",
		ArgsUsage: "<file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "publisher",
				Usage:    "Walrus publisher base URL",
				Sources:  cli.EnvVars("WHISKER_PUBLISH_PUBLISHER_URL"),
				Required: true,
			},
			&cli.UintFlag{
				Name:    "epochs",
				Usage:   "number of epochs to store the blob (0 uses publisher default)",
				Value:   1,
				Sources: cli.EnvVars("WHISKER_PUBLISH_EPOCHS"),
			},
			&cli.BoolFlag{
				Name:    "deletable",
				Usage:   "make the blob deletable by the owner",
				Sources: cli.EnvVars("WHISKER_PUBLISH_DELETABLE"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error)",
				Value:   "info",
				Sources: cli.EnvVars("WHISKER_PUBLISH_LOG_LEVEL"),
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
	if cmd.NArg() == 0 {
		return fmt.Errorf("file path required")
	}
	filePath := cmd.Args().First()

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	opts := walrus.UploadOptions{
		Epochs:    uint32(cmd.Uint("epochs")),
		Deletable: cmd.Bool("deletable"),
	}

	client := walrus.NewPublisherClient(cmd.String("publisher"))

	slog.Info("uploading blob", "file", filePath, "size", info.Size(), "epochs", opts.Epochs, "deletable", opts.Deletable)

	result, err := client.UploadBlob(ctx, f, info.Size(), opts)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	return printResult(result)
}

func printResult(result *walrus.UploadResult) error {
	var out any

	switch {
	case result.NewlyCreated != nil:
		nc := result.NewlyCreated
		out = map[string]any{
			"status":          "newly_created",
			"blob_id":         nc.BlobID,
			"sui_object_id":   nc.SuiObjectID,
			"certified_epoch": nc.CertifiedEpoch,
			"end_epoch":       nc.EndEpoch,
			"cost":            nc.Cost,
			"deletable":       nc.Deletable,
		}
	case result.AlreadyCertified != nil:
		ac := result.AlreadyCertified
		out = map[string]any{
			"status":    "already_certified",
			"blob_id":   ac.BlobID,
			"tx_digest": ac.TxDigest,
			"event_seq": ac.EventSeq,
			"end_epoch": ac.EndEpoch,
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
