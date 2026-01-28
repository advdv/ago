package main

import (
	"context"
	"io"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func tfPlanCmd() *cli.Command {
	return &cli.Command{
		Name:  "plan",
		Usage: "Show Terraform execution plan",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "destroy",
				Usage: "Create a plan to destroy all resources",
			},
		},
		Action: config.RunWithConfig(runTFPlan),
	}
}

func runTFPlan(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doTFPlan(ctx, cfg, tfPlanOptions{
		Destroy: cmd.Bool("destroy"),
		Output:  os.Stdout,
	})
}

type tfPlanOptions struct {
	Destroy bool
	Output  io.Writer
}

func doTFPlan(ctx context.Context, cfg config.Config, opts tfPlanOptions) error {
	exec := cmdexec.New(cfg).InSubdir("infra/tf").WithOutput(opts.Output, opts.Output)

	args := []string{"plan"}
	if opts.Destroy {
		args = append(args, "-destroy")
	}

	return exec.Mise(ctx, "terraform", args...)
}
