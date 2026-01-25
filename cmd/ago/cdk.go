package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

var projectNameRegex = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

func validateProjectName(name string) error {
	if !projectNameRegex.MatchString(name) {
		return errors.Errorf(
			"invalid project name %q: must start with a lowercase letter and contain only lowercase letters and numbers",
			name,
		)
	}
	return nil
}

func cdkCmd() *cli.Command {
	return &cli.Command{
		Name:  "cdk",
		Usage: "CDK account and infrastructure management",
		Commands: []*cli.Command{
			createProjectAccountCmd(),
		},
	}
}

func createProjectAccountCmd() *cli.Command {
	return &cli.Command{
		Name:      "create-project-account",
		Usage:     "Create a new AWS account in the organization for a project",
		ArgsUsage: "[project-directory]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "management-profile",
				Usage:    "AWS profile for the management account",
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
		Action: runCreateProjectAccount,
	}
}

type createAccountOptions struct {
	ProjectDir        string
	ProjectName       string
	ManagementProfile string
	Region            string
	WriteProfile      bool
	Output            io.Writer
}

func runCreateProjectAccount(ctx context.Context, cmd *cli.Command) error {
	projectDir := cmd.Args().First()
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	projectName := filepath.Base(projectDir)
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	opts := createAccountOptions{
		ProjectDir:        projectDir,
		ProjectName:       projectName,
		ManagementProfile: cmd.String("management-profile"),
		Region:            cmd.String("region"),
		WriteProfile:      cmd.Bool("write-profile"),
		Output:            os.Stdout,
	}

	return doCreateProjectAccount(ctx, opts)
}

func doCreateProjectAccount(ctx context.Context, opts createAccountOptions) error {
	templatePath := filepath.Join(opts.ProjectDir, "infra", "cfn", "account-stack.yaml")
	if _, err := os.Stat(templatePath); err != nil {
		return errors.Errorf("account stack template not found at %s - was 'ago init' run?", templatePath)
	}

	stackName := "ago-account-" + opts.ProjectName

	writeOutputf(opts.Output, "Deploying account stack %q using template %s...\n", stackName, templatePath)

	if err := deployStackWithCLI(ctx, opts, stackName, templatePath); err != nil {
		return err
	}

	accountID, err := getStackOutputWithCLI(ctx, opts, stackName, "AccountId")
	if err != nil {
		return err
	}

	writeOutputf(opts.Output, "Account created successfully!\n")
	writeOutputf(opts.Output, "  Account ID: %s\n", accountID)
	writeOutputf(opts.Output, "  Account Name: %s\n", opts.ProjectName)

	if opts.WriteProfile {
		profileName := opts.ProjectName + "-admin"
		if err := writeAWSProfile(ctx, opts, profileName, accountID); err != nil {
			return err
		}
		writeOutputf(opts.Output, "  AWS Profile: %s (written to ~/.aws/config)\n", profileName)

		if err := updateCDKContextProfile(opts, profileName); err != nil {
			return err
		}

		if err := updateCDKJSONProfile(opts, profileName); err != nil {
			return err
		}
	}

	return nil
}

func writeOutputf(w io.Writer, format string, args ...any) {
	if w != nil {
		_, _ = fmt.Fprintf(w, format, args...)
	}
}

func deployStackWithCLI(ctx context.Context, opts createAccountOptions, stackName, templatePath string) error {
	//nolint:gosec // arguments are validated
	cmd := exec.CommandContext(ctx,
		"mise", "exec", "--",
		"aws", "cloudformation", "deploy",
		"--stack-name", stackName,
		"--template-file", templatePath,
		"--region", opts.Region,
		"--profile", opts.ManagementProfile,
		"--no-fail-on-empty-changeset",
	)
	cmd.Dir = opts.ProjectDir
	cmd.Stdout = opts.Output
	cmd.Stderr = opts.Output

	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "aws cloudformation deploy failed")
	}

	return nil
}

func getStackOutputWithCLI(
	ctx context.Context, opts createAccountOptions, stackName, outputKey string,
) (string, error) {
	//nolint:gosec // arguments are validated
	cmd := exec.CommandContext(ctx,
		"mise", "exec", "--",
		"aws", "cloudformation", "describe-stacks",
		"--stack-name", stackName,
		"--region", opts.Region,
		"--profile", opts.ManagementProfile,
		"--query", "Stacks[0].Outputs",
		"--output", "json",
	)
	cmd.Dir = opts.ProjectDir

	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to describe stack")
	}

	var outputs []struct {
		OutputKey   string `json:"OutputKey"`   //nolint:tagliatelle // AWS API uses PascalCase
		OutputValue string `json:"OutputValue"` //nolint:tagliatelle // AWS API uses PascalCase
	}
	if err := json.Unmarshal(output, &outputs); err != nil {
		return "", errors.Wrap(err, "failed to parse stack outputs")
	}

	for _, o := range outputs {
		if o.OutputKey == outputKey {
			return o.OutputValue, nil
		}
	}

	return "", errors.Errorf("output %q not found in stack %q", outputKey, stackName)
}

func updateCDKContextProfile(opts createAccountOptions, profileName string) error {
	contextPath := filepath.Join(opts.ProjectDir, "infra", "cdk", "cdk", "cdk.context.json")

	data, err := os.ReadFile(contextPath)
	if err != nil {
		return errors.Wrap(err, "failed to read cdk.context.json")
	}

	var context map[string]any
	if err := json.Unmarshal(data, &context); err != nil {
		return errors.Wrap(err, "failed to parse cdk.context.json")
	}

	context[opts.ProjectName+"-admin-profile"] = profileName

	output, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cdk.context.json")
	}

	if err := os.WriteFile(contextPath, output, 0o644); err != nil { //nolint:gosec // config file needs to be readable
		return errors.Wrap(err, "failed to write cdk.context.json")
	}

	return nil
}

func updateCDKJSONProfile(opts createAccountOptions, profileName string) error {
	cdkJSONPath := filepath.Join(opts.ProjectDir, "infra", "cdk", "cdk", "cdk.json")

	data, err := os.ReadFile(cdkJSONPath)
	if err != nil {
		return errors.Wrap(err, "failed to read cdk.json")
	}

	var cdkJSON map[string]any
	if err := json.Unmarshal(data, &cdkJSON); err != nil {
		return errors.Wrap(err, "failed to parse cdk.json")
	}

	cdkJSON["profile"] = profileName

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
	ctx context.Context, opts createAccountOptions, profileName, accountID string,
) error {
	roleArn := "arn:aws:iam::" + accountID + ":role/OrganizationAccountAccessRole"

	settings := []struct{ key, value string }{
		{"role_arn", roleArn},
		{"source_profile", opts.ManagementProfile},
		{"region", opts.Region},
		{"cli_pager", ""},
	}

	for _, s := range settings {
		//nolint:gosec // arguments are validated
		cmd := exec.CommandContext(ctx, "mise", "exec", "--",
			"aws", "configure", "set", s.key, s.value, "--profile", profileName)
		cmd.Dir = opts.ProjectDir
		cmd.Stdout = opts.Output
		cmd.Stderr = opts.Output
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "failed to set %s for profile %s", s.key, profileName)
		}
	}

	return nil
}
