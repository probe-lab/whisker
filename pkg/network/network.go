package network

import "fmt"

// Config holds the network-specific endpoint defaults for a Walrus network.
type Config struct {
	Publisher    string
	Aggregator   string
	RPCURL       string
	SystemObject string
}

var mainnetCfg = Config{
	Publisher:    "https://publisher.walrus.space",
	Aggregator:   "https://aggregator.walrus.space",
	RPCURL:       "https://fullnode.mainnet.sui.io:443",
	SystemObject: "0x2134d52768ea07e8c43570ef975eb3e4c27a39fa6396bef985b5abc58d03ddd2",
}

var testnetCfg = Config{
	Publisher:    "https://publisher.walrus-testnet.walrus.space",
	Aggregator:   "https://aggregator.walrus-testnet.walrus.space",
	RPCURL:       "https://fullnode.testnet.sui.io:443",
	SystemObject: "0x6c2547cbbc38025cf3adac45f63cb0a8d12ecf777cdc75a4971612bf97fdf6af",
}

// Defaults returns the endpoint defaults for the named network.
func Defaults(name string) (Config, error) {
	switch name {
	case "mainnet":
		return mainnetCfg, nil
	case "testnet":
		return testnetCfg, nil
	default:
		return Config{}, fmt.Errorf("unknown network %q: must be mainnet or testnet", name)
	}
}
