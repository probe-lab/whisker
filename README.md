# whisker

Walrus availability monitoring toolkit.

## Overview

whisker monitors the availability of blobs on the [Walrus](https://docs.wal.app/) decentralized storage network. It watches on-chain events from the Sui blockchain and measures retrieval performance against Walrus aggregator nodes.

## Binaries

- **whisker** - availability monitor service
- **wkit** - command-line toolkit for interacting with Walrus

## Building

```sh
just build      # build whisker
just build-wkit # build wkit
just build-all  # build both
```

## wkit commands

```sh
wkit watch   # stream Walrus events from the Sui blockchain
wkit fetch   # download a blob from a Walrus aggregator
wkit publish # upload a file to a Walrus publisher
wkit delete  # delete a blob by Sui object ID
```

## Running whisker in the test environment

This runs whisker against the testnet. See the prerequisites section below.

In one terminal set up the environment:

```sh
just env-up        # start ClickHouse and a local Walrus daemon
just env-logs      # tail logs
```

In another terminal

```sh
just run
```

Use `ctrl+c` to stop whisker then

```sh
just env-down      # stop and remove containers
```

### Prerequisites

Testing requires a dedicated Sui testnet wallet funded with SUI and WAL.

Create one using the Walrus CLI, specifying a path so it does not conflict with an existing Sui CLI wallet:

```sh
walrus generate-sui-wallet --sui-network testnet --path ~/.walrus-testnet-wallet
```

Note the wallet address printed by the command. Fund it with SUI at `https://faucet.sui.io/?address=<address>`, then swap for WAL:

```sh
walrus get-wal --wallet ~/.walrus-testnet-wallet/client.yaml
```

Check balance at `https://suiscan.xyz/testnet/account/<address>`

Extract the private key in the format required by `WHISKER_SUI_SIGNER`:

```sh
go run misc/extractkey.go
# or with a custom keystore path:
go run misc/extractkey.go /path/to/sui.keystore
```

**Configure** - create `whisker/.env` (gitignored):

```
# absolute path to the directory containing sui.keystore created above
WALRUS_WALLET_DIR=/home/<user>/.walrus-testnet-wallet

# private key (suiprivkey-prefixed bech32) printed by the script above
# enables blob deletion and storage recycling after each probe; optional
WHISKER_SUI_SIGNER=suiprivkey1...
```

`WALRUS_WALLET_DIR` and `WHISKER_SUI_SIGNER` must reference the same wallet so that storage resources recovered after deletion can be reused by the daemon on the next probe.


## Requirements

- Go 1.26+
- [just](https://github.com/casey/just) (command runner)
- Access to a Sui RPC endpoint
- Access to Walrus aggregator/publisher endpoints

## License

This project is licensed under the Apache License, Version 2.0 ([LICENSE](LICENSE) or
<https://www.apache.org/licenses/LICENSE-2.0>). Copyright 2026 ProbeLab Analytics OÜ
