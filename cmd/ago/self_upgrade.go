package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"
)

func selfUpgradeCmd() *cli.Command {
	return &cli.Command{
		Name:  "self-upgrade",
		Usage: "Upgrade ago CLI to the latest version",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			return installAgoCLI(ctx, dir)
		},
	}
}
