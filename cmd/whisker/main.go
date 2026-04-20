package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/probe"
	"github.com/probe-lab/whisker/sui"
	"github.com/probe-lab/whisker/wait"
	"github.com/probe-lab/whisker/walrus"
)

func main() {
	app := &cli.Command{
		Name:  "whisker",
		Usage: "Walrus availability monitor",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error)",
				Value:   "info",
				Sources: cli.EnvVars("WHISKER_LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "publisher",
				Usage:   "Walrus publisher base URL",
				Value:   testnetPublisher,
				Sources: cli.EnvVars("WHISKER_PUBLISHER_URL"),
			},
			&cli.StringFlag{
				Name:    "aggregator",
				Usage:   "Walrus aggregator base URL",
				Value:   testnetAggregator,
				Sources: cli.EnvVars("WHISKER_AGGREGATOR_URL"),
			},
			&cli.StringFlag{
				Name:    "rpc-url",
				Usage:   "Sui JSON-RPC endpoint URL",
				Value:   testnetRPCURL,
				Sources: cli.EnvVars("WHISKER_SUI_RPC_URL"),
			},
			&cli.StringFlag{
				Name:    "package",
				Usage:   "Walrus package ID on Sui",
				Value:   defaultPackageID,
				Sources: cli.EnvVars("WHISKER_WALRUS_PACKAGE_ID"),
			},
			&cli.DurationFlag{
				Name:    "interval",
				Usage:   "how often to run a storage check",
				Value:   5 * time.Minute,
				Sources: cli.EnvVars("WHISKER_CHECK_INTERVAL"),
			},
			&cli.DurationFlag{
				Name:    "delay",
				Usage:   "initial delay before the first storage check",
				Value:   0,
				Sources: cli.EnvVars("WHISKER_CHECK_DELAY"),
			},
			&cli.Float64Flag{
				Name:    "jitter",
				Usage:   "jitter factor in [0,1) applied to delay and interval",
				Value:   0.1,
				Sources: cli.EnvVars("WHISKER_CHECK_JITTER"),
			},
			&cli.Int64Flag{
				Name:    "probe-size",
				Usage:   "size in bytes of the file uploaded in each storage check",
				Value:   1 * 1024 * 1024,
				Sources: cli.EnvVars("WHISKER_PROBE_SIZE"),
			},
			&cli.DurationFlag{
				Name:    "event-timeout",
				Usage:   "how long to wait for BlobCertified before giving up",
				Value:   10 * time.Minute,
				Sources: cli.EnvVars("WHISKER_EVENT_TIMEOUT"),
			},
			&cli.DurationFlag{
				Name:    "poll-interval",
				Usage:   "Sui event polling interval",
				Value:   5 * time.Second,
				Sources: cli.EnvVars("WHISKER_POLL_INTERVAL"),
			},
			&cli.UintFlag{
				Name:    "probe-epochs",
				Usage:   "number of storage epochs for uploaded probe blobs",
				Value:   1,
				Sources: cli.EnvVars("WHISKER_PROBE_EPOCHS"),
			},
			&cli.StringFlag{
				Name:    "tmp-dir",
				Usage:   "directory for temporary probe files",
				Value:   os.TempDir(),
				Sources: cli.EnvVars("WHISKER_TMP_DIR"),
			},
			&cli.BoolFlag{
				Name:    "dry-run",
				Usage:   "log results only; do not write to any persistent backend",
				Sources: cli.EnvVars("WHISKER_DRY_RUN"),
			},
			&cli.StringFlag{
				Name:    "json-out",
				Usage:   "directory to write newline-delimited JSON result files; one file per run",
				Sources: cli.EnvVars("WHISKER_JSON_OUT"),
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

const (
	testnetPublisher  = "https://publisher.walrus-testnet.walrus.space"
	testnetAggregator = "https://aggregator.walrus-testnet.walrus.space"
	testnetRPCURL     = "https://fullnode.testnet.sui.io:443"
	defaultPackageID  = "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"
)

func run(ctx context.Context, cmd *cli.Command) error {
	runID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate run ID: %w", err)
	}

	checker := &probe.StorageChecker{
		RunID:        runID.String(),
		Publisher:    walrus.NewPublisherClient(cmd.String("publisher")),
		Aggregator:   walrus.NewAggregatorClient(cmd.String("aggregator")),
		Sui:          sui.NewClient(cmd.String("rpc-url")),
		PackageID:    cmd.String("package"),
		PollInterval: cmd.Duration("poll-interval"),
		EventTimeout: cmd.Duration("event-timeout"),
		UploadOpts: walrus.UploadOptions{
			Epochs:    uint32(cmd.Uint("probe-epochs")),
			Deletable: true,
		},
	}

	dir := cmd.String("tmp-dir")
	size := cmd.Int64("probe-size")

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	slog.Info("starting storage check loop",
		"interval", cmd.Duration("interval"),
		"delay", cmd.Duration("delay"),
		"probe_size", size,
		"publisher", cmd.String("publisher"),
		"aggregator", cmd.String("aggregator"),
	)

	writer, err := chooseWriter(cmd, runID.String())
	if err != nil {
		return err
	}
	if closer, ok := writer.(io.Closer); ok {
		defer closer.Close()
	}

	loopErr := wait.Forever(ctx, func(ctx context.Context) error {
		result, err := checker.Check(ctx, dir, size)
		if err != nil {
			slog.Error("storage check failed", "err", err)
			return nil // log and continue; don't stop the loop on individual failures
		}
		if err := writer.WriteStorageCheckResult(ctx, result); err != nil {
			slog.Error("write result failed", "err", err)
		}
		return nil
	}, cmd.Duration("delay"), cmd.Duration("interval"), cmd.Float64("jitter"))

	if errors.Is(loopErr, context.Canceled) {
		return nil
	}
	return loopErr
}

func chooseWriter(cmd *cli.Command, runID string) (probe.ResultWriter, error) {
	if cmd.Bool("dry-run") {
		return &probe.LogWriter{}, nil
	}
	if dir := cmd.String("json-out"); dir != "" {
		return probe.NewJSONFileWriter(dir, runID)
	}
	// Default: log results. Replaced by ClickHouse writer once pw-0ktl lands.
	return &probe.LogWriter{}, nil
}
