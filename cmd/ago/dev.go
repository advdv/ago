package main

import (
	"github.com/advdv/ago/cmd/ago/internal/config"
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
				Action: config.WithConfig(devFmt),
			},
			{
				Name:   "gen",
				Usage:  "Run go generate",
				Action: config.WithConfig(devGen),
			},
		},
	}
}
