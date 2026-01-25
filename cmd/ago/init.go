package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

var miseTomlTemplate = template.Must(template.New("mise.toml").Parse(`[tools]
go = "{{.GoVersion}}"
node = "{{.NodeVersion}}"
"npm:aws-cdk" = "{{.AwsCdkVersion}}"
aws-cli = "{{.AwsCliVersion}}"
amp = "{{.AmpVersion}}"
`))

var cdkMainTemplate = template.Must(template.New("cdk.go").Parse(`package main

import (
	"{{.ModuleName}}/cdk"

	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()
	app := awscdk.NewApp(nil)

	agcdkutil.SetupApp(app, agcdkutil.AppConfig{
		Prefix:                "{{.Prefix}}",
		DeployersGroup:        "{{.Qualifier}}-deployers",
		RestrictedDeployments: []string{"Stag", "Prod"},
	},
		func(stack awscdk.Stack) *cdk.Shared { return cdk.NewShared(stack) },
		func(stack awscdk.Stack, shared *cdk.Shared, deploymentIdent string) {
			cdk.NewDeployment(stack, shared, deploymentIdent)
		},
	)

	app.Synth(nil)
}
`))

var cdkSharedTemplate = template.Must(template.New("shared.go").Parse(`package cdk

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
)

type Shared struct {
}

func NewShared(stack awscdk.Stack) *Shared {
	return &Shared{}
}
`))

var cdkDeploymentTemplate = template.Must(template.New("deployment.go").Parse(`package cdk

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
)

type Deployment struct {
}

func NewDeployment(stack awscdk.Stack, shared *Shared, deploymentIdent string) *Deployment {
	return &Deployment{}
}
`))

type CDKConfig struct {
	Prefix           string
	Qualifier        string
	PrimaryRegion    string
	SecondaryRegions []string
	BaseDomainName   string
	Deployments      []string
	ModuleName       string
	EmailPattern     string
}

var accountStackTemplate = template.Must(template.New("account-stack.yaml").Parse(
	`AWSTemplateFormatVersion: '2010-09-09'
Description: Creates an AWS Organizations account for project {{.Qualifier}}

Resources:
  ProjectAccount:
    Type: AWS::Organizations::Account
    Properties:
      AccountName: {{.Qualifier}}
      Email: {{.Email}}
      RoleName: OrganizationAccountAccessRole

Outputs:
  AccountId:
    Description: The AWS Account ID of the created account
    Value: !GetAtt ProjectAccount.AccountId
    Export:
      Name: {{.Qualifier}}-AccountId
  AccountArn:
    Description: The ARN of the created account
    Value: !GetAtt ProjectAccount.Arn
    Export:
      Name: {{.Qualifier}}-AccountArn
`))

func DefaultCDKConfigFromDir(dir string) CDKConfig {
	name := filepath.Base(dir)
	return CDKConfig{
		Prefix:           name + "-",
		Qualifier:        name,
		PrimaryRegion:    "eu-central-1",
		SecondaryRegions: []string{},
		BaseDomainName:   "example.com",
		Deployments:      []string{"Prod", "Stag", "Dev1", "Dev2", "Dev3"},
		EmailPattern:     "admin+{project}@example.com",
	}
}

func readModuleName(infraDir string) (string, error) {
	goModPath := filepath.Join(infraDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read go.mod")
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if moduleName, ok := strings.CutPrefix(line, "module "); ok {
			return moduleName, nil
		}
	}

	return "", errors.New("module name not found in go.mod")
}

type MiseConfig struct {
	GoVersion     string
	NodeVersion   string
	AwsCdkVersion string
	AwsCliVersion string
	AmpVersion    string
}

func DefaultMiseConfig() MiseConfig {
	return MiseConfig{
		GoVersion:     "latest",
		NodeVersion:   "22",
		AwsCdkVersion: "latest",
		AwsCliVersion: "latest",
		AmpVersion:    "latest",
	}
}

