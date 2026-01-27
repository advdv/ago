package main

import (
	"context"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func deployCmd() *cli.Command {
	return &cli.Command{
		Name:      "deploy",
		Usage:     "Deploy CDK stacks",
		ArgsUsage: "[deployment]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "hotswap",
				Usage: "Enable CDK hotswap for faster iterations",
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Deploy all stacks",
			},
		},
		Action: config.RunWithConfig(runDeploy),
	}
}

func runDeploy(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doDeploy(ctx, cfg, cdkCommandOptions{
		Deployment: cmd.Args().First(),
		All:        cmd.Bool("all"),
		Hotswap:    cmd.Bool("hotswap"),
		Output:     os.Stdout,
	})
}

func doDeploy(ctx context.Context, cfg config.Config, opts cdkCommandOptions) error {
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

	if err := checkDeploymentPermission(deployment, isFullDeployer(userGroups, cdk.Qualifier)); err != nil {
		return err
	}

	args := buildCDKArgs(profile, cdk.Qualifier, cdk.Prefix, userGroups)

	if opts.All {
		args = append(args, "--all", "--require-approval", "never")
	} else {
		args = append(args, cdk.Qualifier+"*Shared", cdk.Qualifier+"*"+deployment)
		args = append(args, "--require-approval", "never")
	}

	if opts.Hotswap {
		args = append(args, "--hotswap")
	}

	return runCDKCommand(ctx, cdkExec, "deploy", args)
}
