package main

import (
	"context"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func devFmt(ctx context.Context, _ *cli.Command, cfg config.Config) error {
	exec := cmdexec.New(cfg).WithOutput(os.Stdout, os.Stderr)

	if err := runInGoModules(ctx, exec, "golangci-lint", "fmt", "./..."); err != nil {
		return err
	}

	return runShfmt(ctx, exec)
}

func runShfmt(ctx context.Context, exec cmdexec.Executor) error {
	shellFiles, err := FindShellScripts(exec.Dir())
	if err != nil {
		return err
	}

	if len(shellFiles) == 0 {
		return nil
	}

	args := append([]string{"-w"}, shellFiles...)

	return exec.Run(ctx, "shfmt", args...)
}