func initCmd() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "Initialize a new ago project",
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "management-profile",
				Usage:    "AWS profile for the management account (used to create project account)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: "AWS region for the project account",
				Value: "eu-central-1",
			},
		},
		Action: runInit,
	}
}

func runInit(ctx context.Context, cmd *cli.Command) error {
	dir := cmd.Args().First()
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "failed to get current working directory")
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return errors.Wrap(err, "failed to get absolute path")
	}
	dir = absDir

	return doInit(ctx, InitOptions{
		Dir:               dir,
		MiseConfig:        DefaultMiseConfig(),
		CDKConfig:         DefaultCDKConfigFromDir(dir),
		RunInstall:        true,
		ManagementProfile: cmd.String("management-profile"),
		Region:            cmd.String("region"),
	})
}

type InitOptions struct {
	Dir               string
	MiseConfig        MiseConfig
	CDKConfig         CDKConfig
	RunInstall        bool
	ManagementProfile string
	Region            string
}

func doInit(ctx context.Context, opts InitOptions) error {
	if err := checkMiseInstalled(ctx); err != nil {
		return err
	}

	if err := ensureEmptyDir(opts.Dir); err != nil {
		return err
	}

	if err := initGitRepo(ctx, opts.Dir); err != nil {
		return err
	}

	if err := writeMiseToml(opts.Dir, opts.MiseConfig); err != nil {
		return err
	}

	if err := trustMiseConfig(ctx, opts.Dir); err != nil {
		return err
	}

	if opts.RunInstall {
		if err := runMiseInstall(ctx, opts.Dir); err != nil {
			return err
		}
	}

	if err := installAgoCLI(ctx, opts.Dir); err != nil {
		return err
	}

	if err := installAmpSkills(ctx, opts.Dir, defaultSkills); err != nil {
		return err
	}

	if err := setupCDKProject(ctx, opts.Dir); err != nil {
		return err
	}

	if err := configureCDKProject(ctx, opts.Dir, opts.CDKConfig); err != nil {
		return err
	}

	if err := writeAccountStackTemplate(opts.Dir, opts.CDKConfig); err != nil {
		return err
	}

	projectName := filepath.Base(opts.Dir)
	if err := doCreateProjectAccount(ctx, createAccountOptions{
		ProjectDir:        opts.Dir,
		ProjectName:       projectName,
		ManagementProfile: opts.ManagementProfile,
		Region:            opts.Region,
		WriteProfile:      true,
		Output:            os.Stdout,
	}); err != nil {
		return err
	}

	if err := verifyCDKSetup(ctx, opts.Dir); err != nil {
		return err
	}

	return nil
}

func verifyCDKSetup(ctx context.Context, dir string) error {
	cdkDir := filepath.Join(dir, "infra", "cdk", "cdk")
	cmd := exec.CommandContext(ctx, "mise", "exec", "--", "cdk", "ls")
	cmd.Dir = cdkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "cdk ls failed - CDK setup may be incomplete")
	}
	return nil
}

func checkMiseInstalled(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "mise", "--version")
	if err := cmd.Run(); err != nil {
		return errors.New("mise is not installed or not in PATH")
	}
	return nil
}

func initGitRepo(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "git init failed")
	}
	return nil
}

func ensureEmptyDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return errors.Newf("%q is not a directory", dir)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return errors.Wrap(err, "failed to read directory")
		}
		if len(entries) > 0 {
			return errors.Newf("directory %q is not empty", dir)
		}

		return nil
	} else if !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to check directory")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create directory")
	}

	return nil
}

func writeMiseToml(dir string, cfg MiseConfig) error {
	var buf bytes.Buffer
	if err := miseTomlTemplate.Execute(&buf, cfg); err != nil {
		return errors.Wrap(err, "failed to execute mise.toml template")
	}

	path := filepath.Join(dir, "mise.toml")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil { //nolint:gosec // config file needs to be readable
		return errors.Wrap(err, "failed to write mise.toml")
	}

	return nil
}

