package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
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
			&cli.StringFlag{
				Name:  "project-dir",
				Usage: "Project directory (defaults to current directory)",
			},
		},
		Action: runDestroy,
	}
}

func runDestroy(ctx context.Context, cmd *cli.Command) error {
	projectDir := cmd.String("project-dir")
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	return doDestroy(ctx, cdkDestroyOptions{
		ProjectDir: projectDir,
		Deployment: cmd.Args().First(),
		All:        cmd.Bool("all"),
		Force:      cmd.Bool("force"),
		Output:     os.Stdout,
	})
}

type cdkDestroyOptions struct {
	ProjectDir string
	Deployment string
	All        bool
	Force      bool
	Output     io.Writer
}

func doDestroy(ctx context.Context, opts cdkDestroyOptions) error {
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

	deployment, err := resolveDeploymentIdent(ctx, cdkCommandOptions{
		ProjectDir: opts.ProjectDir,
		Deployment: opts.Deployment,
		All:        opts.All,
	}, "", prefix, cdkContext, username, usernameErr)
	if err != nil {
		return err
	}

	profile := resolveProfile(ctx, opts.ProjectDir, cdkContext, qualifier, username)

	userGroups, err := getUserGroups(ctx, opts.ProjectDir, profile, username)
	if err != nil {
		return err
	}

	if err := checkDeploymentPermission(deployment, isFullDeployer(userGroups, qualifier)); err != nil {
		return err
	}

	args := buildCDKArgs(profile, qualifier, prefix, userGroups)

	if opts.All {
		args = append(args, "--all")
	} else {
		args = append(args, qualifier+"*Shared", qualifier+"*"+deployment)
	}

	if opts.Force {
		args = append(args, "--force")
	}

	return runCDKCommand(ctx, opts.ProjectDir, cdkDir, opts.Output, "destroy", args)
}
