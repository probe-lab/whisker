package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name:  "wkit",
		Usage: "Walrus toolkit",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level (debug, info, warn, error)",
				Value:   "info",
				Sources: cli.EnvVars("WKIT_LOG_LEVEL"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			level := slog.LevelInfo
			if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
				return ctx, err
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			})))
			return ctx, nil
		},
		Commands: []*cli.Command{
			fetchCommand(),
			publishCommand(),
			watchCommand(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
