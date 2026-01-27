package main

import (
	"context"
	"os"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func checkUncommittedChanges(ctx context.Context, _ *cli.Command, cfg config.Config) error {
	if os.Getenv("CI") != "true" {
		return nil
	}

	exec := cmdexec.New(cfg)

	status, err := exec.Output(ctx, "git", "status", "--porcelain")
	if err != nil {
		return err
	}

	if strings.TrimSpace(status) != "" {
		os.Stderr.WriteString("ERROR: Code is not up to date.\n")
		os.Stderr.WriteString("Run generating tasks locally and commit the changes.\n\n")
		_ = exec.WithOutput(os.Stdout, os.Stderr).Run(ctx, "git", "status", "--short")
		os.Exit(1)
	}

	return nil
}
