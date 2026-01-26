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
	"slices"
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

// validateDeployerUsername checks that the deployer username starts with a capital letter.
// This is important for CDK resource naming schemes, where the username is used to construct
// deployment identifiers like "DevAdam" or "DevBob". Starting with a capital ensures consistent
// PascalCase naming in CloudFormation resource names and stack identifiers.
var deployerUsernameRegex = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)

func validateDeployerUsername(username string) error {
	if !deployerUsernameRegex.MatchString(username) {
		return errors.Errorf(
			"invalid deployer username %q: must start with a capital letter (e.g., 'Adam', not 'adam')",
			username,
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
			addDeployerCmd(),
			removeDeployerCmd(),
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
		Action: runCreateProjectAccount,
	}
}

type createAccountOptions struct {
	ProjectDir        string
	ProjectName       string
	ManagementProfile string
	Region            string
	WriteProfile      bool
	EmailPattern      string
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
		EmailPattern:      cmd.String("email-pattern"),
		Output:            os.Stdout,
	}

	return doCreateProjectAccount(ctx, opts)
}

func doCreateProjectAccount(ctx context.Context, opts createAccountOptions) error {
	if opts.EmailPattern == "" {
		return errors.New("email pattern is required for account creation")
	}

	templatePath, cleanup, err := renderAccountStackTemplate(opts.ProjectName, opts.EmailPattern)
	if err != nil {
		return errors.Wrap(err, "failed to render account stack template")
	}
	defer cleanup()

	stackName := "ago-account-" + opts.ProjectName

	writeOutputf(opts.Output, "Deploying account stack %q...\n", stackName)

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
	deployers := extractStringSlice(cdkContext, prefix+"deployers")
	devDeployers := extractStringSlice(cdkContext, prefix+"dev-deployers")

	writeOutputf(opts.Output, "Verifying AWS access with profile %q...\n", profile)
	if err := verifyAWSAccess(ctx, opts, profile); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Deploying pre-bootstrap stack...\n")
	if len(deployers) > 0 {
		writeOutputf(opts.Output, "  Deployers: %s\n", strings.Join(deployers, ", "))
	}
	if len(devDeployers) > 0 {
		writeOutputf(opts.Output, "  Dev deployers: %s\n", strings.Join(devDeployers, ", "))
	}

	preBootstrapStackName := qualifier + "-pre-bootstrap"

	templatePath, cleanup, err := renderPreBootstrapTemplate(qualifier)
	if err != nil {
		return errors.Wrap(err, "failed to render pre-bootstrap template")
	}
	defer cleanup()

	err = deployPreBootstrapStack(
		ctx, opts, profile, preBootstrapStackName, templatePath, qualifier,
		secondaryRegions, deployers, devDeployers)
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

	writeOutputf(opts.Output, "Syncing deployer credentials...\n")
	if err := syncDeployerCredentials(ctx, opts, profile, qualifier, deployers, devDeployers); err != nil {
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
	profile, stackName, templatePath, qualifier string,
	secondaryRegions, deployers, devDeployers []string,
) error {
	secondaryRegionsParam := strings.Join(secondaryRegions, ",")
	deployersParam := strings.Join(deployers, ",")
	devDeployersParam := strings.Join(devDeployers, ",")

	args := []string{
		"exec", "--",
		"aws", "cloudformation", "deploy",
		"--stack-name", stackName,
		"--template-file", templatePath,
		"--parameter-overrides",
		"Qualifier=" + qualifier,
		"SecondaryRegions=" + secondaryRegionsParam,
		"Deployers=" + deployersParam,
		"DevDeployers=" + devDeployersParam,
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

	if err := writeContextFile(contextPath, contextJSON); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Run 'ago cdk bootstrap' to create the user and configure credentials.\n")
	return nil
}

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

	if err := writeContextFile(contextPath, contextJSON); err != nil {
		return err
	}

	writeOutputf(opts.Output,
		"Run 'ago cdk bootstrap' to delete the user and remove credentials from ~/.aws.\n")
	return nil
}

func readContextFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cdk.context.json")
	}

	var context map[string]any
	if err := json.Unmarshal(data, &context); err != nil {
		return nil, errors.Wrap(err, "failed to parse cdk.context.json")
	}

	return context, nil
}

func writeContextFile(path string, context map[string]any) error {
	output, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cdk.context.json")
	}

	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(path, output, 0o644); err != nil {
		return errors.Wrap(err, "failed to write cdk.context.json")
	}

	return nil
}

