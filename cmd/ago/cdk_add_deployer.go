package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func addDeployerCmd() *cli.Command {
	return &cli.Command{
		Name:      "add-deployer",
		Usage:     "Add a deployer user to the project configuration",
		ArgsUsage: "<username>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dev",
				Usage: "Add to dev-deployers group instead of full deployers",
			},
			&cli.StringFlag{
				Name:  "project-dir",
				Usage: "Project directory (defaults to current directory)",
			},
		},
		Action: runAddDeployer,
	}
}

type deployerOptions struct {
	ProjectDir string
	Username   string
	DevOnly    bool
	Output     io.Writer
}

func runAddDeployer(ctx context.Context, cmd *cli.Command) error {
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
		DevOnly:    cmd.Bool("dev"),
		Output:     os.Stdout,
	}

	return doAddDeployer(ctx, opts)
}

func doAddDeployer(ctx context.Context, opts deployerOptions) error {
	if err := validateDeployerUsername(opts.Username); err != nil {
		return err
	}

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

	if slices.Contains(deployers, opts.Username) {
		return errors.Errorf("user %q already exists in deployers list", opts.Username)
	}
	if slices.Contains(devDeployers, opts.Username) {
		return errors.Errorf("user %q already exists in dev-deployers list", opts.Username)
	}

	contextJSON, err := readContextFile(contextPath)
	if err != nil {
		return err
	}

	if opts.DevOnly {
		devDeployers = append(devDeployers, opts.Username)
		contextJSON[prefix+"dev-deployers"] = devDeployers
		writeOutputf(opts.Output, "Added %q to dev-deployers in cdk.context.json\n", opts.Username)
	} else {
		deployers = append(deployers, opts.Username)
		contextJSON[prefix+"deployers"] = deployers
		writeOutputf(opts.Output, "Added %q to deployers in cdk.context.json\n", opts.Username)
	}

	deploymentIdent := "Dev" + opts.Username
	deployments := extractStringSlice(cdkContext, prefix+"deployments")
	if !slices.Contains(deployments, deploymentIdent) {
		deployments = append(deployments, deploymentIdent)
		contextJSON[prefix+"deployments"] = deployments
		writeOutputf(opts.Output, "Added %q to deployments in cdk.context.json\n", deploymentIdent)
	}

	if err := writeContextFile(contextPath, contextJSON); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Run 'ago cdk bootstrap' to create the user and configure credentials.\n")
	return nil
}
