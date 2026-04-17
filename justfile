sui_rpc_url        := "https://fullnode.testnet.sui.io:443"
walrus_package_id  := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"
walrus_aggregator  := "https://aggregator.walrus-testnet.walrus.space"
walrus_publisher   := "https://publisher.walrus-testnet.walrus.space"

clean:
    rm -rf dist

build-all: build-whisker build-whisker-watch build-whisker-fetch build-whisker-publish

build-whisker:
    mkdir -p dist
    go build -o dist/whisker ./cmd/whisker

build-whisker-watch:
    mkdir -p dist
    go build -o dist/whisker-watch ./cmd/whisker-watch

build-whisker-fetch:
    mkdir -p dist
    go build -o dist/whisker-fetch ./cmd/whisker-fetch

build-whisker-publish:
    mkdir -p dist
    go build -o dist/whisker-publish ./cmd/whisker-publish

watch: build-whisker-watch
    ./dist/whisker-watch --rpc-url {{sui_rpc_url}} --package {{walrus_package_id}} --human

fetch blob_id: build-whisker-fetch
    ./dist/whisker-fetch --aggregator {{walrus_aggregator}} --out {{blob_id}} {{blob_id}}

publish file: build-whisker-publish
    ./dist/whisker-publish --publisher {{walrus_publisher}} {{file}}
