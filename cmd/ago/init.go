package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/advdv/ago/cmd/ago/internal/initwizard"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

var miseTomlTemplate = template.Must(template.New("mise.toml").Parse(`[tools]
go = "{{.GoVersion}}"
node = "{{.NodeVersion}}"
"npm:aws-cdk" = "{{.AwsCdkVersion}}"
aws-cli = "{{.AwsCliVersion}}"
amp = "{{.AmpVersion}}"
granted = "{{.GrantedVersion}}"
golangci-lint = "{{.GolangciLintVersion}}"
shellcheck = "{{.ShellcheckVersion}}"
shfmt = "{{.ShfmtVersion}}"
ko = "{{.KoVersion}}"
"github:advdv/ago" = "{{.AgoVersion}}"
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
		cdk.NewShared,
		cdk.NewDeployment,
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

func NewDeployment(stack awscdk.Stack, shared *Shared, deploymentIdent string) {
}
`))

var tfGitignoreTemplate = template.Must(template.New(".gitignore").Parse(`# Local .terraform directories
**/.terraform/*

# .tfstate files
*.tfstate
*.tfstate.*

# Crash log files
crash.log
crash.*.log

# Exclude all .tfvars files, which are likely to contain sensitive data
*.tfvars
*.tfvars.json

# Ignore override files
override.tf
override.tf.json
*_override.tf
*_override.tf.json

# Ignore CLI configuration files
.terraformrc
terraform.rc

# Ignore lock info files
.terraform.tfstate.lock.info
`))

var tfMainTemplate = template.Must(template.New("main.tf").Parse(`terraform {
  cloud {
    organization = "{{.TerraformCloudOrg}}"
    workspaces {
      name = "{{.ProjectIdent}}"
    }
  }
}

# Add your providers and resources below
`))

var backendGoModTemplate = template.Must(template.New("go.mod").Parse(`module {{.ModuleName}}/backend

go {{.GoVersion}}
`))

var backendGitignoreTemplate = template.Must(template.New(".gitignore").Parse(`# Compiled binaries
*.exe
*.dll
*.so
*.dylib
*.test

# Coverage
*.out
coverage.*
*.coverprofile
profile.cov

# Go workspace
go.work
go.work.sum

# Environment
.env

# Editor/IDE
.idea
.DS_Store
`))

var backendKoYamlTemplate = template.Must(template.New(".ko.yaml").Parse(`defaultBaseImage: cgr.dev/chainguard/static:latest
`))

var backendCoreAPIMainTemplate = template.Must(template.New("main.go").Parse(`package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, World!")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	log.Printf("Starting server on :%s", port)

	//nolint:gosec // G114: timeouts configured at infrastructure level
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
`))

