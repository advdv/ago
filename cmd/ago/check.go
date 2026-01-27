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
				Name:   "test",
				Usage:  "Run Go tests",
				Action: config.RunWithConfig(checkTests),
			},
			{
				Name:   "lint",
				Usage:  "Lint Go code using golangci-lint",
				Action: config.RunWithConfig(checkLint),
			},
			{
				Name:   "compiles",
				Usage:  "Check that all packages compile",
				Action: config.RunWithConfig(checkCompiles),
			},
			{
				Name:   "uncommitted-changes",
				Usage:  "Check generated code is checked-in",
				Action: config.RunWithConfig(checkUncommittedChanges),
			},
		},
	}
}
