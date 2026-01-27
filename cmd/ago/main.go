package main

import (
	"context"
	"fmt"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	finder := config.NewFinder(config.NewLoader())

	cmd := &cli.Command{
		Name:    "ago",
		Usage:   "Development task runner for the ago project",
		Version: Version,
		Commands: []*cli.Command{
			cdkCmd(),
			checkCmd(),
			devCmd(),
			initCmd(),
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			if cmd.Args().First() == "init" {
				return ctx, nil
			}

			cwd, err := os.Getwd()
			if err != nil {
				return ctx, err
			}

			cfg, projectDir, err := finder.Find(cwd)
			if err != nil {
				return ctx, err
			}

			return config.WithContext(ctx, config.Context{
				Config:     cfg,
				ProjectDir: projectDir,
			}), nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
