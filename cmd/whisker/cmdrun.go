package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	pldb "github.com/probe-lab/go-commons/db"
	"github.com/urfave/cli/v3"
	"go.opentelemetry.io/otel"

	"github.com/probe-lab/whisker/pkg/db"
	"github.com/probe-lab/whisker/pkg/probe"
	"github.com/probe-lab/whisker/pkg/sui"
	"github.com/probe-lab/whisker/pkg/wait"
	"github.com/probe-lab/whisker/pkg/walrus"
)

const (
	testnetPublisher  = "https://publisher.walrus-testnet.walrus.space"
	testnetAggregator = "https://aggregator.walrus-testnet.walrus.space"
	testnetRPCURL     = "https://fullnode.testnet.sui.io:443"
	// defaultSystemObjectID is the Walrus system object on Sui testnet.
	// Both package IDs are derived from this object at startup.
	defaultSystemObjectID = "0x6c2547cbbc38025cf3adac45f63cb0a8d12ecf777cdc75a4971612bf97fdf6af"
)

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "Start probes",
	Flags: []cli.Flag{
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
		&cli.Int64SliceFlag{
			Name:    "probe-size",
			Usage:   "size in bytes of the file uploaded in each storage check, comma separated list",
			Value:   []int64{100 * 1024, 1 * 1024 * 1024, 10 * 1024 * 1024},
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
			Usage:   "run probes normally but do not write metrics to any persistent store (ClickHouse etc.)",
			Sources: cli.EnvVars("WHISKER_DRY_RUN"),
		},
		&cli.StringFlag{
			Name:    "json-out",
			Usage:   "directory to write newline-delimited JSON result files; one file per run",
			Sources: cli.EnvVars("WHISKER_JSON_OUT"),
		},
		&cli.StringFlag{
			Name:    "signer",
			Usage:   "Sui private key (suiprivkey-prefixed bech32) or BIP-39 mnemonic; enables blob deletion and storage recycling after each probe",
			Sources: cli.EnvVars("WHISKER_SUI_SIGNER"),
		},
		&cli.StringFlag{
			Name:    "walrus-system-object",
			Usage:   "Walrus system object ID on Sui",
			Value:   defaultSystemObjectID,
			Sources: cli.EnvVars("WHISKER_WALRUS_SYSTEM_OBJECT_ID"),
		},
		&cli.StringFlag{
			Name:    "network",
			Usage:   "network name written to probe results (mainnet or testnet)",
			Value:   "testnet",
			Sources: cli.EnvVars("WHISKER_NETWORK"),
		},
		&cli.StringFlag{
			Name:    "probe-location",
			Usage:   "location identifier written to probe results",
			Sources: cli.EnvVars("WHISKER_PROBE_LOCATION"),
		},
		&cli.StringFlag{
			Name:    "clickhouse-url",
			Usage:   "ClickHouse connection URL (clickhouse://user:pass@host:port/db)",
			Sources: cli.EnvVars("WHISKER_CLICKHOUSE_URL"),
		},
	},
	Action: run,
}

func run(ctx context.Context, cmd *cli.Command) error {
	runID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate run ID: %w", err)
	}

	suiClient := sui.NewClient(cmd.String("rpc-url"))

	sysInfo, err := suiClient.FetchWalrusSystemInfo(ctx, cmd.String("walrus-system-object"))
	if err != nil {
		return fmt.Errorf("discover walrus package IDs: %w", err)
	}
	slog.Debug("discovered walrus package IDs",
		"package_id", sysInfo.PackageID,
		"tx_package_id", sysInfo.TxPackageID,
	)

	checker := &probe.StorageChecker{
		RunID:        runID.String(),
		Publisher:    walrus.NewPublisherClient(cmd.String("publisher")),
		Aggregator:   walrus.NewAggregatorClient(cmd.String("aggregator")),
		Sui:          suiClient,
		TxPackageID:  sysInfo.TxPackageID,
		PollInterval: cmd.Duration("poll-interval"),
		EventTimeout: cmd.Duration("event-timeout"),
		UploadOpts: walrus.UploadOptions{
			Epochs:    uint32(cmd.Uint("probe-epochs")),
			Deletable: true,
		},
	}

	if secret := cmd.String("signer"); secret != "" {
		signer, err := sui.LoadSigner(secret)
		if err != nil {
			return fmt.Errorf("load signer: %w", err)
		}
		executor, err := sui.NewTransactionExecutor(cmd.String("rpc-url"), signer)
		if err != nil {
			return fmt.Errorf("create transaction executor: %w", err)
		}
		checker.Executor = executor
		checker.SystemObjectID = cmd.String("walrus-system-object")
	}

	dir := cmd.String("tmp-dir")
	sizes := cmd.Int64Slice("probe-size")

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	metrics, err := newProbeMetrics(otel.GetMeterProvider())
	if err != nil {
		return fmt.Errorf("create metrics: %w", err)
	}

	slog.Info("starting storage check loop",
		"interval", cmd.Duration("interval"),
		"delay", cmd.Duration("delay"),
		"probe_sizes", sizes,
		"publisher", cmd.String("publisher"),
		"aggregator", cmd.String("aggregator"),
	)

	writer, err := chooseWriter(ctx, cmd, runID.String())
	if err != nil {
		return err
	}
	if closer, ok := writer.(io.Closer); ok {
		defer closer.Close()
	}

	loopErr := wait.Forever(ctx, func(ctx context.Context) error {
		size := sizes[rand.N(len(sizes))]
		result, err := checker.Check(ctx, dir, size)
		if err != nil {
			slog.Error("storage check failed", "err", err)
			metrics.recordError(ctx, size)
			return nil // log and continue; don't stop the loop on individual failures
		}
		metrics.record(ctx, result)
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

func chooseWriter(ctx context.Context, cmd *cli.Command, runID string) (probe.ResultWriter, error) {
	if cmd.Bool("dry-run") {
		return &probe.LogWriter{}, nil
	}
	if dir := cmd.String("json-out"); dir != "" {
		return probe.NewJSONFileWriter(dir, runID)
	}
	if rawURL := cmd.String("clickhouse-url"); rawURL != "" {
		chCfg, err := parseClickHouseURL(rawURL)
		if err != nil {
			return nil, fmt.Errorf("parse clickhouse URL: %w", err)
		}
		client, err := db.NewClickhouseClient(ctx, chCfg, pldb.DefaultClickHouseMigrationsConfig())
		if err != nil {
			return nil, fmt.Errorf("create clickhouse client: %w", err)
		}
		client.Network = cmd.String("network")
		client.ProbeLocation = cmd.String("probe-location")
		client.PublisherURL = cmd.String("publisher")
		client.AggregatorURL = cmd.String("aggregator")
		return client, nil
	}
	return &probe.LogWriter{}, nil
}

func parseClickHouseURL(rawURL string) (*pldb.ClickHouseConfig, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	port := 9000
	if p := u.Port(); p != "" {
		if port, err = strconv.Atoi(p); err != nil {
			return nil, fmt.Errorf("invalid port %q: %w", p, err)
		}
	}

	pass, _ := u.User.Password()
	ssl := u.Scheme == "clickhouses" || u.Query().Get("ssl") == "true" || u.Query().Get("secure") == "true"

	return &pldb.ClickHouseConfig{
		BaseConfig: &pldb.ClickHouseBaseConfig{
			Host: u.Hostname(),
			Port: port,
			User: u.User.Username(),
			Pass: pass,
			SSL:  ssl,
		},
		Database: strings.TrimPrefix(u.Path, "/"),
	}, nil
}
