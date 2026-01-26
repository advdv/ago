package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

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

	services, err := ParseServicesFromContext(cdkContext, prefix)
	if err != nil {
		return errors.Wrap(err, "failed to parse services from context")
	}

	writeOutputf(opts.Output, "Deploying pre-bootstrap stack...\n")
	if len(deployers) > 0 {
		writeOutputf(opts.Output, "  Deployers: %s\n", strings.Join(deployers, ", "))
	}
	if len(devDeployers) > 0 {
		writeOutputf(opts.Output, "  Dev deployers: %s\n", strings.Join(devDeployers, ", "))
	}
	writeOutputf(opts.Output, "  Services: %s\n", strings.Join(services, ", "))

	preBootstrapStackName := qualifier + "-pre-bootstrap"

	templatePath, cleanup, err := renderPreBootstrapTemplate(qualifier, services)
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
