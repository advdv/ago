package main

import (
	"context"
	"io"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func tfInitCmd() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Initialize Terraform working directory",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "upgrade",
				Usage: "Upgrade provider plugins to newest version",
			},
		},
		Action: config.RunWithConfig(runTFInit),
	}
}

func runTFInit(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doTFInit(ctx, cfg, tfInitOptions{
		Upgrade: cmd.Bool("upgrade"),
		Output:  os.Stdout,
	})
}

type tfInitOptions struct {
	Upgrade bool
	Output  io.Writer
}

func doTFInit(ctx context.Context, cfg config.Config, opts tfInitOptions) error {
	exec := cmdexec.New(cfg).InSubdir("infra/tf").WithOutput(opts.Output, opts.Output)

	args := []string{"init"}
	if opts.Upgrade {
		args = append(args, "-upgrade")
	}

	return exec.Mise(ctx, "terraform", args...)
}
