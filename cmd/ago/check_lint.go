package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/bitfield/script"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func checkLint(ctx context.Context, cmd *cli.Command) error {
	if _, err := script.Exec("golangci-lint run ./...").Stdout(); err != nil {
		return err
	}

	if err := runShellcheck(ctx); err != nil {
		return err
	}

	return nil
}

func runShellcheck(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get working directory")
	}

	shellFiles, err := FindShellScripts(cwd)
	if err != nil {
		return errors.Wrap(err, "failed to find shell scripts")
	}

	if len(shellFiles) == 0 {
		return nil
	}

	args := append([]string{}, shellFiles...)
	shellCmd := exec.CommandContext(ctx, "shellcheck", args...)
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	if err := shellCmd.Run(); err != nil {
		return errors.Wrap(err, "shellcheck failed")
	}

	return nil
}
