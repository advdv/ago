package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func bootstrapCmd() *cli.Command {
	return &cli.Command{
		Name:   "bootstrap",
		Usage:  "Bootstrap CDK in the AWS account",
		Action: config.RunWithConfig(runBootstrap),
	}
}

type bootstrapOptions struct {
	Output io.Writer
}

func runBootstrap(ctx context.Context, _ *cli.Command, cfg config.Config) error {
	return doBootstrap(ctx, cfg, bootstrapOptions{
		Output: os.Stdout,
	})
}

func doBootstrap(ctx context.Context, cfg config.Config, opts bootstrapOptions) error {
	cdkDir := filepath.Join(cfg.ProjectDir, "infra", "cdk", "cdk")

	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.Output)
	cdkExec := cmdexec.New(cfg).InSubdir("infra/cdk/cdk").WithOutput(opts.Output, opts.Output)

	writeOutputf(opts.Output, "Reading CDK context...\n")
	cdkCtx, err := getCDKContext(cdkDir)
	if err != nil {
		return err
	}

	profile, ok := cdkCtx["admin-profile"].(string)
	if !ok || profile == "" {
		return errors.New("admin-profile not found in cdk.json - was 'ago infra create-aws-account' run?")
	}

	prefix, err := detectPrefix(cdkCtx)
	if err != nil {
		return err
	}

	qualifier, ok := cdkCtx[prefix+"qualifier"].(string)
	if !ok || qualifier == "" {
		return errors.Errorf("qualifier not found at context key %q", prefix+"qualifier")
	}

	secondaryRegions := extractStringSlice(cdkCtx, prefix+"secondary-regions")
	deployers := extractStringSlice(cdkCtx, prefix+"deployers")
	devDeployers := extractStringSlice(cdkCtx, prefix+"dev-deployers")

	writeOutputf(opts.Output, "Verifying AWS access with profile %q...\n", profile)
	if err := verifyAWSAccess(ctx, exec, profile); err != nil {
		return err
	}

	services, err := ParseServicesFromContext(cdkCtx, prefix)
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

	err = deployPreBootstrapStack(ctx, exec, profile, preBootstrapStackName, templatePath, qualifier,
		secondaryRegions, deployers, devDeployers)
	if err != nil {
		return err
	}

	executionPolicyArn, err := getStackOutput(ctx, exec, profile, preBootstrapStackName, "ExecutionPolicyArn")
	if err != nil {
		return err
	}

	permissionsBoundaryName, err := getStackOutput(ctx, exec, profile, preBootstrapStackName, "PermissionsBoundaryName")
	if err != nil {
		return err
	}

	boundaryConfig, ok := cdkCtx["@aws-cdk/core:permissionsBoundary"].(map[string]any)
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
	err = runCDKBootstrap(ctx, cdkExec, profile, qualifier, executionPolicyArn, permissionsBoundaryName)
	if err != nil {
		return err
	}

	writeOutputf(opts.Output, "Syncing deployer credentials...\n")
	if err := syncDeployerCredentials(ctx, exec, opts.Output, profile, qualifier, deployers, devDeployers); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Bootstrap complete!\n")
	return nil
}

func verifyAWSAccess(ctx context.Context, exec cmdexec.Executor, profile string) error {
	return exec.Mise(ctx, "aws", "sts", "get-caller-identity", "--profile", profile)
}

func deployPreBootstrapStack(
	ctx context.Context, exec cmdexec.Executor,
	profile, stackName, templatePath, qualifier string,
	secondaryRegions, deployers, devDeployers []string,
) error {
	secondaryRegionsParam := strings.Join(secondaryRegions, ",")
	deployersParam := strings.Join(deployers, ",")
	devDeployersParam := strings.Join(devDeployers, ",")

	return exec.Mise(ctx, "aws", "cloudformation", "deploy",
		"--stack-name", stackName,
		"--template-file", templatePath,
		"--parameter-overrides",
		"Qualifier="+qualifier,
		"SecondaryRegions="+secondaryRegionsParam,
		"Deployers="+deployersParam,
		"DevDeployers="+devDeployersParam,
		"--capabilities", "CAPABILITY_NAMED_IAM",
		"--no-fail-on-empty-changeset",
		"--profile", profile,
	)
}

