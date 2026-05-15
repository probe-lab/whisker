package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/pkg/network"
	"github.com/probe-lab/whisker/pkg/sui"
	"github.com/probe-lab/whisker/pkg/walrus"
)

var publishCmd = &cli.Command{
	Name:      "publish",
	Usage:     "Upload a file to a Walrus publisher",
	ArgsUsage: "<file>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "publisher",
			Usage:       "Walrus publisher base URL",
			DefaultText: "derived from --network",
			Sources:     cli.EnvVars("WHISKER_PUBLISH_PUBLISHER_URL"),
		},
		&cli.StringFlag{
			Name:    "network",
			Usage:   "network preset: testnet or mainnet (sets --publisher default)",
			Value:   "testnet",
			Sources: cli.EnvVars("WHISKER_PUBLISH_NETWORK"),
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
			Name:    "send-to",
			Usage:   "Sui address to receive the blob object (default: address derived from --private-key if set)",
			Sources: cli.EnvVars("WHISKER_PUBLISH_SEND_TO"),
		},
	},
	Action: runPublish,
}

func runPublish(ctx context.Context, cmd *cli.Command) error {
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

	sendTo := cmd.String("send-to")
	if sendTo == "" {
		if pk := cmd.Root().String("private-key"); pk != "" {
			signer, err := sui.LoadSigner(pk)
			if err != nil {
				return fmt.Errorf("derive address from private key: %w", err)
			}
			sendTo = signer.Address
		}
	}

	opts := walrus.UploadOptions{
		Epochs:    uint32(cmd.Uint("epochs")),
		Deletable: cmd.Bool("deletable"),
		SendTo:    sendTo,
	}

	cfg, err := network.Defaults(cmd.String("network"))
	if err != nil {
		return err
	}
	client := walrus.NewPublisherClient(resolveFlag(cmd, "publisher", cfg.Publisher))

	slog.Info("uploading blob", "file", filePath, "size", info.Size(), "epochs", opts.Epochs, "deletable", opts.Deletable, "send_to", opts.SendTo)

	result, err := client.UploadBlob(ctx, f, info.Size(), opts)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	return printUploadResult(result)
}

func printUploadResult(result *walrus.UploadResult) error {
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
