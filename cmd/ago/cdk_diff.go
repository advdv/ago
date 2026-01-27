package main

import (
	"context"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func diffCmd() *cli.Command {
	return &cli.Command{
		Name:      "diff",
		Usage:     "Show CDK stack differences",
		ArgsUsage: "[deployment]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Diff all stacks",
			},
		},
		Action: config.WithConfig(runDiff),
	}
}

func runDiff(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doDiff(ctx, cfg, cdkCommandOptions{
		Deployment: cmd.Args().First(),
		All:        cmd.Bool("all"),
		Output:     os.Stdout,
	})
}

func doDiff(ctx context.Context, cfg config.Config, opts cdkCommandOptions) error {
	cdk, err := loadCDKContext(cfg)
	if err != nil {
		return err
	}

	exec := cdk.Exec.WithOutput(opts.Output, opts.Output)
	cdkExec := cdk.CDKExec.WithOutput(opts.Output, opts.Output)

	username, usernameErr := getCallerUsername(ctx, exec, cdk.Qualifier, cdk.CDKContext)

	deployment, err := resolveDeploymentIdent(opts, cdk.Prefix, cdk.CDKContext, username, usernameErr)
	if err != nil {
		return err
	}

	profile := resolveProfile(ctx, exec, cdk.CDKContext, cdk.Qualifier, username)

	userGroups, err := getUserGroups(ctx, exec, profile, username)
	if err != nil {
		return err
	}

	args := buildCDKArgs(profile, cdk.Qualifier, cdk.Prefix, userGroups)

	if opts.All {
		args = append(args, "--all")
	} else {
		args = append(args, cdk.Qualifier+"*Shared", cdk.Qualifier+"*"+deployment)
	}

	return runCDKCommand(ctx, cdkExec, "diff", args)
}
