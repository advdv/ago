package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
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
			&cli.StringFlag{
				Name:  "project-dir",
				Usage: "Project directory (defaults to current directory)",
			},
		},
		Action: runDiff,
	}
}

func runDiff(ctx context.Context, cmd *cli.Command) error {
	projectDir := cmd.String("project-dir")
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	return doDiff(ctx, cdkCommandOptions{
		ProjectDir: projectDir,
		Deployment: cmd.Args().First(),
		All:        cmd.Bool("all"),
		Output:     os.Stdout,
	})
}

func doDiff(ctx context.Context, opts cdkCommandOptions) error {
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

	username, usernameErr := getCallerUsername(ctx, opts.ProjectDir, cdkContext)

	deployment, err := resolveDeploymentIdent(ctx, opts, "", prefix, cdkContext, username, usernameErr)
	if err != nil {
		return err
	}

	profile := resolveProfile(ctx, opts.ProjectDir, cdkContext, qualifier, username)

	args := buildCDKArgs(profile, qualifier, prefix, cdkContext)

	if opts.All {
		args = append(args, "--all")
	} else {
		args = append(args, qualifier+"*Shared", qualifier+"*"+deployment)
	}

	return runCDKCommand(ctx, opts.ProjectDir, cdkDir, opts.Output, "diff", args)
}
