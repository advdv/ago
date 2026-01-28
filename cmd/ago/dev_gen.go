package main

import (
	"context"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func devGen(ctx context.Context, _ *cli.Command, cfg config.Config) error {
	exec := cmdexec.New(cfg).WithOutput(os.Stdout, os.Stderr)

	return runInGoModules(ctx, exec, "go", "generate", "./...")
}
