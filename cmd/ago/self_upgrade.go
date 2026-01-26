package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/cockroachdb/errors"
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
			return upgradeAgoCLI(ctx, dir)
		},
	}
}

// upgradeAgoCLI upgrades the ago CLI in an existing project.
//
// Unlike installAgoCLI (which uses "mise use" to add the package to mise.toml),
// this function uses "mise install" to reinstall the package that's already
// defined in mise.toml. This ensures we operate on the same package identifier.
func upgradeAgoCLI(ctx context.Context, dir string) error {
	const agoPackage = "go:github.com/advdv/ago/cmd/ago@main"

	env := append(os.Environ(), "GOPROXY=direct", "GOFLAGS=-mod=mod")

	// Uninstall existing version so mise will fetch fresh from main branch
	uninstallCmd := exec.CommandContext(ctx, "mise", "uninstall", agoPackage)
	uninstallCmd.Dir = dir
	uninstallCmd.Env = env
	_ = uninstallCmd.Run() // Ignore error - package might not exist

	// Reinstall based on mise.toml entry
	installCmd := exec.CommandContext(ctx, "mise", "install", agoPackage)
	installCmd.Dir = dir
	installCmd.Env = env
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to upgrade ago CLI")
	}

	return nil
}
