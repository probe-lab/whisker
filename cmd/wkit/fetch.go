package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/pkg/network"
	"github.com/probe-lab/whisker/pkg/walrus"
)

func fetchCommand() *cli.Command {
	return &cli.Command{
		Name:      "fetch",
		Usage:     "Fetch a blob from a Walrus aggregator",
		ArgsUsage: "<blob-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "aggregator",
				Usage:       "Walrus aggregator base URL",
				DefaultText: "derived from --network",
				Sources:     cli.EnvVars("WKIT_FETCH_AGGREGATOR"),
			},
			&cli.StringFlag{
				Name:    "network",
				Usage:   "network preset: testnet or mainnet (sets --aggregator default)",
				Value:   "testnet",
				Sources: cli.EnvVars("WKIT_FETCH_NETWORK"),
			},
			&cli.StringFlag{
				Name:    "out",
				Usage:   "output file path (default: blob ID as filename)",
				Sources: cli.EnvVars("WKIT_FETCH_OUT"),
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Usage:   "request timeout",
				Value:   60 * time.Second,
				Sources: cli.EnvVars("WKIT_FETCH_TIMEOUT"),
			},
		},
		Action: runFetch,
	}
}

func runFetch(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return fmt.Errorf("blob ID required")
	}
	blobID := cmd.Args().First()

	outPath := cmd.String("out")
	if outPath == "" {
		outPath = blobID
	}

	cfg, err := network.Defaults(cmd.String("network"))
	if err != nil {
		return err
	}
	client := walrus.NewAggregatorClient(flagOr(cmd, "aggregator", cfg.Aggregator))
	client.HTTPClient.Timeout = cmd.Duration("timeout")

	slog.Info("fetching blob", "blob_id", blobID, "out", outPath)

	tmp, err := os.CreateTemp(filepath.Dir(outPath), ".wkit-fetch-*")
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
