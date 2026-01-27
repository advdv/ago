package main

import (
	"context"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func devGen(ctx context.Context, _ *cli.Command, cfg config.Config) error {
	return cmdexec.New(cfg).WithOutput(os.Stdout, os.Stderr).Run(ctx, "go", "generate", "./...")
}
