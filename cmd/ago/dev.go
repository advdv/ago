package main

import (
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
