set dotenv-load

sui_rpc_url        := "https://fullnode.testnet.sui.io:443"
walrus_package_id  := "0xd84704c17fc870b8764832c535aa6b11f21a95cd6f5bb38a9b07d2cf42220c66"
walrus_tx_package_id := "0x849e95d2718938d66c37fb91df76d72f78526c1864c339bac415ce8ecda2d8cc"
walrus_aggregator  := "https://aggregator.walrus-testnet.walrus.space"
walrus_publisher   := "https://publisher.walrus-testnet.walrus.space"
walrus_bin         := env("WALRUS_BIN", "walrus")
sui_bin            := env("SUI_BIN", "sui")
wallet_dir         := env("WALRUS_WALLET_DIR", "")
compose            := "docker compose -f docker/compose.yaml --env-file .env"

clean:
    rm -rf dist

# start deps only: ClickHouse and a local Walrus aggregator
env-up:
    {{compose}} up -d

# start all components including whisker (builds image first)
env-up-full:
    {{compose}} --profile full up -d --build

# stop all environment services and remove containers
env-down:
    {{compose}} --profile full down

# follow logs; optionally pass a service name to filter
env-logs *args='':
    {{compose}} --profile full logs -f {{args}}


run: build
    ./dist/whisker \
        --publisher {{walrus_publisher}} \
        --aggregator {{walrus_aggregator}} \
        --rpc-url {{sui_rpc_url}} \
        --package {{walrus_package_id}} \
        --tx-package {{walrus_tx_package_id}} \
        --interval 5m \
        --probe-size 1048576 \
        --event-timeout 10m \
        --poll-interval 5s \
        --log-level debug

# run whisker locally against the local Walrus daemon (env-up must be running)
run-local: build
    ./dist/whisker \
        --publisher http://localhost:31415 \
        --aggregator http://localhost:31415 \
        --rpc-url {{sui_rpc_url}} \
        --package {{walrus_package_id}} \
        --tx-package {{walrus_tx_package_id}} \
        --interval 5m \
        --probe-size 1048576 \
        --event-timeout 10m \
        --poll-interval 5s \
        --log-level debug



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
    ./dist/wkit watch --rpc-url {{sui_rpc_url}} --package {{walrus_tx_package_id}} --human

fetch blob_id: build-wkit
    ./dist/wkit fetch --aggregator {{walrus_aggregator}} --out {{blob_id}} {{blob_id}}

publish file: build-wkit
    ./dist/wkit publish --deletable --publisher {{walrus_publisher}} {{file}}

delete object_id: build-wkit
    ./dist/wkit delete --rpc-url {{sui_rpc_url}} {{object_id}}



# WALRUS UTILITIES

# swap SUI for WAL using the configured wallet
get-wal:
    {{walrus_bin}} get-wal \
        --config docker/walrus/client_config.yaml \
        --wallet {{wallet_dir}}/client.yaml

# list non-expired blobs owned by the wallet
list-blobs:
    {{walrus_bin}} list-blobs \
        --config docker/walrus/client_config.yaml \
        --wallet {{wallet_dir}}/client.yaml

# list all blobs including expired ones
list-blobs-all:
    {{walrus_bin}} list-blobs \
        --include-expired \
        --config docker/walrus/client_config.yaml \
        --wallet {{wallet_dir}}/client.yaml

# delete a blob by blob ID and reclaim the storage resource
delete-blob blob_id:
    {{walrus_bin}} delete \
        --blob-id {{blob_id}} \
        --config docker/walrus/client_config.yaml \
        --wallet {{wallet_dir}}/client.yaml

# burn all expired Sui blob objects to reclaim gas storage deposits
burn-expired:
    {{walrus_bin}} burn-blobs \
        --all-expired \
        --config docker/walrus/client_config.yaml \
        --wallet {{wallet_dir}}/client.yaml

