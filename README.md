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

## Requirements

- Go 1.26+
- [just](https://github.com/casey/just) (command runner)
- Access to a Sui RPC endpoint
- Access to Walrus aggregator/publisher endpoints

## License

This project is licensed under the Apache License, Version 2.0 ([LICENSE](LICENSE) or
<https://www.apache.org/licenses/LICENSE-2.0>). Copyright 2026 ProbeLab Analytics OÜ
