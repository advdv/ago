package main

import (
	"context"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func checkLint(ctx context.Context, _ *cli.Command, cfg config.Config) error {
	exec := cmdexec.New(cfg).WithOutput(os.Stdout, os.Stderr)

	if err := exec.InSubdir("infra").Run(ctx, "golangci-lint", "run", "./..."); err != nil {
		return err
	}

	return runShellcheck(ctx, exec)
}

func runShellcheck(ctx context.Context, exec cmdexec.Executor) error {
	shellFiles, err := FindShellScripts(exec.Dir())
	if err != nil {
		return err
	}

	if len(shellFiles) == 0 {
		return nil
	}

	args := append([]string{}, shellFiles...)

	return exec.Run(ctx, "shellcheck", args...)
}
