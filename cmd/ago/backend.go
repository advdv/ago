package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func backendCmd() *cli.Command {
	return &cli.Command{
		Name:  "backend",
		Usage: "Backend development commands",
		Commands: []*cli.Command{
			{
				Name:   "build",
				Usage:  "Build backend container images using ko",
				Action: config.RunWithConfig(runBackendBuild),
			},
		},
	}
}

func runBackendBuild(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doBackendBuild(ctx, cfg, backendBuildOptions{
		Output: os.Stdout,
		ErrOut: os.Stderr,
	})
}

type backendBuildOptions struct {
	Output io.Writer
	ErrOut io.Writer
}

func doBackendBuild(ctx context.Context, cfg config.Config, opts backendBuildOptions) error {
	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.ErrOut).InSubdir("backend")

	cmdDir := filepath.Join(exec.Dir(), "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return errors.Wrap(err, "failed to read backend/cmd directory")
	}

	var importPaths []string
	for _, entry := range entries {
		if entry.IsDir() {
			importPaths = append(importPaths, "./cmd/"+entry.Name())
		}
	}

	if len(importPaths) == 0 {
		return errors.New("no commands found in backend/cmd")
	}

	args := append([]string{"build"}, importPaths...)
	return exec.Mise(ctx, "ko", args...)
}
