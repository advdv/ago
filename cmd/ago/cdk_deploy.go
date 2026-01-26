package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
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
			&cli.StringFlag{
				Name:  "project-dir",
				Usage: "Project directory (defaults to current directory)",
			},
		},
		Action: runDeploy,
	}
}

type cdkCommandOptions struct {
	ProjectDir string
	Deployment string
	All        bool
	Hotswap    bool
	Output     io.Writer
}

func runDeploy(ctx context.Context, cmd *cli.Command) error {
	projectDir := cmd.String("project-dir")
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	return doDeploy(ctx, cdkCommandOptions{
		ProjectDir: projectDir,
		Deployment: cmd.Args().First(),
		All:        cmd.Bool("all"),
		Hotswap:    cmd.Bool("hotswap"),
		Output:     os.Stdout,
	})
}

func doDeploy(ctx context.Context, opts cdkCommandOptions) error {
	cdkDir := filepath.Join(opts.ProjectDir, "infra", "cdk", "cdk")

	cdkContext, err := getCDKContext(ctx, cdkDir)
	if err != nil {
		return err
	}

	prefix, err := detectPrefix(cdkContext)
	if err != nil {
		return err
	}

	qualifier, ok := cdkContext[prefix+"qualifier"].(string)
	if !ok || qualifier == "" {
		return errors.Errorf("qualifier not found at context key %q", prefix+"qualifier")
	}

	username, usernameErr := getCallerUsername(ctx, opts.ProjectDir, qualifier, cdkContext)

	deployment, err := resolveDeploymentIdent(ctx, opts, "", prefix, cdkContext, username, usernameErr)
	if err != nil {
		return err
	}

	profile := resolveProfile(ctx, opts.ProjectDir, cdkContext, qualifier, username)

	isFullDep, err := isFullDeployer(ctx, opts.ProjectDir, profile, qualifier, username)
	if err != nil {
		return err
	}

	if err := checkDeploymentPermission(deployment, isFullDep); err != nil {
		return err
	}

	args := buildCDKArgs(profile, qualifier, prefix, cdkContext)

	if opts.All {
		args = append(args, "--all", "--require-approval", "never")
	} else {
		args = append(args, qualifier+"*Shared", qualifier+"*"+deployment)
		args = append(args, "--require-approval", "never")
	}

	if opts.Hotswap {
		args = append(args, "--hotswap")
	}

	return runCDKCommand(ctx, opts.ProjectDir, cdkDir, opts.Output, "deploy", args)
}
