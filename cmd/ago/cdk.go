package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
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
			deployCmd(),
			diffCmd(),
			destroyCmd(),
		},
	}
}

func writeOutputf(w io.Writer, format string, args ...any) {
	if w != nil {
		_, _ = fmt.Fprintf(w, format, args...)
	}
}

func getCDKContext(cdkDir string) (map[string]any, error) {
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

var errAssumedRole = errors.New("using assumed role")

func isAssumedRoleARN(arn string) bool {
	return strings.Contains(arn, ":assumed-role/")
}

func getCallerUsername(
	ctx context.Context, exec cmdexec.Executor, qualifier string, cdkContext map[string]any,
) (string, error) {
	deployerProfile := findLocalDeployerProfile(ctx, exec, qualifier)
	if deployerProfile != "" {
		username, err := getUsernameFromProfile(ctx, exec, deployerProfile)
		if err == nil {
			return username, nil
		}
	}

	profile, ok := cdkContext["admin-profile"].(string)
	if !ok || profile == "" {
		return "", errors.New("admin-profile not found in cdk.json")
	}

	output, err := exec.MiseOutput(ctx, "aws", "sts", "get-caller-identity",
		"--profile", profile,
		"--query", "Arn",
		"--output", "text",
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to get caller identity")
	}

	if isAssumedRoleARN(output) {
		return "", errAssumedRole
	}

	parts := strings.Split(output, "/")
	if len(parts) < 2 {
		return "", errors.Errorf("unexpected ARN format: %s", output)
	}

	return parts[len(parts)-1], nil
}

func findLocalDeployerProfile(ctx context.Context, exec cmdexec.Executor, qualifier string) string {
	if qualifier == "" {
		return ""
	}

	output, err := exec.MiseOutput(ctx, "aws", "configure", "list-profiles")
	if err != nil {
		return ""
	}

	prefix := qualifier + "-"
	for profile := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(profile, prefix) && profile != qualifier+"-admin" {
			return profile
		}
	}

	return ""
}

func getUsernameFromProfile(ctx context.Context, exec cmdexec.Executor, profile string) (string, error) {
	output, err := exec.MiseOutput(ctx, "aws", "sts", "get-caller-identity",
		"--profile", profile,
		"--query", "Arn",
		"--output", "text",
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to get caller identity")
	}

	if isAssumedRoleARN(output) {
		return "", errAssumedRole
	}

	parts := strings.Split(output, "/")
	if len(parts) < 2 {
		return "", errors.Errorf("unexpected ARN format: %s", output)
	}

	return parts[len(parts)-1], nil
}

func formatDeploymentsList(deployments []string) string {
	if len(deployments) == 0 {
		return "(none)"
	}
	return strings.Join(deployments, ", ")
}

type cdkCommandOptions struct {
	Deployment string
	All        bool
	Hotswap    bool
	Output     io.Writer
}

func resolveDeploymentIdent(
	opts cdkCommandOptions,
	prefix string,
	cdkContext map[string]any,
	username string,
	usernameErr error,
) (string, error) {
	deployments := extractStringSlice(cdkContext, prefix+"deployments")

	if opts.Deployment != "" {
		if !slices.Contains(deployments, opts.Deployment) {
			return "", errors.Errorf("deployment %q not found\n\nAvailable deployments: %s",
				opts.Deployment, formatDeploymentsList(deployments))
		}
		return opts.Deployment, nil
	}

	if os.Getenv("CI") != "" {
		return "", errors.New("deployment identifier required in CI mode")
	}

	if errors.Is(usernameErr, errAssumedRole) {
		return "", errors.Errorf(`cannot auto-detect deployment: you're using an assumed role, not an IAM user

To deploy, either:
  - Specify a deployment explicitly: ago cdk deploy <deployment>
  - Add yourself as a deployer: ago cdk add-deployer <YourName>
    Then run: ago cdk bootstrap
    Then retry without arguments

Available deployments: %s`, formatDeploymentsList(deployments))
	}

	if usernameErr != nil {
		return "", errors.Wrap(usernameErr, "failed to detect username")
	}

	deployment := "Dev" + username

	if !slices.Contains(deployments, deployment) {
		return "", errors.Errorf(`deployment %q not found

Run 'ago cdk add-deployer %s' to add yourself, then 'ago cdk bootstrap'.

Available deployments: %s`, deployment, username, formatDeploymentsList(deployments))
	}

	return deployment, nil
}

