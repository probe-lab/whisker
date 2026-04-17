sui_rpc_url := "https://fullnode.testnet.sui.io:443"
walrus_package_id := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"

build-all:
    mkdir -p dist
    for dir in cmd/*/; do \
        go build -o dist/$(basename "$dir") ./"$dir"; \
    done

sui-watch:
    SUI_RPC_URL={{sui_rpc_url}} WALRUS_PACKAGE_ID={{walrus_package_id}} ./dist/sui-watch | jq .
