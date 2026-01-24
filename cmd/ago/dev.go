package main

import (
	"context"

	"github.com/bitfield/script"
	"github.com/urfave/cli/v3"
)

func devCmd() *cli.Command {
	return &cli.Command{
		Name:  "dev",
		Usage: "Development commands",
		Commands: []*cli.Command{
			{
				Name:   "fmt",
				Usage:  "Format Go code using golangci-lint",
				Action: devFmt,
			},
			{
				Name:   "gen",
				Usage:  "Run go generate",
				Action: devGen,
			},
		},
	}
}

func devFmt(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("golangci-lint fmt ./...").Stdout()
	return err
}

func devGen(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("go generate ./...").Stdout()
	return err
}