func getUserGroups(ctx context.Context, exec cmdexec.Executor, profile, username string) ([]string, error) {
	output, err := exec.MiseOutput(ctx, "aws", "iam", "list-groups-for-user",
		"--user-name", username,
		"--profile", profile,
		"--query", "Groups[].GroupName",
		"--output", "json",
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list groups for user")
	}

	var groups []string
	if err := json.Unmarshal([]byte(output), &groups); err != nil {
		return nil, errors.Wrap(err, "failed to parse groups")
	}

	return groups, nil
}

func isFullDeployer(groups []string, qualifier string) bool {
	deployersGroup := qualifier + "-deployers"
	return slices.Contains(groups, deployersGroup)
}

func checkDeploymentPermission(deployment string, isFullDep bool) error {
	if (strings.HasPrefix(deployment, "Prod") || strings.HasPrefix(deployment, "Stag")) && !isFullDep {
		return errors.Errorf(
			"deployment %q requires full deployer permissions (member of deployers group)",
			deployment,
		)
	}
	return nil
}

func resolveProfile(
	ctx context.Context, exec cmdexec.Executor, cdkContext map[string]any, qualifier, username string,
) string {
	deployerProfile := qualifier + "-" + strings.ToLower(username)

	output, err := exec.MiseOutput(ctx, "aws", "configure", "list-profiles")
	if err == nil {
		profiles := strings.Split(output, "\n")
		if slices.Contains(profiles, deployerProfile) {
			return deployerProfile
		}
	}

	if adminProfile, ok := cdkContext["admin-profile"].(string); ok && adminProfile != "" {
		return adminProfile
	}

	return deployerProfile
}

func buildCDKArgs(profile, qualifier, prefix string, userGroups []string) []string {
	args := make([]string, 0, 10)
	args = append(args,
		"--profile", profile,
		"--qualifier", qualifier,
		"--toolkit-stack-name", qualifier+"Bootstrap",
	)

	if len(userGroups) > 0 {
		args = append(args, "-c", prefix+"deployer-groups="+strings.Join(userGroups, " "))
	}

	return args
}

func runCDKCommand(ctx context.Context, exec cmdexec.Executor, command string, args []string) error {
	fullArgs := append([]string{command}, args...)
	return exec.Mise(ctx, "cdk", fullArgs...)
}

// cdkContext holds common CDK context needed by most CDK commands.
type cdkContext struct {
	Exec       cmdexec.Executor
	CDKExec    cmdexec.Executor
	CDKDir     string
	CDKContext map[string]any
	Prefix     string
	Qualifier  string
}

func loadCDKContext(cfg config.Config) (*cdkContext, error) {
	cdkDir := filepath.Join(cfg.ProjectDir, "infra", "cdk", "cdk")

	cdkCtx, err := getCDKContext(cdkDir)
	if err != nil {
		return nil, err
	}

	prefix, err := detectPrefix(cdkCtx)
	if err != nil {
		return nil, err
	}

	qualifier, ok := cdkCtx[prefix+"qualifier"].(string)
	if !ok || qualifier == "" {
		return nil, errors.Errorf("qualifier not found at context key %q", prefix+"qualifier")
	}

	return &cdkContext{
		Exec:       cmdexec.New(cfg),
		CDKExec:    cmdexec.New(cfg).InSubdir("infra/cdk/cdk"),
		CDKDir:     cdkDir,
		CDKContext: cdkCtx,
		Prefix:     prefix,
		Qualifier:  qualifier,
	}, nil
}
