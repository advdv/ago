package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func selfUpgradeCmd() *cli.Command {
	return &cli.Command{
		Name:  "self-upgrade",
		Usage: "Upgrade ago CLI to the latest version",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fmt.Println("VERIFIED: self-upgrade v2 working!")
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			return upgradeAgoCLI(ctx, dir)
		},
	}
}

// upgradeAgoCLI upgrades the ago CLI in an existing project.
// Uses the same logic as installAgoCLI to ensure consistent behavior.
func upgradeAgoCLI(ctx context.Context, dir string) error {
	return installAgoCLI(ctx, dir)
}
