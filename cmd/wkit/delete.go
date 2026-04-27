package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	sdkmodels "github.com/block-vision/sui-go-sdk/models"
	sdktx "github.com/block-vision/sui-go-sdk/transaction"
	"github.com/urfave/cli/v3"

	"github.com/probe-lab/whisker/pkg/sui"
)

const (
	walrusTestnetSystemObject = "0x6c2547cbbc38025cf3adac45f63cb0a8d12ecf777cdc75a4971612bf97fdf6af"
	walrusMainnetSystemObject = "0x2134d52768ea07e8c43570ef975eb3e4c27a39fa6396bef985b5abc58d03ddd2"
)

func deleteCommand() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete a deletable Walrus blob by Sui object ID",
		ArgsUsage: "<sui-object-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "rpc-url",
				Usage:    "Sui JSON-RPC endpoint URL",
				Sources:  cli.EnvVars("WKIT_DELETE_RPC_URL"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "system",
				Usage:   "Walrus system object ID",
				Sources: cli.EnvVars("WKIT_DELETE_SYSTEM_OBJECT"),
			},
			&cli.StringFlag{
				Name:    "network",
				Usage:   "network preset: testnet or mainnet (sets --system default)",
				Value:   "testnet",
				Sources: cli.EnvVars("WKIT_DELETE_NETWORK"),
			},
			&cli.Uint64Flag{
				Name:    "gas-budget",
				Usage:   "gas budget in MIST (default 0.05 SUI)",
				Sources: cli.EnvVars("WKIT_DELETE_GAS_BUDGET"),
			},
		},
		Action: runDelete,
	}
}

func runDelete(ctx context.Context, cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return fmt.Errorf("sui object ID required")
	}
	blobObjectID := cmd.Args().First()

	privateKey := cmd.Root().String("private-key")
	if privateKey == "" {
		return fmt.Errorf("--private-key or WHISKER_SUI_SIGNER is required")
	}
	signer, err := sui.LoadSigner(privateKey)
	if err != nil {
		return fmt.Errorf("load signer: %w", err)
	}
	slog.Debug("loaded signer", "address", signer.Address)

	rpcURL := cmd.String("rpc-url")
	exec, err := sui.NewTransactionExecutor(rpcURL, signer)
	if err != nil {
		return fmt.Errorf("create executor: %w", err)
	}

	systemObjectID := resolveSystemObject(cmd)

	sysInfo, err := sui.NewClient(rpcURL).FetchWalrusSystemInfo(ctx, systemObjectID)
	if err != nil {
		return fmt.Errorf("discover walrus package ID: %w", err)
	}
	slog.Debug("discovered package ID", "tx_package_id", sysInfo.TxPackageID)

	slog.Info("deleting blob", "object_id", blobObjectID, "package", sysInfo.TxPackageID)

	systemArg, err := exec.ResolveObject(ctx, systemObjectID, false)
	if err != nil {
		return fmt.Errorf("resolve system object: %w", err)
	}

	blobArg, err := exec.ResolveObject(ctx, blobObjectID, false)
	if err != nil {
		return fmt.Errorf("resolve blob object: %w", err)
	}

	tx := exec.NewTransaction()

	systemInput := tx.Object(systemArg)
	blobInput := tx.Object(blobArg)

	// delete_blob(system: &System, blob: Blob) returns Storage
	storage := tx.MoveCall(
		sdkmodels.SuiAddress(sysInfo.TxPackageID),
		"system",
		"delete_blob",
		nil,
		[]sdktx.Argument{systemInput, blobInput},
	)

	// transfer the reclaimed storage resource back to the sender
	tx.TransferObjects([]sdktx.Argument{storage}, tx.Pure(signer.Address))

	if err := exec.AutoSelectGas(ctx, tx, cmd.Uint64("gas-budget")); err != nil {
		return fmt.Errorf("select gas: %w", err)
	}

	digest, err := exec.Execute(ctx, tx)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, digest)
	return nil
}

func resolveSystemObject(cmd *cli.Command) string {
	if v := cmd.String("system"); v != "" {
		return v
	}
	if cmd.String("network") == "mainnet" {
		return walrusMainnetSystemObject
	}
	return walrusTestnetSystemObject
}
