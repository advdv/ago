package main

import (
	"context"
	"io"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func tfApplyCmd() *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "Apply Terraform changes",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "auto-approve",
				Usage: "Skip interactive approval (for CI)",
			},
			&cli.BoolFlag{
				Name:  "destroy",
				Usage: "Destroy all resources instead of creating/updating",
			},
		},
		Action: config.RunWithConfig(runTFApply),
	}
}

func runTFApply(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doTFApply(ctx, cfg, tfApplyOptions{
		AutoApprove: cmd.Bool("auto-approve"),
		Destroy:     cmd.Bool("destroy"),
		Output:      os.Stdout,
	})
}

type tfApplyOptions struct {
	AutoApprove bool
	Destroy     bool
	Output      io.Writer
}

func doTFApply(ctx context.Context, cfg config.Config, opts tfApplyOptions) error {
	exec := cmdexec.New(cfg).InSubdir("infra/tf").WithOutput(opts.Output, opts.Output)

	args := []string{"apply"}
	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}
	if opts.Destroy {
		args = append(args, "-destroy")
	}

	return exec.Mise(ctx, "terraform", args...)
}
