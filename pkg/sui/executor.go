package sui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	sdkmodels "github.com/block-vision/sui-go-sdk/models"
	sdksigner "github.com/block-vision/sui-go-sdk/signer"
	sdksui "github.com/block-vision/sui-go-sdk/sui"
	sdktx "github.com/block-vision/sui-go-sdk/transaction"
)

// storageResourceTypeSuffix is the Move type suffix for Walrus Storage objects.
const storageResourceTypeSuffix = "::storage_resource::Storage"

const (
	suiCoinType      = "0x2::sui::SUI"
	defaultGasBudget = uint64(50_000_000) // 0.05 SUI
)

// LoadSigner loads an ed25519 Sui signer.
// secret may be a suiprivkey-prefixed bech32 encoded private key (as exported by the Sui CLI)
// or a BIP-39 mnemonic phrase.
func LoadSigner(secret string) (*sdksigner.Signer, error) {
	if strings.HasPrefix(secret, "suiprivkey") {
		return sdksigner.NewSignerWithSecretKey(secret)
	}
	return sdksigner.NewSignertWithMnemonic(secret)
}

// TransactionExecutor builds, signs, and submits Sui programmable transactions.
type TransactionExecutor struct {
	Signer *sdksigner.Signer
	client *sdksui.Client
}

// NewTransactionExecutor returns an executor for the given RPC endpoint and signer.
func NewTransactionExecutor(rpcURL string, sig *sdksigner.Signer) (*TransactionExecutor, error) {
	raw := sdksui.NewSuiClient(rpcURL)
	client, ok := raw.(*sdksui.Client)
	if !ok {
		return nil, fmt.Errorf("unexpected sui client type")
	}
	return &TransactionExecutor{Signer: sig, client: client}, nil
}

// NewTransaction returns a transaction pre-wired with the executor's signer and RPC client.
func (e *TransactionExecutor) NewTransaction() *sdktx.Transaction {
	tx := sdktx.NewTransaction()
	tx.SetSuiClient(e.client)
	tx.SetSigner(e.Signer)
	tx.SetSender(sdkmodels.SuiAddress(e.Signer.Address))
	return tx
}

