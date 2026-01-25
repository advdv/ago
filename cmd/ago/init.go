package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
		Action:    runInit,
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

	return doInit(ctx, InitOptions{
		Dir:        dir,
		MiseConfig: DefaultMiseConfig(),
		RunInstall: true,
	})
}

type InitOptions struct {
	Dir        string
	MiseConfig MiseConfig
	RunInstall bool
}

func doInit(ctx context.Context, opts InitOptions) error {
	if err := checkMiseInstalled(ctx); err != nil {
		return err
	}

	if err := ensureEmptyDir(opts.Dir); err != nil {
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

	return nil
}

func checkMiseInstalled(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "mise", "--version")
	if err := cmd.Run(); err != nil {
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
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
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
// This process requires special handling for two reasons:
//
//  1. GOPROXY=direct is required to bypass the Go module proxy cache.
//     The proxy can cache module versions for up to 24 hours, so without
//     this flag, users might not get the absolute latest commit.
//
//  2. We must first uninstall any existing version before installing.
//     Mise considers "@latest" as already installed if present, and won't
//     reinstall even if a newer commit exists. Uninstalling first forces
//     mise to fetch and install the current latest version.
func installAgoCLI(ctx context.Context, dir string) error {
	const agoPackage = "go:github.com/advdv/ago/cmd/ago@latest"

	// First uninstall any existing version. This is necessary because mise
	// won't reinstall if @latest is already present, even if there's a newer commit.
	// We ignore errors here since the package might not be installed yet.
	uninstallCmd := exec.CommandContext(ctx, "mise", "uninstall", agoPackage)
	uninstallCmd.Dir = dir
	uninstallCmd.Env = append(os.Environ(), "GOPROXY=direct")
	_ = uninstallCmd.Run() // Ignore error - package might not exist

	// Install the latest version, bypassing the Go module proxy cache
	// to ensure we get the absolute latest commit.
	installCmd := exec.CommandContext(ctx, "mise", "use", agoPackage)
	installCmd.Dir = dir
	installCmd.Env = append(os.Environ(), "GOPROXY=direct")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return errors.Wrap(err, "failed to install ago CLI")
	}

	return nil
}