func getStackOutput(ctx context.Context, exec cmdexec.Executor, profile, stackName, outputKey string) (string, error) {
	output, err := exec.MiseOutput(ctx, "aws", "cloudformation", "describe-stacks",
		"--stack-name", stackName,
		"--query", "Stacks[0].Outputs",
		"--output", "json",
		"--profile", profile,
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

func runCDKBootstrap(
	ctx context.Context, exec cmdexec.Executor,
	profile, qualifier, executionPolicyArn, permissionsBoundaryName string,
) error {
	toolkitStackName := qualifier + "Bootstrap"

	return exec.Mise(ctx, "cdk", "bootstrap",
		"--profile", profile,
		"--qualifier", qualifier,
		"--toolkit-stack-name", toolkitStackName,
		"--cloudformation-execution-policies", executionPolicyArn,
		"--custom-permissions-boundary", permissionsBoundaryName,
	)
}

func syncDeployerCredentials(
	ctx context.Context, exec cmdexec.Executor, output io.Writer,
	profile, qualifier string, deployers, devDeployers []string,
) error {
	existingProfiles, err := listDeployerProfiles(qualifier)
	if err != nil {
		writeOutputf(output, "  Warning: could not list existing profiles: %v\n", err)
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
			writeOutputf(output, "  Removing profile %q...\n", existingProfile)
			if err := removeAWSProfile(existingProfile); err != nil {
				writeOutputf(output, "    Warning: failed to remove profile: %v\n", err)
			}
		}
	}

	for profileName, info := range expectedProfiles {
		credentialsJSON, err := getSecretValue(ctx, exec, profile, info.secretPath)
		if err != nil {
			writeOutputf(output, "  Warning: could not fetch credentials for %s: %v\n", info.username, err)
			continue
		}

		var credentials struct {
			AccessKeyID     string `json:"aws_access_key_id"`
			SecretAccessKey string `json:"aws_secret_access_key"`
		}
		if err := json.Unmarshal([]byte(credentialsJSON), &credentials); err != nil {
			writeOutputf(output, "  Warning: could not parse credentials for %s: %v\n", info.username, err)
			continue
		}

		writeOutputf(output, "  Configuring profile %q for user %s...\n", profileName, info.username)
		err = writeDeployerProfile(ctx, exec, profileName, credentials.AccessKeyID, credentials.SecretAccessKey)
		if err != nil {
			writeOutputf(output, "    Warning: failed to write profile: %v\n", err)
		}
	}

	return nil
}

func listDeployerProfiles(qualifier string) ([]string, error) {
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

func removeAWSProfile(profileName string) error {
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
	ctx context.Context, exec cmdexec.Executor,
	profileName, accessKeyID, secretAccessKey string,
) error {
	settings := []struct{ key, value string }{
		{"aws_access_key_id", accessKeyID},
		{"aws_secret_access_key", secretAccessKey},
		{"region", "eu-central-1"},
		{"cli_pager", ""},
	}

	for _, s := range settings {
		if err := exec.Mise(ctx, "aws", "configure", "set", s.key, s.value, "--profile", profileName); err != nil {
			return errors.Wrapf(err, "failed to set %s for profile %s", s.key, profileName)
		}
	}

	return nil
}

func getSecretValue(ctx context.Context, exec cmdexec.Executor, profile, secretName string) (string, error) {
	return exec.MiseOutput(ctx, "aws", "secretsmanager", "get-secret-value",
		"--secret-id", secretName,
		"--query", "SecretString",
		"--output", "text",
		"--profile", profile,
	)
}