// ObjectContent fetches an object's parsed Move fields as a map.
func (e *TransactionExecutor) ObjectContent(ctx context.Context, objectID string) (map[string]any, error) {
	resp, err := e.client.SuiGetObject(ctx, sdkmodels.SuiGetObjectRequest{
		ObjectId: objectID,
		Options:  sdkmodels.SuiObjectDataOptions{ShowContent: true},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch object %s: %w", objectID, err)
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("object %s not found", objectID)
	}
	if resp.Data.Content == nil {
		return nil, fmt.Errorf("object %s has no content", objectID)
	}
	return resp.Data.Content.Fields, nil
}

// ResolveObject fetches the object at objectID and returns a CallArg suitable for use
// as a Move call argument. For shared objects, mutable controls whether the reference
// is passed mutably. For owned objects, mutable is ignored.
func (e *TransactionExecutor) ResolveObject(ctx context.Context, objectID string, mutable bool) (sdktx.CallArg, error) {
	resp, err := e.client.SuiGetObject(ctx, sdkmodels.SuiGetObjectRequest{
		ObjectId: objectID,
		Options: sdkmodels.SuiObjectDataOptions{
			ShowOwner: true,
		},
	})
	if err != nil {
		return sdktx.CallArg{}, fmt.Errorf("fetch object %s: %w", objectID, err)
	}
	if resp.Data == nil {
		return sdktx.CallArg{}, fmt.Errorf("object %s not found", objectID)
	}

	data := resp.Data
	owner, err := parseObjectOwner(data.Owner)
	if err != nil {
		return sdktx.CallArg{}, fmt.Errorf("parse owner for %s: %w", objectID, err)
	}

	if owner.Shared.InitialSharedVersion != 0 {
		addrBytes, err := sdktx.ConvertSuiAddressStringToBytes(sdkmodels.SuiAddress(objectID))
		if err != nil {
			return sdktx.CallArg{}, fmt.Errorf("convert object ID %s: %w", objectID, err)
		}
		return sdktx.CallArg{
			Object: &sdktx.ObjectArg{
				SharedObject: &sdktx.SharedObjectRef{
					ObjectId:             *addrBytes,
					InitialSharedVersion: owner.Shared.InitialSharedVersion,
					Mutable:              mutable,
				},
			},
		}, nil
	}

	ref, err := sdktx.NewSuiObjectRef(
		sdkmodels.SuiAddress(data.ObjectId),
		data.Version,
		sdkmodels.ObjectDigest(data.Digest),
	)
	if err != nil {
		return sdktx.CallArg{}, fmt.Errorf("build object ref for %s: %w", objectID, err)
	}
	return sdktx.CallArg{
		Object: &sdktx.ObjectArg{
			ImmOrOwnedObject: ref,
		},
	}, nil
}

// AutoSelectGas selects a SUI coin owned by the signer to pay for gas and sets
// gas payment, owner, and budget on tx. budget=0 uses a default of 0.05 SUI.
func (e *TransactionExecutor) AutoSelectGas(ctx context.Context, tx *sdktx.Transaction, budget uint64) error {
	if budget == 0 {
		budget = defaultGasBudget
	}

	coins, err := e.client.SuiXGetCoins(ctx, sdkmodels.SuiXGetCoinsRequest{
		Owner:    e.Signer.Address,
		CoinType: suiCoinType,
		Limit:    1,
	})
	if err != nil {
		return fmt.Errorf("fetch gas coins: %w", err)
	}
	if len(coins.Data) == 0 {
		return fmt.Errorf("no SUI coins found for address %s", e.Signer.Address)
	}

	coin := coins.Data[0]
	slog.Debug("selected gas coin", "object_id", coin.CoinObjectId, "balance", coin.Balance)

	gasRef, err := sdktx.NewSuiObjectRef(
		sdkmodels.SuiAddress(coin.CoinObjectId),
		coin.Version,
		sdkmodels.ObjectDigest(coin.Digest),
	)
	if err != nil {
		return fmt.Errorf("build gas ref: %w", err)
	}

	tx.SetGasPayment([]sdktx.SuiObjectRef{*gasRef}).
		SetGasOwner(sdkmodels.SuiAddress(e.Signer.Address)).
		SetGasBudget(budget)

	return nil
}

// Execute submits the transaction and returns the transaction digest on success.
func (e *TransactionExecutor) Execute(ctx context.Context, tx *sdktx.Transaction) (string, error) {
	resp, err := tx.Execute(ctx, sdkmodels.SuiTransactionBlockOptions{
		ShowEffects: true,
	}, "WaitForLocalExecution")
	if err != nil {
		return "", fmt.Errorf("execute transaction: %w", err)
	}

	if resp.Effects.Status.Status != "success" {
		return resp.Digest, fmt.Errorf("transaction failed: %s", resp.Effects.Status.Error)
	}

	slog.Debug("transaction executed", "digest", resp.Digest)
	return resp.Digest, nil
}

// DeleteBlob deletes a deletable Walrus blob and transfers the freed Storage object
// back to the signer. Returns the Storage object ID and the transaction digest.
// The Storage object can be reused by a publisher configured with reuse_resources=true.
func (e *TransactionExecutor) DeleteBlob(ctx context.Context, packageID, systemObjectID, blobObjectID string, gasBudget uint64) (storageObjectID, digest string, err error) {
	systemArg, err := e.ResolveObject(ctx, systemObjectID, false)
	if err != nil {
		return "", "", fmt.Errorf("resolve system object: %w", err)
	}

	blobArg, err := e.ResolveObject(ctx, blobObjectID, false)
	if err != nil {
		return "", "", fmt.Errorf("resolve blob object: %w", err)
	}

	tx := e.NewTransaction()

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

	tx.TransferObjects([]sdktx.Argument{storage}, tx.Pure(e.Signer.Address))

	if err := e.AutoSelectGas(ctx, tx, gasBudget); err != nil {
		return "", "", fmt.Errorf("select gas: %w", err)
	}

	resp, err := tx.Execute(ctx, sdkmodels.SuiTransactionBlockOptions{
		ShowEffects:       true,
		ShowObjectChanges: true,
	}, "WaitForLocalExecution")
	if err != nil {
		return "", "", fmt.Errorf("execute transaction: %w", err)
	}

	if resp.Effects.Status.Status != "success" {
		return "", resp.Digest, fmt.Errorf("transaction failed: %s", resp.Effects.Status.Error)
	}

	for _, change := range resp.ObjectChanges {
		if strings.HasSuffix(change.ObjectType, storageResourceTypeSuffix) {
			storageObjectID = change.ObjectId
			break
		}
	}

	slog.Debug("blob deleted", "digest", resp.Digest, "storage_object_id", storageObjectID)
	return storageObjectID, resp.Digest, nil
}

// parseObjectOwner extracts an ObjectOwner from the raw owner field of a SuiObjectData.
// The field is interface{} in the SDK model (can be a string like "Immutable"
// or a nested object like {"AddressOwner":"0x..."} or {"Shared":{...}}).
func parseObjectOwner(raw any) (sdkmodels.ObjectOwner, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return sdkmodels.ObjectOwner{}, err
	}
	var owner sdkmodels.ObjectOwner
	if err := json.Unmarshal(b, &owner); err != nil {
		return sdkmodels.ObjectOwner{}, err
	}
	return owner, nil
}
