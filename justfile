sui_rpc_url := "https://fullnode.testnet.sui.io:443"
walrus_package_id := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"

build-all: build-whisker build-sui-watch

build-whisker:
    mkdir -p dist
    go build -o dist/whisker ./cmd/whisker

build-sui-watch:
    mkdir -p dist
    go build -o dist/sui-watch ./cmd/sui-watch

sui-watch: build-sui-watch
    ./dist/sui-watch --rpc-url {{sui_rpc_url}} --package {{walrus_package_id}} | jq .
