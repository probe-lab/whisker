sui_rpc_url := "https://fullnode.testnet.sui.io:443"
walrus_package_id := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"

build-all:
    mkdir -p dist
    for dir in cmd/*/; do \
        go build -o dist/$(basename "$dir") ./"$dir"; \
    done

sui-watch:
    ./dist/sui-watch --rpc-url {{sui_rpc_url}} --package {{walrus_package_id}} | jq .
