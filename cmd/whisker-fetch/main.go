package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/walrus"
)

func main() {
	app := &cli.Command{
		Name:      "whisker-fetch",
		Usage:     "Fetch a blob from a Walrus aggregator",
		ArgsUsage: "<blob-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "aggregator",
				Usage:    "Walrus aggregator base URL",
				Sources:  cli.EnvVars("WHISKER_FETCH_AGGREGATOR"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "out",
				Usage:   "output file path (default: blob ID as filename)",
				Sources: cli.EnvVars("WHISKER_FETCH_OUT"),
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Usage:   "request timeout",
				Value:   60 * time.Second,
				Sources: cli.EnvVars("WHISKER_FETCH_TIMEOUT"),
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error)",
				Value:   "info",
				Sources: cli.EnvVars("WHISKER_FETCH_LOG_LEVEL"),
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
		return fmt.Errorf("blob ID required")
	}
	blobID := cmd.Args().First()

	outPath := cmd.String("out")
	if outPath == "" {
		outPath = blobID
	}

	client := walrus.NewAggregatorClient(cmd.String("aggregator"))
	client.HTTPClient.Timeout = cmd.Duration("timeout")

	slog.Info("fetching blob", "blob_id", blobID, "out", outPath)

	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".whisker-fetch-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	result, err := client.FetchBlob(ctx, blobID, tmp)
	if err != nil {
		return fmt.Errorf("fetch blob: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpName, outPath); err != nil {
		return fmt.Errorf("save output file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "size:       %d bytes\n", result.Size)
	fmt.Fprintf(os.Stderr, "ttfb:       %s\n", result.TTFB.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "ttlb:       %s\n", result.TTLB.Round(time.Millisecond))
	fmt.Fprintf(os.Stderr, "throughput: %.1f KB/s\n", result.Throughput/1024)

	return nil
}
