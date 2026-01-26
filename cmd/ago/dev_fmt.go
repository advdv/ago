package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/bitfield/script"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func devFmt(ctx context.Context, cmd *cli.Command) error {
	if _, err := script.Exec("golangci-lint fmt ./...").Stdout(); err != nil {
		return err
	}

	if err := runShfmt(ctx); err != nil {
		return err
	}

	return nil
}

func runShfmt(ctx context.Context) error {
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

	args := append([]string{"-w"}, shellFiles...)
	shfmtCmd := exec.CommandContext(ctx, "shfmt", args...)
	shfmtCmd.Stdout = os.Stdout
	shfmtCmd.Stderr = os.Stderr

	if err := shfmtCmd.Run(); err != nil {
		return errors.Wrap(err, "shfmt failed")
	}

	return nil
}
