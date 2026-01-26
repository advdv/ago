package main

import (
	"context"
	"os"
	"strings"

	"github.com/bitfield/script"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

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
