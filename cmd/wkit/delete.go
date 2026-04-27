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
	walrusTestnetPackageID    = "0x849e95d2718938d66c37fb91df76d72f78526c1864c339bac415ce8ecda2d8cc"
	walrusMainnetSystemObject = "0x2134d52768ea07e8c43570ef975eb3e4c27a39fa6396bef985b5abc58d03ddd2"
	walrusMainnetPackageID    = "0xfdc88f7d7cf30afab2f82e8380d11ee8f70efb90e863d1de8616fae1bb09ea77"
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
				Name:    "package",
				Usage:   "Walrus package ID",
				Sources: cli.EnvVars("WKIT_DELETE_PACKAGE_ID"),
			},
			&cli.StringFlag{
				Name:    "network",
				Usage:   "network preset: testnet or mainnet (sets --system and --package defaults)",
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

	exec, err := sui.NewTransactionExecutor(cmd.String("rpc-url"), signer)
	if err != nil {
		return fmt.Errorf("create executor: %w", err)
	}

	systemObjectID, explicitPackageID := resolveNetworkDefaults(cmd)

	// Derive the live package ID from the system object type unless overridden.
	packageID := explicitPackageID
	if cmd.String("package") == "" {
		derived, err := packageIDFromSystemObject(ctx, exec, systemObjectID)
		if err != nil {
			slog.Warn("could not derive package ID from system object, using preset", "err", err, "package", packageID)
		} else {
			slog.Debug("derived package ID from system object", "package", derived)
			packageID = derived
		}
	}

	slog.Info("deleting blob", "object_id", blobObjectID, "package", packageID)

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
		sdkmodels.SuiAddress(packageID),
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

// packageIDFromSystemObject reads the current Walrus package ID from the system
// object's content. The system object stores a `package_id` field that is updated
// on each upgrade, giving us the live package ID without following the upgrade chain.
func packageIDFromSystemObject(ctx context.Context, exec *sui.TransactionExecutor, systemObjectID string) (string, error) {
	resp, err := exec.ObjectContent(ctx, systemObjectID)
	if err != nil {
		return "", err
	}
	pkg, ok := resp["package_id"]
	if !ok {
		return "", fmt.Errorf("system object has no package_id field")
	}
	pkgID, ok := pkg.(string)
	if !ok || pkgID == "" {
		return "", fmt.Errorf("system object package_id is not a string: %v", pkg)
	}
	return pkgID, nil
}

func resolveNetworkDefaults(cmd *cli.Command) (systemObjectID, packageID string) {
	network := cmd.String("network")

	switch network {
	case "mainnet":
		systemObjectID = walrusMainnetSystemObject
		packageID = walrusMainnetPackageID
	default:
		systemObjectID = walrusTestnetSystemObject
		packageID = walrusTestnetPackageID
	}

	// explicit flags override network presets
	if v := cmd.String("system"); v != "" {
		systemObjectID = v
	}
	if v := cmd.String("package"); v != "" {
		packageID = v
	}
	return systemObjectID, packageID
}
