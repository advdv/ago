package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func removeDeployerCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove-deployer",
		Usage:     "Remove a deployer user from the project configuration",
		ArgsUsage: "<username>",
		Action:    config.WithConfig(runRemoveDeployer),
	}
}

type removeDeployerOptions struct {
	Username string
	Output   io.Writer
}

func runRemoveDeployer(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	username := cmd.Args().First()
	if username == "" {
		return errors.New("username argument is required")
	}

	return doRemoveDeployer(ctx, cfg, removeDeployerOptions{
		Username: username,
		Output:   os.Stdout,
	})
}

func doRemoveDeployer(_ context.Context, cfg config.Config, opts removeDeployerOptions) error {
	cdkDir := filepath.Join(cfg.ProjectDir, "infra", "cdk", "cdk")
	contextPath := filepath.Join(cdkDir, "cdk.context.json")

	cdkCtx, err := getCDKContext(cdkDir)
	if err != nil {
		return err
	}

	prefix, err := detectPrefix(cdkCtx)
	if err != nil {
		return err
	}

	deployers := extractStringSlice(cdkCtx, prefix+"deployers")
	devDeployers := extractStringSlice(cdkCtx, prefix+"dev-deployers")

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
	deployments := extractStringSlice(cdkCtx, prefix+"deployments")
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
