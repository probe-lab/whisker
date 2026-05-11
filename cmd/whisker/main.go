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
	Commands: []*cli.Command{
		runCmd,
		healthCmd,
	},
})
