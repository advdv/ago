package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

	qualifier, _ := cdkContext[prefix+"qualifier"].(string)
	isFirstDeployer := len(deployers) == 0 && len(devDeployers) == 0
	if isFirstDeployer && qualifier != "" {
		if err := setCDKJSONProfile(cdkDir, qualifier, opts.Username); err != nil {
			writeOutputf(opts.Output, "Warning: could not update cdk.json profile: %v\n", err)
		} else {
			writeOutputf(opts.Output, "Updated cdk.json profile to %q\n", qualifier+"-"+strings.ToLower(opts.Username))
		}
	}

	writeOutputf(opts.Output, "Run 'ago cdk bootstrap' to create the user and configure credentials.\n")
	return nil
}

func setCDKJSONProfile(cdkDir, qualifier, username string) error {
	cdkJSONPath := filepath.Join(cdkDir, "cdk.json")

	data, err := os.ReadFile(cdkJSONPath)
	if err != nil {
		return errors.Wrap(err, "failed to read cdk.json")
	}

	var cdkJSON map[string]any
	if err := json.Unmarshal(data, &cdkJSON); err != nil {
		return errors.Wrap(err, "failed to parse cdk.json")
	}

	profileName := qualifier + "-" + strings.ToLower(username)
	cdkJSON["profile"] = profileName

	output, err := json.MarshalIndent(cdkJSON, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cdk.json")
	}

	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(cdkJSONPath, output, 0o644); err != nil {
		return errors.Wrap(err, "failed to write cdk.json")
	}

	return nil
}