func getSecretValue(ctx context.Context, profile, projectDir, secretName string) (string, error) {
	cmd := exec.CommandContext(ctx, "mise", "exec", "--",
		"aws", "secretsmanager", "get-secret-value",
		"--secret-id", secretName,
		"--query", "SecretString",
		"--output", "text",
		"--profile", profile,
	)
	cmd.Dir = projectDir

	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get secret %q", secretName)
	}

	return strings.TrimSpace(string(output)), nil
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func syncDeployerCredentials(
	ctx context.Context, opts bootstrapOptions, profile, qualifier string,
	deployers, devDeployers []string,
) error {
	existingProfiles, err := listDeployerProfiles(ctx, opts.ProjectDir, qualifier)
	if err != nil {
		writeOutputf(opts.Output, "  Warning: could not list existing profiles: %v\n", err)
		existingProfiles = nil
	}

	type deployerInfo struct {
		username   string
		secretPath string
	}
	expectedProfiles := make(map[string]deployerInfo)
	for _, username := range deployers {
		profileName := qualifier + "-" + strings.ToLower(username)
		expectedProfiles[profileName] = deployerInfo{
			username:   username,
			secretPath: qualifier + "/deployers/" + username,
		}
	}
	for _, username := range devDeployers {
		profileName := qualifier + "-" + strings.ToLower(username)
		expectedProfiles[profileName] = deployerInfo{
			username:   username,
			secretPath: qualifier + "/dev-deployers/" + username,
		}
	}

	for _, existingProfile := range existingProfiles {
		if _, expected := expectedProfiles[existingProfile]; !expected {
			writeOutputf(opts.Output, "  Removing profile %q...\n", existingProfile)
			if err := removeAWSProfile(ctx, opts.ProjectDir, existingProfile); err != nil {
				writeOutputf(opts.Output, "    Warning: failed to remove profile: %v\n", err)
			}
		}
	}

	for profileName, info := range expectedProfiles {
		credentialsJSON, err := getSecretValue(ctx, profile, opts.ProjectDir, info.secretPath)
		if err != nil {
			writeOutputf(opts.Output, "  Warning: could not fetch credentials for %s: %v\n", info.username, err)
			continue
		}

		var credentials struct {
			AccessKeyID     string `json:"aws_access_key_id"`
			SecretAccessKey string `json:"aws_secret_access_key"`
		}
		if err := json.Unmarshal([]byte(credentialsJSON), &credentials); err != nil {
			writeOutputf(opts.Output, "  Warning: could not parse credentials for %s: %v\n", info.username, err)
			continue
		}

		writeOutputf(opts.Output, "  Configuring profile %q for user %s...\n", profileName, info.username)
		err = writeDeployerProfile(ctx, opts.ProjectDir, profileName,
			credentials.AccessKeyID, credentials.SecretAccessKey)
		if err != nil {
			writeOutputf(opts.Output, "    Warning: failed to write profile: %v\n", err)
		}
	}

	return nil
}

func listDeployerProfiles(ctx context.Context, projectDir, qualifier string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get home directory")
	}

	credentialsPath := filepath.Join(home, ".aws", "credentials")
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to read credentials file")
	}

	prefix := "[" + qualifier + "-"
	var profiles []string
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, "]") {
			profileName := line[1 : len(line)-1]
			profiles = append(profiles, profileName)
		}
	}

	return profiles, nil
}

func removeAWSProfile(ctx context.Context, projectDir, profileName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get home directory")
	}

	if err := removeProfileFromFile(
		filepath.Join(home, ".aws", "credentials"), profileName); err != nil {
		return err
	}

	if err := removeProfileFromFile(
		filepath.Join(home, ".aws", "config"), "profile "+profileName); err != nil {
		return err
	}

	return nil
}

func removeProfileFromFile(filePath, sectionName string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "failed to read %s", filePath)
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	inSection := false
	sectionHeader := "[" + sectionName + "]"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == sectionHeader {
			inSection = true
			continue
		}

		if inSection && strings.HasPrefix(trimmed, "[") {
			inSection = false
		}

		if !inSection {
			result = append(result, line)
		}
	}

	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	output := strings.Join(result, "\n")
	if output != "" {
		output += "\n"
	}

	if err := os.WriteFile(filePath, []byte(output), 0o600); err != nil {
		return errors.Wrapf(err, "failed to write %s", filePath)
	}

	return nil
}

func writeDeployerProfile(
	ctx context.Context, projectDir, profileName, accessKeyID, secretAccessKey string,
) error {
	settings := []struct{ key, value string }{
		{"aws_access_key_id", accessKeyID},
		{"aws_secret_access_key", secretAccessKey},
		{"region", "eu-central-1"},
		{"cli_pager", ""},
	}

	for _, s := range settings {
		//nolint:gosec // arguments are validated
		cmd := exec.CommandContext(ctx, "mise", "exec", "--",
			"aws", "configure", "set", s.key, s.value, "--profile", profileName)
		cmd.Dir = projectDir
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "failed to set %s for profile %s", s.key, profileName)
		}
	}

	return nil
}
