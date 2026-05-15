set dotenv-load

network            := env("WALRUS_NETWORK", "testnet")
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

# stop all environment services, remove containers and volumes
env-down:
    {{compose}} --profile full down -v

# follow container logs; optionally pass a service name to filter
env-logs *args='':
    {{compose}} --profile full logs -f {{args}}


# run whisker locally against the local Walrus daemon (env-up must be running)
run: build
    ./dist/whisker --log.level debug run \
        --publisher http://localhost:31415 \
        --aggregator http://localhost:31415 \
        --interval 15m \
        --probe-size 102400,1048576,10485760 \
        --event-timeout 10m \
        --poll-interval 5s \
        --network {{network}} \
        --probe-location local \
        --clickhouse-url clickhouse://whisker:whisker@localhost:9000/whisker

# dump the contents of storage_checks from the local ClickHouse
view-clickhouse:
    {{compose}} exec clickhouse clickhouse-client \
        --user whisker --password whisker --database whisker \
        --query "SELECT * FROM storage_checks FORMAT PrettyCompact"

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
    ./dist/wkit watch --network {{network}} --human

fetch blob_id: build-wkit
    ./dist/wkit fetch --network {{network}} --out {{blob_id}} {{blob_id}}

publish file: build-wkit
    ./dist/wkit publish --network {{network}} --deletable {{file}}

delete object_id: build-wkit
    ./dist/wkit delete --network {{network}} {{object_id}}



# WALRUS UTILITIES

# swap SUI for WAL using the configured wallet (testnet only)
get-wal:
    #!/usr/bin/env sh
    if [ "{{network}}" = "mainnet" ]; then
        echo "get-wal is not available on mainnet - acquire WAL via a DEX (e.g. Cetus) or exchange"
        exit 1
    fi
    {{walrus_bin}} get-wal \
        --config docker/walrus/client_config.yaml \
        --context {{network}} \
        --wallet {{wallet_dir}}/client.yaml

# list non-expired blobs owned by the wallet
list-blobs:
    {{walrus_bin}} list-blobs \
        --config docker/walrus/client_config.yaml \
        --context {{network}} \
        --wallet {{wallet_dir}}/client.yaml

# list all blobs including expired ones
list-blobs-all:
    {{walrus_bin}} list-blobs \
        --include-expired \
        --config docker/walrus/client_config.yaml \
        --context {{network}} \
        --wallet {{wallet_dir}}/client.yaml

# delete a blob by blob ID and reclaim the storage resource
delete-blob blob_id:
    {{walrus_bin}} delete \
        --blob-id {{blob_id}} \
        --config docker/walrus/client_config.yaml \
        --context {{network}} \
        --wallet {{wallet_dir}}/client.yaml

# burn all expired Sui blob objects to reclaim gas storage deposits
burn-expired:
    {{walrus_bin}} burn-blobs \
        --all-expired \
        --config docker/walrus/client_config.yaml \
        --context {{network}} \
        --wallet {{wallet_dir}}/client.yaml

