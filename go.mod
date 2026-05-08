module github.com/probe-lab/whisker

go 1.26.2

require github.com/urfave/cli/v3 v3.8.0

require (
	github.com/block-vision/sui-go-sdk v1.2.1
	github.com/btcsuite/btcutil v1.0.2
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
)

require (
	github.com/ClickHouse/ch-go v0.71.0 // indirect
	github.com/ClickHouse/clickhouse-go/v2 v2.45.0 // indirect
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cosmos/go-bip39 v1.0.0 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.12.0 // indirect
	github.com/golang-migrate/migrate/v4 v4.19.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/leodido/go-urn v1.2.2 // indirect
	github.com/mr-tron/base58 v1.3.0 // indirect
	github.com/paulmach/orb v0.13.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/probe-lab/go-commons v0.0.0-20260429113009-d7436297c6ed // indirect
	github.com/samber/lo v1.49.1 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/uptrace/opentelemetry-go-extra/otelsql v0.3.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/time v0.15.0 // indirect
)

// Adds missing Unwrapped field to SuiEffects
replace github.com/block-vision/sui-go-sdk => github.com/iand/sui-go-sdk v0.0.0-20260427111348-ec70e05c7e81
