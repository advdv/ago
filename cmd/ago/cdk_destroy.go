package main

import (
	"context"
	"io"
	"os"

	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/urfave/cli/v3"
)

func destroyCmd() *cli.Command {
	return &cli.Command{
		Name:      "destroy",
		Usage:     "Destroy CDK stacks",
		ArgsUsage: "[deployment]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Destroy all stacks",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Skip confirmation prompts",
			},
		},
		Action: config.WithConfig(runDestroy),
	}
}

type cdkDestroyOptions struct {
	Deployment string
	All        bool
	Force      bool
	Output     io.Writer
}

func runDestroy(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doDestroy(ctx, cfg, cdkDestroyOptions{
		Deployment: cmd.Args().First(),
		All:        cmd.Bool("all"),
		Force:      cmd.Bool("force"),
		Output:     os.Stdout,
	})
}

func doDestroy(ctx context.Context, cfg config.Config, opts cdkDestroyOptions) error {
	cdk, err := loadCDKContext(cfg)
	if err != nil {
		return err
	}

	exec := cdk.Exec.WithOutput(opts.Output, opts.Output)
	cdkExec := cdk.CDKExec.WithOutput(opts.Output, opts.Output)

	username, usernameErr := getCallerUsername(ctx, exec, cdk.Qualifier, cdk.CDKContext)

	deployment, err := resolveDeploymentIdent(cdkCommandOptions{
		Deployment: opts.Deployment,
		All:        opts.All,
	}, cdk.Prefix, cdk.CDKContext, username, usernameErr)
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
		args = append(args, "--all")
	} else {
		args = append(args, cdk.Qualifier+"*Shared", cdk.Qualifier+"*"+deployment)
	}

	if opts.Force {
		args = append(args, "--force")
	}

	return runCDKCommand(ctx, cdkExec, "destroy", args)
}
