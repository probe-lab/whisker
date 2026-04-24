sui_rpc_url        := "https://fullnode.testnet.sui.io:443"
walrus_package_id  := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"
walrus_aggregator  := "https://aggregator.walrus-testnet.walrus.space"
walrus_publisher   := "https://publisher.walrus-testnet.walrus.space"

clean:
    rm -rf dist

# start deps only: ClickHouse and a local Walrus aggregator
env-up:
    docker compose -f docker/compose.yaml up -d

# start all components including whisker (builds image first)
env-up-full:
    docker compose -f docker/compose.yaml --profile full up -d --build

# stop all environment services and remove containers
env-down:
    docker compose -f docker/compose.yaml --profile full down

# follow logs; optionally pass a service name to filter
env-logs *args='':
    docker compose -f docker/compose.yaml --profile full logs -f {{args}}

build-image:
    docker build -t whisker -f docker/whisker/Dockerfile .

test:
    go test ./...

build:
    mkdir -p dist
    go build -o dist/whisker ./cmd/whisker

build-all: build build-wkit

build-wkit:
    mkdir -p dist
    go build -o dist/wkit ./cmd/wkit

watch: build-wkit
    ./dist/wkit watch --rpc-url {{sui_rpc_url}} --package {{walrus_package_id}} --human

fetch blob_id: build-wkit
    ./dist/wkit fetch --aggregator {{walrus_aggregator}} --out {{blob_id}} {{blob_id}}

publish file: build-wkit
    ./dist/wkit publish --deletable --publisher {{walrus_publisher}} {{file}}

delete object_id: build-wkit
    ./dist/wkit delete --rpc-url {{sui_rpc_url}} {{object_id}}

probe: build
    ./dist/whisker \
        --publisher {{walrus_publisher}} \
        --aggregator {{walrus_aggregator}} \
        --rpc-url {{sui_rpc_url}} \
        --package {{walrus_package_id}} \
        --interval 5m \
        --probe-size 1048576 \
        --event-timeout 10m \
        --poll-interval 5s \
        --log-level debug \
        --dry-run
