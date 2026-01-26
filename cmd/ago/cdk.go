package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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
			bootstrapCmd(),
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

func bootstrapCmd() *cli.Command {
	return &cli.Command{
		Name:      "bootstrap",
		Usage:     "Bootstrap CDK in the AWS account",
		ArgsUsage: "[project-directory]",
		Action:    runBootstrap,
	}
}

type bootstrapOptions struct {
	ProjectDir string
	Output     io.Writer
}

func runBootstrap(ctx context.Context, cmd *cli.Command) error {
	projectDir := cmd.Args().First()
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	return doBootstrap(ctx, bootstrapOptions{
		ProjectDir: projectDir,
		Output:     os.Stdout,
	})
}

func doBootstrap(ctx context.Context, opts bootstrapOptions) error {
	cdkDir := filepath.Join(opts.ProjectDir, "infra", "cdk", "cdk")

	writeOutputf(opts.Output, "Reading CDK context...\n")
	cdkContext, err := getCDKContext(ctx, cdkDir)
	if err != nil {
		return err
	}

	profile, ok := cdkContext["admin-profile"].(string)
	if !ok || profile == "" {
		return errors.New("admin-profile not found in cdk.json - was 'ago cdk create-project-account' run?")
	}

	prefix, err := detectPrefix(cdkContext)
	if err != nil {
		return err
	}

	qualifier, ok := cdkContext[prefix+"qualifier"].(string)
	if !ok || qualifier == "" {
		return errors.Errorf("qualifier not found at context key %q", prefix+"qualifier")
	}

	secondaryRegions := extractStringSlice(cdkContext, prefix+"secondary-regions")

	writeOutputf(opts.Output, "Verifying AWS access with profile %q...\n", profile)
	if err := verifyAWSAccess(ctx, opts, profile); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Deploying pre-bootstrap stack...\n")
	preBootstrapStackName := qualifier + "-pre-bootstrap"
	templatePath := filepath.Join(opts.ProjectDir, "infra", "cfn", "pre-bootstrap.cfn.yaml")

	err = deployPreBootstrapStack(
		ctx, opts, profile, preBootstrapStackName, templatePath, qualifier, secondaryRegions)
	if err != nil {
		return err
	}

	executionPolicyArn, err := getStackOutputWithProfile(
		ctx, opts, profile, preBootstrapStackName, "ExecutionPolicyArn")
	if err != nil {
		return err
	}

	permissionsBoundaryName, err := getStackOutputWithProfile(
		ctx, opts, profile, preBootstrapStackName, "PermissionsBoundaryName")
	if err != nil {
		return err
	}

	boundaryConfig, ok := cdkContext["@aws-cdk/core:permissionsBoundary"].(map[string]any)
	if !ok {
		return errors.New("@aws-cdk/core:permissionsBoundary not found in cdk.context.json")
	}
	contextBoundaryName, ok := boundaryConfig["name"].(string)
	if !ok || contextBoundaryName == "" {
		return errors.New("@aws-cdk/core:permissionsBoundary.name not found in cdk.context.json")
	}

	if contextBoundaryName != permissionsBoundaryName {
		return errors.Errorf(
			"CDK context @aws-cdk/core:permissionsBoundary.name (%q) must match pre-bootstrap output (%q)",
			contextBoundaryName, permissionsBoundaryName,
		)
	}

	writeOutputf(opts.Output, "Running CDK bootstrap...\n")
	err = runCDKBootstrap(
		ctx, opts, cdkDir, profile, qualifier, executionPolicyArn, permissionsBoundaryName)
	if err != nil {
		return err
	}

	writeOutputf(opts.Output, "Bootstrap complete!\n")
	return nil
}