func trustMiseConfig(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "mise", "trust")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "mise trust failed")
	}
	return nil
}

func runMiseInstall(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "mise", "install")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "mise install failed")
	}
	return nil
}

func configureCDKProject(ctx context.Context, dir string, cfg CDKConfig) error {
	infraDir := filepath.Join(dir, "infra")
	cdkPkgDir := filepath.Join(infraDir, "cdk")
	cdkDir := filepath.Join(cdkPkgDir, "cdk")

	moduleName, err := readModuleName(infraDir)
	if err != nil {
		return err
	}
	cfg.ModuleName = moduleName

	if err := writeCDKGoFiles(cdkPkgDir, cdkDir, cfg); err != nil {
		return err
	}

	if err := writeCDKContextJSON(cdkDir, cfg); err != nil {
		return err
	}

	if err := addAgcdkutilDependency(ctx, infraDir); err != nil {
		return err
	}

	return nil
}

func writeCDKGoFiles(cdkPkgDir, cdkDir string, cfg CDKConfig) error {
	templates := map[string]struct {
		tmpl *template.Template
		dir  string
	}{
		"cdk.go":        {cdkMainTemplate, cdkDir},
		"shared.go":     {cdkSharedTemplate, cdkPkgDir},
		"deployment.go": {cdkDeploymentTemplate, cdkPkgDir},
	}

	for filename, t := range templates {
		var buf bytes.Buffer
		if err := t.tmpl.Execute(&buf, cfg); err != nil {
			return errors.Wrapf(err, "failed to execute %s template", filename)
		}

		path := filepath.Join(t.dir, filename)
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil { //nolint:gosec // source file needs to be readable
			return errors.Wrapf(err, "failed to write %s", filename)
		}
	}

	return nil
}

func writeAccountStackTemplate(dir string, cfg CDKConfig) error {
	cfnDir := filepath.Join(dir, "infra", "cfn")
	if err := os.MkdirAll(cfnDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create infra/cfn directory")
	}

	email := strings.ReplaceAll(cfg.EmailPattern, "{project}", cfg.Qualifier)

	data := struct {
		Qualifier string
		Email     string
	}{
		Qualifier: cfg.Qualifier,
		Email:     email,
	}

	var buf bytes.Buffer
	if err := accountStackTemplate.Execute(&buf, data); err != nil {
		return errors.Wrap(err, "failed to execute account stack template")
	}

	templatePath := filepath.Join(cfnDir, "account-stack.yaml")
	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(templatePath, buf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write account-stack.yaml")
	}

	return nil
}

func writeCDKContextJSON(cdkDir string, cfg CDKConfig) error {
	context := map[string]any{
		cfg.Prefix + "qualifier":         cfg.Qualifier,
		cfg.Prefix + "primary-region":    cfg.PrimaryRegion,
		cfg.Prefix + "secondary-regions": cfg.SecondaryRegions,
		cfg.Prefix + "deployments":       cfg.Deployments,
		cfg.Prefix + "base-domain-name":  cfg.BaseDomainName,
	}

	output, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cdk.context.json")
	}

	contextPath := filepath.Join(cdkDir, "cdk.context.json")
	if err := os.WriteFile(contextPath, output, 0o644); err != nil { //nolint:gosec // config file needs to be readable
		return errors.Wrap(err, "failed to write cdk.context.json")
	}

	return nil
}

func addAgcdkutilDependency(ctx context.Context, infraDir string) error {
	cmd := exec.CommandContext(ctx, "go", "get", "github.com/advdv/ago/agcdkutil")
	cmd.Dir = infraDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to add agcdkutil dependency")
	}

	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = infraDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return errors.Wrap(err, "go mod tidy failed")
	}

	return nil
}

