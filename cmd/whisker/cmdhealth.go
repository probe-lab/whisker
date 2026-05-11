package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/urfave/cli/v3"
)

var healthCmd = &cli.Command{
	Name:  "health",
	Usage: "Check the health of a running whisker process",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "addr",
			Usage: "host:port of the whisker metrics server (matches --metrics.host and --metrics.port)",
			Value: "localhost:6060",
		},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		url := "http://" + cmd.String("addr") + "/healthz"
		resp, err := http.Get(url) //nolint:noctx
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health check returned status %d", resp.StatusCode)
		}
		return nil
	},
}