func getCDKContext(ctx context.Context, cdkDir string) (map[string]any, error) {
	cdkJSONPath := filepath.Join(cdkDir, "cdk.json")
	cdkContextPath := filepath.Join(cdkDir, "cdk.context.json")

	result := make(map[string]any)

	cdkJSONData, err := os.ReadFile(cdkJSONPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cdk.json")
	}
	var cdkJSON map[string]any
	if err := json.Unmarshal(cdkJSONData, &cdkJSON); err != nil {
		return nil, errors.Wrap(err, "failed to parse cdk.json")
	}
	maps.Copy(result, cdkJSON)

	cdkContextData, err := os.ReadFile(cdkContextPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cdk.context.json")
	}
	var cdkContextJSON map[string]any
	if err := json.Unmarshal(cdkContextData, &cdkContextJSON); err != nil {
		return nil, errors.Wrap(err, "failed to parse cdk.context.json")
	}
	maps.Copy(result, cdkContextJSON)

	return result, nil
}

func detectPrefix(context map[string]any) (string, error) {
	for key := range context {
		if idx := len(key) - len("qualifier"); idx > 0 && key[idx:] == "qualifier" {
			return key[:idx], nil
		}
	}
	return "", errors.New("could not detect context prefix - no key ending with 'qualifier' found")
}

func extractStringSlice(context map[string]any, key string) []string {
	val, ok := context[key]
	if !ok {
		return nil
	}
	slice, ok := val.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func verifyAWSAccess(ctx context.Context, opts bootstrapOptions, profile string) error {
	cmd := exec.CommandContext(ctx, "mise", "exec", "--",
		"aws", "sts", "get-caller-identity", "--profile", profile)
	cmd.Dir = opts.ProjectDir
	cmd.Stdout = opts.Output
	cmd.Stderr = opts.Output
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to verify AWS access")
	}
	return nil
}

func deployPreBootstrapStack(
	ctx context.Context, opts bootstrapOptions,
	profile, stackName, templatePath, qualifier string, secondaryRegions []string,
) error {
	secondaryRegionsParam := ""
	if len(secondaryRegions) > 0 {
		secondaryRegionsParam = strings.Join(secondaryRegions, ",")
	}

	args := []string{
		"exec", "--",
		"aws", "cloudformation", "deploy",
		"--stack-name", stackName,
		"--template-file", templatePath,
		"--parameter-overrides",
		"Qualifier=" + qualifier,
		"SecondaryRegions=" + secondaryRegionsParam,
		"--capabilities", "CAPABILITY_NAMED_IAM",
		"--no-fail-on-empty-changeset",
		"--profile", profile,
	}

	cmd := exec.CommandContext(ctx, "mise", args...)
	cmd.Dir = opts.ProjectDir
	cmd.Stdout = opts.Output
	cmd.Stderr = opts.Output
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to deploy pre-bootstrap stack")
	}
	return nil
}

func getStackOutputWithProfile(
	ctx context.Context, opts bootstrapOptions, profile, stackName, outputKey string,
) (string, error) {
	cmd := exec.CommandContext(ctx, "mise", "exec", "--",
		"aws", "cloudformation", "describe-stacks",
		"--stack-name", stackName,
		"--query", "Stacks[0].Outputs",
		"--output", "json",
		"--profile", profile,
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

func runCDKBootstrap(
	ctx context.Context, opts bootstrapOptions, cdkDir string,
	profile, qualifier, executionPolicyArn, permissionsBoundaryName string,
) error {
	toolkitStackName := qualifier + "Bootstrap"

	cmd := exec.CommandContext(ctx, "mise", "exec", "--",
		"cdk", "bootstrap",
		"--profile", profile,
		"--qualifier", qualifier,
		"--toolkit-stack-name", toolkitStackName,
		"--cloudformation-execution-policies", executionPolicyArn,
		"--custom-permissions-boundary", permissionsBoundaryName,
	)
	cmd.Dir = cdkDir
	cmd.Stdout = opts.Output
	cmd.Stderr = opts.Output
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "cdk bootstrap failed")
	}
	return nil
}
