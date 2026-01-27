package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func createProjectAccountCmd() *cli.Command {
	return &cli.Command{
		Name:  "create-project-account",
		Usage: "Create a new AWS account in the organization for a project",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "management-profile",
				Usage:    "AWS profile for the management account",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "email-pattern",
				Usage:    "Email pattern for the account (use {project} as placeholder)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: "AWS region for the CloudFormation stack",
				Value: "eu-central-1",
			},
			&cli.BoolFlag{
				Name:  "write-profile",
				Usage: "Write AWS CLI profile to ~/.aws/config",
				Value: true,
			},
		},
		Action: config.WithConfig(runCreateProjectAccount),
	}
}

type createAccountOptions struct {
	ProjectName       string
	ManagementProfile string
	Region            string
	WriteProfile      bool
	EmailPattern      string
	Output            io.Writer
}

func runCreateProjectAccount(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	projectName := filepath.Base(cfg.ProjectDir)
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	return doCreateProjectAccount(ctx, cfg, createAccountOptions{
		ProjectName:       projectName,
		ManagementProfile: cmd.String("management-profile"),
		Region:            cmd.String("region"),
		WriteProfile:      cmd.Bool("write-profile"),
		EmailPattern:      cmd.String("email-pattern"),
		Output:            os.Stdout,
	})
}

func doCreateProjectAccount(ctx context.Context, cfg config.Config, opts createAccountOptions) error {
	if opts.EmailPattern == "" {
		return errors.New("email pattern is required for account creation")
	}

	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.Output)

	templatePath, cleanup, err := renderAccountStackTemplate(opts.ProjectName, opts.EmailPattern)
	if err != nil {
		return errors.Wrap(err, "failed to render account stack template")
	}
	defer cleanup()

	stackName := "ago-account-" + opts.ProjectName

	writeOutputf(opts.Output, "Deploying account stack %q...\n", stackName)

	if err := deployAccountStack(ctx, exec, opts, stackName, templatePath); err != nil {
		return err
	}

	accountID, err := getAccountStackOutput(ctx, exec, opts, stackName, "AccountId")
	if err != nil {
		return err
	}

	writeOutputf(opts.Output, "Account created successfully!\n")
	writeOutputf(opts.Output, "  Account ID: %s\n", accountID)
	writeOutputf(opts.Output, "  Account Name: %s\n", opts.ProjectName)

	if opts.WriteProfile {
		profileName := opts.ProjectName + "-admin"
		if err := writeAWSProfile(ctx, exec, opts, profileName, accountID); err != nil {
			return err
		}
		writeOutputf(opts.Output, "  AWS Profile: %s (written to ~/.aws/config)\n", profileName)

		if err := updateCDKContextProfile(cfg.ProjectDir, opts.ProjectName, profileName); err != nil {
			return err
		}

		if err := updateCDKJSONProfile(cfg.ProjectDir, profileName); err != nil {
			return err
		}
	}

	return nil
}

func deployAccountStack(
	ctx context.Context, exec cmdexec.Executor, opts createAccountOptions, stackName, templatePath string,
) error {
	return exec.Mise(ctx, "aws", "cloudformation", "deploy",
		"--stack-name", stackName,
		"--template-file", templatePath,
		"--region", opts.Region,
		"--profile", opts.ManagementProfile,
		"--no-fail-on-empty-changeset",
	)
}

func getAccountStackOutput(
	ctx context.Context, exec cmdexec.Executor, opts createAccountOptions, stackName, outputKey string,
) (string, error) {
	output, err := exec.MiseOutput(ctx, "aws", "cloudformation", "describe-stacks",
		"--stack-name", stackName,
		"--region", opts.Region,
		"--profile", opts.ManagementProfile,
		"--query", "Stacks[0].Outputs",
		"--output", "json",
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to describe stack")
	}

	var outputs []struct {
		OutputKey   string `json:"OutputKey"`   //nolint:tagliatelle // AWS API uses PascalCase
		OutputValue string `json:"OutputValue"` //nolint:tagliatelle // AWS API uses PascalCase
	}
	if err := json.Unmarshal([]byte(output), &outputs); err != nil {
		return "", errors.Wrap(err, "failed to parse stack outputs")
	}

	for _, o := range outputs {
		if o.OutputKey == outputKey {
			return o.OutputValue, nil
		}
	}

	return "", errors.Errorf("output %q not found in stack %q", outputKey, stackName)
}

func updateCDKContextProfile(projectDir, projectName, profileName string) error {
	contextPath := filepath.Join(projectDir, "infra", "cdk", "cdk", "cdk.context.json")

	data, err := os.ReadFile(contextPath)
	if err != nil {
		return errors.Wrap(err, "failed to read cdk.context.json")
	}

	var context map[string]any
	if err := json.Unmarshal(data, &context); err != nil {
		return errors.Wrap(err, "failed to parse cdk.context.json")
	}

	context[projectName+"-admin-profile"] = profileName

	output, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cdk.context.json")
	}

	if err := os.WriteFile(contextPath, output, 0o644); err != nil { //nolint:gosec // config file needs to be readable
		return errors.Wrap(err, "failed to write cdk.context.json")
	}

	return nil
}

func updateCDKJSONProfile(projectDir, profileName string) error {
	cdkJSONPath := filepath.Join(projectDir, "infra", "cdk", "cdk", "cdk.json")

	data, err := os.ReadFile(cdkJSONPath)
	if err != nil {
		return errors.Wrap(err, "failed to read cdk.json")
	}

	var cdkJSON map[string]any
	if err := json.Unmarshal(data, &cdkJSON); err != nil {
		return errors.Wrap(err, "failed to parse cdk.json")
	}

	cdkJSON["profile"] = profileName
	cdkJSON["admin-profile"] = profileName

	output, err := json.MarshalIndent(cdkJSON, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cdk.json")
	}

	if err := os.WriteFile(cdkJSONPath, output, 0o644); err != nil { //nolint:gosec // config file needs to be readable
		return errors.Wrap(err, "failed to write cdk.json")
	}

	return nil
}

func writeAWSProfile(
	ctx context.Context, exec cmdexec.Executor, opts createAccountOptions, profileName, accountID string,
) error {
	roleArn := "arn:aws:iam::" + accountID + ":role/OrganizationAccountAccessRole"

	settings := []struct{ key, value string }{
		{"role_arn", roleArn},
		{"source_profile", opts.ManagementProfile},
		{"region", opts.Region},
		{"cli_pager", ""},
	}

	for _, s := range settings {
		if err := exec.Mise(ctx, "aws", "configure", "set", s.key, s.value, "--profile", profileName); err != nil {
			return errors.Wrapf(err, "failed to set %s for profile %s", s.key, profileName)
		}
	}

	return nil
}
