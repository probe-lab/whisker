sui_rpc_url        := "https://fullnode.testnet.sui.io:443"
walrus_package_id  := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"
walrus_aggregator  := "https://aggregator.walrus-testnet.walrus.space"
walrus_publisher   := "https://publisher.walrus-testnet.walrus.space"

clean:
    rm -rf dist

build-all: build-whisker build-wkit

build-whisker:
    mkdir -p dist
    go build -o dist/whisker ./cmd/whisker

build-wkit:
    mkdir -p dist
    go build -o dist/wkit ./cmd/wkit

watch: build-wkit
    ./dist/wkit watch --rpc-url {{sui_rpc_url}} --package {{walrus_package_id}} --human

fetch blob_id: build-wkit
    ./dist/wkit fetch --aggregator {{walrus_aggregator}} --out {{blob_id}} {{blob_id}}

publish file: build-wkit
    ./dist/wkit publish --publisher {{walrus_publisher}} {{file}}
