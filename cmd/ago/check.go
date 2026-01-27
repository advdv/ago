package main

import (
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func checkCmd() *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "Run various checks",
		Commands: []*cli.Command{
			{
				Name:   "tests",
				Usage:  "Run Go tests",
				Action: config.WithConfig(checkTests),
			},
			{
				Name:   "lint",
				Usage:  "Lint Go code using golangci-lint",
				Action: config.WithConfig(checkLint),
			},
			{
				Name:   "compiles",
				Usage:  "Check that all packages compile",
				Action: config.WithConfig(checkCompiles),
			},
			{
				Name:   "uncommitted-changes",
				Usage:  "Check generated code is checked-in",
				Action: config.WithConfig(checkUncommittedChanges),
			},
		},
	}
}
