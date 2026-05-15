package main

import "github.com/urfave/cli/v3"

// resolveFlag returns the value of the named flag if non-empty, otherwise fallback.
func resolveFlag(cmd *cli.Command, name, fallback string) string {
	if v := cmd.String(name); v != "" {
		return v
	}
	return fallback
}

// networkFlag is the shared --network flag definition used by every subcommand.
var networkFlag = &cli.StringFlag{
	Name:    "network",
	Usage:   "network to use: mainnet or testnet",
	Value:   "testnet",
	Sources: cli.EnvVars("WHISKER_NETWORK"),
}
