package main

import (
	"context"
	"os"
	"strings"

	"github.com/bitfield/script"
	"github.com/cockroachdb/errors"
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

func checkTests(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("go test ./...").Stdout()
	return err
}

func checkLint(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("golangci-lint run ./...").Stdout()
	return err
}

func checkCompiles(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("go build ./...").Stdout()
	return err
}

func checkUncommittedChanges(ctx context.Context, cmd *cli.Command) error {
	if os.Getenv("CI") != "true" {
		return nil
	}

	status, err := script.Exec("git status --porcelain").String()
	if err != nil {
		return errors.Wrap(err, "failed to check git status")
	}

	if strings.TrimSpace(status) != "" {
		os.Stderr.WriteString("ERROR: Code is not up to date.\n")
		os.Stderr.WriteString("Run generating tasks locally and commit the changes.\n\n")
		_, _ = script.Exec("git status --short").Stdout()
		os.Exit(1)
	}

	return nil
}