var golangciLintTemplate = template.Must(template.New(".golangci.yml").Parse(`version: "2"
linters:
  default: all
  disable:
    - canonicalheader
    - containedctx
    - dogsled
    - dupl
    - dupword
    - err113
    - exhaustruct
    - funlen
    - gochecknoglobals
    - gochecknoinits
    - goconst
    - gomoddirectives
    - ireturn
    - maintidx
    - mnd
    - nlreturn
    - nonamedreturns
    - perfsprint
    - thelper
    - unparam
    - wrapcheck
    - wsl_v5
    - wsl
    - noinlineerr
    - embeddedstructfieldcheck
    - tagalign
  settings:
    forbidigo:
      analyze-types: true
      forbid:
        - pattern: ^print(ln)?$
        - pattern: ^fmt\.Print.*$
          msg: Do not commit print statements.
        - pattern: ^fmt\.Errorf.*$
          msg: use 'github.com/cockroachdb/errors' for errors
    tagliatelle:
      case:
        rules:
          json: snake
    cyclop:
      max-complexity: 30
    depguard:
      rules:
        force_cockroachdb_errors:
          list-mode: lax
          files:
            - $all
          deny:
            - pkg: errors
              desc: use 'github.com/cockroachdb/errors' instead
    revive:
      rules:
        - name: package-comments
          disabled: true
    funlen:
      lines: 200
    nlreturn:
      block-size: 3
    staticcheck:
      checks:
        - all
        - "-ST1000"
        - "-QF1008"
    varnamelen:
      max-distance: 15
      ignore-names:
        - id
        - err
        - db
        - tx
        - w
        - r
        - ok
        - op # operation
        - lc # lifecycle
        - pk # partition key
        - sk # sort key
  exclusions:
    generated: lax
    presets:
      - common-false-positives
      - legacy
      - std-error-handling
    rules:
      - linters:
          - interfacebloat
        path: internal/rpc/rpc.go
      - linters:
          - paralleltest
        path: testing/e2e/
      - linters:
          - dupl
          - err113
          - errcheck
          - forcetypeassert
          - gochecknoglobals
          - gocyclo
          - gosec
          - lll
          - nilnil
          - nlreturn
          - perfsprint
          - revive
          - varnamelen
          - wrapcheck
          - gocognit
        path: _test\.go
      - path: (.+)\.go$
        text: ST1003
    paths:
      - node_modules
      - third_party$
      - builtin$
      - examples$
formatters:
  enable:
    - gofmt
    - gofumpt
    - goimports
  exclusions:
    generated: lax
    paths:
      - node_modules
      - third_party$
      - builtin$
      - examples$
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
	Services         []string
}

type TFConfig struct {
	TerraformCloudOrg string
	ProjectIdent      string
}

type BackendConfig struct {
	ModuleName string
	GoVersion  string
}

func DefaultCDKConfigFromDir(dir string) CDKConfig {
	name := filepath.Base(dir)
	return CDKConfig{
		Prefix:           name + "-",
		Qualifier:        name,
		PrimaryRegion:    "eu-central-1",
		SecondaryRegions: []string{"eu-north-1"},
		BaseDomainName:   "example.com",
		Deployments:      []string{"Prod", "Stag", "Dev1", "Dev2", "Dev3"},
		EmailPattern:     "admin+{project}@example.com",
		Services:         DefaultServices(),
	}
}

func DefaultBackendConfigFromDir(dir string) BackendConfig {
	name := filepath.Base(dir)
	return BackendConfig{
		ModuleName: "github.com/example/" + name,
		GoVersion:  "1.25",
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
	GoVersion           string
	NodeVersion         string
	AwsCdkVersion       string
	AwsCliVersion       string
	AmpVersion          string
	GrantedVersion      string
	GolangciLintVersion string
	ShellcheckVersion   string
	ShfmtVersion        string
	KoVersion           string
	AgoVersion          string
}

func DefaultMiseConfig() MiseConfig {
	return MiseConfig{
		GoVersion:           "latest",
		NodeVersion:         "22",
		AwsCdkVersion:       "latest",
		AwsCliVersion:       "latest",
		AmpVersion:          "latest",
		GrantedVersion:      "latest",
		GolangciLintVersion: "latest",
		ShellcheckVersion:   "latest",
		ShfmtVersion:        "latest",
		KoVersion:           "latest",
		AgoVersion:          "latest",
	}
}

func initCmd() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "Initialize a new ago project",
		ArgsUsage: "[directory]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "yes",
				Aliases: []string{"y"},
				Usage:   "Accept all defaults without prompting",
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

	defaultIdent := filepath.Base(dir)

	var result initwizard.Result
	if cmd.Bool("yes") {
		result = initwizard.DefaultResult(defaultIdent)
	} else {
		wizard := initwizard.New(initwizard.NewFormBuilder(), initwizard.NewInteractiveRunner())
		var err error
		result, err = wizard.Run(defaultIdent)
		if err != nil {
			return errors.Wrap(err, "wizard failed")
		}
	}

	cdkConfig := DefaultCDKConfigFromDir(dir)
	cdkConfig.Prefix = result.ProjectIdent + "-"
	cdkConfig.Qualifier = result.ProjectIdent
	cdkConfig.PrimaryRegion = result.PrimaryRegion
	cdkConfig.SecondaryRegions = result.SecondaryRegions

	tfConfig := TFConfig{
		TerraformCloudOrg: result.TerraformCloudOrg,
		ProjectIdent:      result.ProjectIdent,
	}

	backendConfig := DefaultBackendConfigFromDir(dir)

	return doInit(ctx, InitOptions{
		Dir:               dir,
		MiseConfig:        DefaultMiseConfig(),
		CDKConfig:         cdkConfig,
		TFConfig:          tfConfig,
		BackendConfig:     backendConfig,
		RunInstall:        true,
		ManagementProfile: result.ManagementProfile,
		Region:            result.PrimaryRegion,
		InitialDeployer:   result.InitialDeployer,
	})
}

type InitOptions struct {
	Dir                 string
	MiseConfig          MiseConfig
	CDKConfig           CDKConfig
	TFConfig            TFConfig
	BackendConfig       BackendConfig
	RunInstall          bool
	ManagementProfile   string
	Region              string
	InitialDeployer     string
	SkipAccountCreation bool
	SkipCDKVerify       bool
}

func doInit(ctx context.Context, opts InitOptions) error {
	exec := cmdexec.NewWithDir(opts.Dir).WithOutput(os.Stdout, os.Stderr)

	if err := checkMiseInstalled(ctx); err != nil {
		return err
	}

	if err := ensureEmptyDir(opts.Dir); err != nil {
		return err
	}

	if err := exec.Run(ctx, "git", "init"); err != nil {
		return errors.Wrap(err, "git init failed")
	}

	if err := config.WriteToFile(opts.Dir, config.Default(), config.NewWriter()); err != nil {
		return err
	}

	if err := writeMiseToml(opts.Dir, opts.MiseConfig); err != nil {
		return err
	}

	if err := exec.Run(ctx, "mise", "trust"); err != nil {
		return errors.Wrap(err, "mise trust failed")
	}

	if err := exec.Run(ctx, "mise", "upgrade"); err != nil {
		return errors.Wrap(err, "mise upgrade failed")
	}

	if opts.RunInstall {
		if err := exec.Run(ctx, "mise", "install"); err != nil {
			return errors.Wrap(err, "mise install failed")
		}
	}

	if err := installAmpSkills(ctx, exec); err != nil {
		return err
	}

	if err := setupCDKProject(ctx, exec, opts.Dir); err != nil {
		return err
	}

	if err := configureCDKProject(ctx, exec, opts.Dir, opts.CDKConfig); err != nil {
		return err
	}

	if err := setupTFProject(opts.Dir, opts.TFConfig); err != nil {
		return err
	}

	if err := setupBackendProject(ctx, exec, opts.Dir, opts.BackendConfig); err != nil {
		return err
	}

	if opts.InitialDeployer != "" {
		if err := exec.Mise(ctx, "ago", "infra", "cdk", "add-deployer", opts.InitialDeployer); err != nil {
			return errors.Wrap(err, "failed to add initial deployer")
		}
	}

	if !opts.SkipAccountCreation {
		cfg := config.Config{ProjectDir: opts.Dir}
		projectName := filepath.Base(opts.Dir)
		if err := doCreateProjectAccount(ctx, cfg, createAccountOptions{
			ProjectName:       projectName,
			ManagementProfile: opts.ManagementProfile,
			Region:            opts.Region,
			WriteProfile:      true,
			EmailPattern:      opts.CDKConfig.EmailPattern,
			Output:            os.Stdout,
		}); err != nil {
			return err
		}
	}

	if !opts.SkipCDKVerify {
		if err := verifyCDKSetup(ctx, exec, opts.CDKConfig); err != nil {
			return err
		}
	}

	if err := exec.Mise(ctx, "ago", "dev", "fmt"); err != nil {
		return errors.Wrap(err, "failed to run ago dev fmt")
	}

	return nil
}

func verifyCDKSetup(ctx context.Context, exec cmdexec.Executor, cfg CDKConfig) error {
	cdkExec := exec.InSubdir("infra/cdk/cdk")
	deployerGroupsCtx := cfg.Prefix + "deployer-groups=" + cfg.Qualifier + "-deployers"

	return cdkExec.Mise(ctx, "cdk", "ls", "--context", deployerGroupsCtx)
}

func checkMiseInstalled(ctx context.Context) error {
	exec := cmdexec.NewWithDir(".")
	if _, err := exec.Output(ctx, "mise", "--version"); err != nil {
		return errors.New("mise is not installed or not in PATH")
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

func configureCDKProject(ctx context.Context, exec cmdexec.Executor, dir string, cfg CDKConfig) error {
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

	if err := writeGolangciLintConfig(infraDir); err != nil {
		return err
	}

	infraExec := exec.InSubdir("infra")
	if err := infraExec.Run(ctx, "go", "get", "github.com/advdv/ago/agcdkutil"); err != nil {
		return errors.Wrap(err, "failed to add agcdkutil dependency")
	}

	if err := infraExec.Run(ctx, "go", "mod", "tidy"); err != nil {
		return errors.Wrap(err, "go mod tidy failed")
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

func writeCDKContextJSON(cdkDir string, cfg CDKConfig) error {
	context := map[string]any{
		cfg.Prefix + "qualifier":         cfg.Qualifier,
		cfg.Prefix + "primary-region":    cfg.PrimaryRegion,
		cfg.Prefix + "secondary-regions": cfg.SecondaryRegions,
		cfg.Prefix + "deployments":       cfg.Deployments,
		cfg.Prefix + "base-domain-name":  cfg.BaseDomainName,
		cfg.Prefix + "services":          cfg.Services,
		"@aws-cdk/core:permissionsBoundary": map[string]string{
			"name": cfg.Qualifier + "-permissions-boundary",
		},
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

func writeGolangciLintConfig(infraDir string) error {
	var buf bytes.Buffer
	if err := golangciLintTemplate.Execute(&buf, nil); err != nil {
		return errors.Wrap(err, "failed to execute .golangci.yml template")
	}

	path := filepath.Join(infraDir, ".golangci.yml")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil { //nolint:gosec // config file needs to be readable
		return errors.Wrap(err, "failed to write .golangci.yml")
	}

	return nil
}

func setupCDKProject(ctx context.Context, exec cmdexec.Executor, dir string) error {
	infraDir := filepath.Join(dir, "infra")
	cdkDir := filepath.Join(infraDir, "cdk", "cdk")

	if err := os.MkdirAll(cdkDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create CDK directory")
	}

	cdkExec := exec.InSubdir("infra/cdk/cdk")
	if err := cdkExec.Mise(ctx, "cdk", "init", "app", "--language=go", "--generate-only"); err != nil {
		return errors.Wrap(err, "cdk init failed")
	}

	for _, filename := range []string{"go.mod", "go.sum"} {
		src := filepath.Join(cdkDir, filename)
		dst := filepath.Join(infraDir, filename)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return errors.Wrapf(err, "failed to move %s to infra directory", filename)
			}
		}
	}

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

	readmePath := filepath.Join(cdkDir, "README.md")
	if err := os.Remove(readmePath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove README.md")
	}

	return nil
}

func setupTFProject(dir string, cfg TFConfig) error {
	tfDir := filepath.Join(dir, "infra", "tf")

	if err := os.MkdirAll(tfDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create tf directory")
	}

	var gitignoreBuf bytes.Buffer
	if err := tfGitignoreTemplate.Execute(&gitignoreBuf, nil); err != nil {
		return errors.Wrap(err, "failed to execute tf .gitignore template")
	}

	gitignorePath := filepath.Join(tfDir, ".gitignore")
	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(gitignorePath, gitignoreBuf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write tf .gitignore")
	}

	var mainBuf bytes.Buffer
	if err := tfMainTemplate.Execute(&mainBuf, cfg); err != nil {
		return errors.Wrap(err, "failed to execute tf main.tf template")
	}

	mainPath := filepath.Join(tfDir, "main.tf")
	//nolint:gosec // source file needs to be readable
	if err := os.WriteFile(mainPath, mainBuf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write tf main.tf")
	}

	return nil
}

func setupBackendProject(ctx context.Context, exec cmdexec.Executor, dir string, cfg BackendConfig) error {
	backendDir := filepath.Join(dir, "backend")

	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create backend directory")
	}

	var goModBuf bytes.Buffer
	if err := backendGoModTemplate.Execute(&goModBuf, cfg); err != nil {
		return errors.Wrap(err, "failed to execute backend go.mod template")
	}

	goModPath := filepath.Join(backendDir, "go.mod")
	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(goModPath, goModBuf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write backend go.mod")
	}

	var gitignoreBuf bytes.Buffer
	if err := backendGitignoreTemplate.Execute(&gitignoreBuf, nil); err != nil {
		return errors.Wrap(err, "failed to execute backend .gitignore template")
	}

	gitignorePath := filepath.Join(backendDir, ".gitignore")
	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(gitignorePath, gitignoreBuf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write backend .gitignore")
	}

	if err := writeGolangciLintConfig(backendDir); err != nil {
		return errors.Wrap(err, "failed to write backend .golangci.yml")
	}

	var koYamlBuf bytes.Buffer
	if err := backendKoYamlTemplate.Execute(&koYamlBuf, nil); err != nil {
		return errors.Wrap(err, "failed to execute backend .ko.yaml template")
	}

	koYamlPath := filepath.Join(backendDir, ".ko.yaml")
	//nolint:gosec // config file needs to be readable
	if err := os.WriteFile(koYamlPath, koYamlBuf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write backend .ko.yaml")
	}

	coreAPIDir := filepath.Join(backendDir, "cmd", "coreapi")
	if err := os.MkdirAll(coreAPIDir, 0o755); err != nil {
		return errors.Wrap(err, "failed to create backend cmd/coreapi directory")
	}

	var mainBuf bytes.Buffer
	if err := backendCoreAPIMainTemplate.Execute(&mainBuf, nil); err != nil {
		return errors.Wrap(err, "failed to execute backend main.go template")
	}

	mainPath := filepath.Join(coreAPIDir, "main.go")
	//nolint:gosec // source file needs to be readable
	if err := os.WriteFile(mainPath, mainBuf.Bytes(), 0o644); err != nil {
		return errors.Wrap(err, "failed to write backend main.go")
	}

	backendExec := exec.InSubdir("backend")
	if err := backendExec.Run(ctx, "go", "mod", "tidy"); err != nil {
		return errors.Wrap(err, "backend go mod tidy failed")
	}

	return nil
}

var defaultSkills = []string{
	"solid-principles",
}

func installAmpSkills(ctx context.Context, exec cmdexec.Executor) error {
	for _, skill := range defaultSkills {
		skillURL := "https://github.com/advdv/ago/tree/main/.agents/skills/" + skill
		if err := exec.Run(ctx, "amp", "skill", "add", skillURL); err != nil {
			return errors.Wrapf(err, "failed to install amp skill %q", skill)
		}
	}
	return nil
}
