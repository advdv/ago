package main

import (
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
				Action: checkTests,
			},
			{
				Name:   "lint",
				Usage:  "Lint Go code using golangci-lint",
				Action: checkLint,
			},
			{
				Name:   "compiles",
				Usage:  "Check that all packages compile",
				Action: checkCompiles,
			},
			{
				Name:   "uncommitted-changes",
				Usage:  "Check generated code is checked-in",
				Action: checkUncommittedChanges,
			},
		},
	}
}