func setupCDKProject(ctx context.Context, dir string) error {
	infraDir := filepath.Join(dir, "infra")
	cdkDir := filepath.Join(infraDir, "cdk", "cdk")

	if err := os.MkdirAll(cdkDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create CDK directory")
	}

	// Initialize CDK Go project
	// We use "mise exec" to run cdk from the project root where mise.toml is located,
	// so mise can provide the cdk binary from npm:aws-cdk.
	initCmd := exec.CommandContext(ctx, "mise", "exec", "--", "cdk", "init", "app", "--language=go", "--generate-only")
	// cdk init requires the target directory as current working directory
	initCmd.Dir = cdkDir
	// But we need mise from the parent, so we set MISE_PROJECT_DIR
	initCmd.Env = append(os.Environ(), "MISE_PROJECT_DIR="+dir)
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		return errors.Wrap(err, "cdk init failed")
	}

	// Move go.mod and go.sum from cdkDir to infraDir so ./infra is the Go module root
	for _, filename := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(cdkDir, filename)
		dst := filepath.Join(infraDir, filename)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return errors.Wrapf(err, "failed to move %s to infra directory", filename)
			}
		}
	}

	// Append "cdk" to .gitignore so compiled binary doesn't get committed
	gitignorePath := filepath.Join(cdkDir, ".gitignore")
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.Wrap(err, "failed to open .gitignore")
	}
	if _, err := f.WriteString("\ncdk\n"); err != nil {
		f.Close()
		return errors.Wrap(err, "failed to write to .gitignore")
	}
	f.Close()

	// Remove the generated test file
	entries, err := os.ReadDir(cdkDir)
	if err != nil {
		return errors.Wrap(err, "failed to read CDK directory")
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") {
			if err := os.Remove(filepath.Join(cdkDir, name)); err != nil {
				return errors.Wrapf(err, "failed to remove %s", name)
			}
		}
	}

	// Remove the generated README.md
	readmePath := filepath.Join(cdkDir, "README.md")
	if err := os.Remove(readmePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove README.md")
	}

	return nil
}

var defaultSkills = []string{
	"solid-principles",
	"setting-up-cdk-app",
}

func installAmpSkills(ctx context.Context, dir string, skills []string) error {
	for _, skill := range skills {
		skillURL := "https://github.com/advdv/ago/tree/main/.agents/skills/" + skill
		cmd := exec.CommandContext(ctx, "amp", "skill", "add", skillURL)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return errors.Wrapf(err, "failed to install amp skill %q", skill)
		}
	}
	return nil
}

// installAgoCLI installs the ago CLI tool using mise.
//
// This process requires special handling for three reasons:
//
//  1. GOPROXY=direct is required to bypass the Go module proxy cache.
//     The proxy can cache module versions for up to 24 hours, so without
//     this flag, users might not get the absolute latest commit.
//
//  2. We must first uninstall any existing version before installing.
//     Mise considers "@latest" as already installed if present, and won't
//     reinstall even if a newer commit exists. Uninstalling first forces
//     mise to fetch and install the current latest version.
//
//  3. GOFLAGS=-mod=mod prevents Go from using a parent go.mod file.
//     If the new project is created inside an existing Go module (e.g., ago/t1),
//     Go would otherwise try to resolve the package within that module context,
//     causing "invalid import path" errors.
func installAgoCLI(ctx context.Context, dir string) error {
	const agoPackage = "go:github.com/advdv/ago/cmd/ago@latest"

	env := append(os.Environ(), "GOPROXY=direct", "GOFLAGS=-mod=mod")

	// First uninstall any existing version. This is necessary because mise
	// won't reinstall if @latest is already present, even if there's a newer commit.
	// We ignore errors here since the package might not be installed yet.
	uninstallCmd := exec.CommandContext(ctx, "mise", "uninstall", agoPackage)
	uninstallCmd.Dir = dir
	uninstallCmd.Env = env
	_ = uninstallCmd.Run() // Ignore error - package might not exist

	// Install the latest version, bypassing the Go module proxy cache
	// to ensure we get the absolute latest commit.
	installCmd := exec.CommandContext(ctx, "mise", "use", agoPackage)
	installCmd.Dir = dir
	installCmd.Env = env
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install ago CLI")
	}

	return nil
}
