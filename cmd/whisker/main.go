package main

import (
	"log/slog"
	"os"

	plcli "github.com/probe-lab/go-commons/cli"
	"github.com/urfave/cli/v3"
)

func main() {
	if err := rootCmd.Run(); err != nil {
		slog.Error("terminated abnormally", "err", err)
		os.Exit(1)
	}
}

var rootCmd, _ = plcli.NewRootCommand(&cli.Command{
	Name:  "whisker",
	Usage: "A Walrus network availability monitor",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "private-key",
			Usage:   "Sui private key: suiprivkey bech32 or BIP-39 mnemonic",
			Sources: cli.EnvVars("WHISKER_SUI_SIGNER"),
		},
	},
	Commands: []*cli.Command{
		runCmd,
		healthCmd,
		watchCmd,
		fetchCmd,
		publishCmd,
		deleteCmd,
	},
})
