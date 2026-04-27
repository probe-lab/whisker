package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/btcsuite/btcutil/bech32"
)

func main() {
	path := filepath.Join(os.Getenv("HOME"), ".walrus-testnet-wallet", "sui.keystore")
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var keystore []string
	if err := json.Unmarshal(raw, &keystore); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(keystore) == 0 {
		fmt.Fprintln(os.Stderr, "error: keystore is empty")
		os.Exit(1)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keystore[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	converted, err := bech32.ConvertBits(keyBytes, 8, 5, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	encoded, err := bech32.Encode("suiprivkey", converted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(encoded)
}
