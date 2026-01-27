package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/config"
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
		},
		Action: config.RunWithConfig(runAddDeployer),
	}
}

type deployerOptions struct {
	Username string
	DevOnly  bool
	Output   io.Writer
}

func runAddDeployer(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	username := cmd.Args().First()
	if username == "" {
		return errors.New("username argument is required")
	}

	return doAddDeployer(ctx, cfg, deployerOptions{
		Username: username,
		DevOnly:  cmd.Bool("dev"),
		Output:   os.Stdout,
	})
}

func doAddDeployer(_ context.Context, cfg config.Config, opts deployerOptions) error {
	if err := validateDeployerUsername(opts.Username); err != nil {
		return err
	}

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
	isFirstDeployer := len(deployers) == 0 && len(devDeployers) == 0

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
	deployments := extractStringSlice(cdkCtx, prefix+"deployments")
	if !slices.Contains(deployments, deploymentIdent) {
		deployments = append(deployments, deploymentIdent)
		contextJSON[prefix+"deployments"] = deployments
		writeOutputf(opts.Output, "Added %q to deployments in cdk.context.json\n", deploymentIdent)
	}

	if err := writeContextFile(contextPath, contextJSON); err != nil {
		return err
	}

	qualifier, _ := cdkCtx[prefix+"qualifier"].(string)
	if isFirstDeployer && qualifier != "" {
		if err := setCDKJSONProfile(cdkDir, qualifier, opts.Username); err != nil {
			writeOutputf(opts.Output, "Warning: could not update cdk.json profile: %v\n", err)
		} else {
			writeOutputf(opts.Output, "Updated cdk.json profile to %q\n", qualifier+"-"+strings.ToLower(opts.Username))
		}
	}

	writeOutputf(opts.Output, "Run 'ago infra cdk bootstrap' to create the user and configure credentials.\n")
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
