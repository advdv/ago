package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func removeDeployerCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove-deployer",
		Usage:     "Remove a deployer user from the project configuration",
		ArgsUsage: "<username>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "project-dir",
				Usage: "Project directory (defaults to current directory)",
			},
		},
		Action: runRemoveDeployer,
	}
}

func runRemoveDeployer(ctx context.Context, cmd *cli.Command) error {
	username := cmd.Args().First()
	if username == "" {
		return errors.New("username argument is required")
	}

	projectDir := cmd.String("project-dir")
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	opts := deployerOptions{
		ProjectDir: projectDir,
		Username:   username,
		Output:     os.Stdout,
	}

	return doRemoveDeployer(ctx, opts)
}

func doRemoveDeployer(ctx context.Context, opts deployerOptions) error {
	cdkDir := filepath.Join(opts.ProjectDir, "infra", "cdk", "cdk")
	contextPath := filepath.Join(cdkDir, "cdk.context.json")

	cdkContext, err := getCDKContext(ctx, cdkDir)
	if err != nil {
		return err
	}

	prefix, err := detectPrefix(cdkContext)
	if err != nil {
		return err
	}

	deployers := extractStringSlice(cdkContext, prefix+"deployers")
	devDeployers := extractStringSlice(cdkContext, prefix+"dev-deployers")

	foundInDeployers := slices.Contains(deployers, opts.Username)
	foundInDevDeployers := slices.Contains(devDeployers, opts.Username)

	if !foundInDeployers && !foundInDevDeployers {
		return errors.Errorf("user %q not found in deployers or dev-deployers list", opts.Username)
	}

	contextJSON, err := readContextFile(contextPath)
	if err != nil {
		return err
	}

	if foundInDeployers {
		deployers = slices.DeleteFunc(deployers, func(s string) bool { return s == opts.Username })
		contextJSON[prefix+"deployers"] = deployers
		writeOutputf(opts.Output, "Removed %q from deployers in cdk.context.json\n", opts.Username)
	}
	if foundInDevDeployers {
		devDeployers = slices.DeleteFunc(devDeployers, func(s string) bool { return s == opts.Username })
		contextJSON[prefix+"dev-deployers"] = devDeployers
		writeOutputf(opts.Output, "Removed %q from dev-deployers in cdk.context.json\n", opts.Username)
	}

	deploymentIdent := "Dev" + opts.Username
	deployments := extractStringSlice(cdkContext, prefix+"deployments")
	if slices.Contains(deployments, deploymentIdent) {
		deployments = slices.DeleteFunc(deployments, func(s string) bool { return s == deploymentIdent })
		contextJSON[prefix+"deployments"] = deployments
		writeOutputf(opts.Output, "Removed %q from deployments in cdk.context.json\n", deploymentIdent)
	}

	if err := writeContextFile(contextPath, contextJSON); err != nil {
		return err
	}

	writeOutputf(opts.Output,
		"Run 'ago cdk bootstrap' to delete the user and remove credentials from ~/.aws.\n")
	return nil
}
