package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
)

func TestEnsureEmptyDir(t *testing.T) {
	t.Parallel()

	t.Run("creates new directory", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "newproject")

		err := ensureEmptyDir(targetDir)
		if err != nil {
			t.Fatalf("ensureEmptyDir failed: %v", err)
		}

		info, err := os.Stat(targetDir)
		if err != nil {
			t.Fatalf("directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory to be created")
		}
	})

	t.Run("succeeds if directory exists but is empty", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		err := ensureEmptyDir(tmpDir)
		if err != nil {
			t.Fatalf("ensureEmptyDir failed on empty existing directory: %v", err)
		}
	})

	t.Run("fails if directory is not empty", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("content"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := ensureEmptyDir(tmpDir)
		if err == nil {
			t.Fatal("expected error when directory is not empty")
		}
	})

	t.Run("fails if path is a file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		err := ensureEmptyDir(filePath)
		if err == nil {
			t.Fatal("expected error when path is a file")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "a", "b", "c", "newproject")

		err := ensureEmptyDir(targetDir)
		if err != nil {
			t.Fatalf("ensureEmptyDir failed: %v", err)
		}

		info, err := os.Stat(targetDir)
		if err != nil {
			t.Fatalf("directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory to be created")
		}
	})
}

func TestWriteMiseToml(t *testing.T) {
	t.Parallel()

	t.Run("writes mise.toml with default config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		cfg := DefaultMiseConfig()
		err := writeMiseToml(tmpDir, cfg)
		if err != nil {
			t.Fatalf("writeMiseToml failed: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(tmpDir, "mise.toml"))
		if err != nil {
			t.Fatalf("failed to read mise.toml: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, `go = "latest"`) {
			t.Error("mise.toml should contain go")
		}
		if !strings.Contains(contentStr, `node = "22"`) {
			t.Error("mise.toml should contain node@22")
		}
		if !strings.Contains(contentStr, `"npm:aws-cdk" = "latest"`) {
			t.Error("mise.toml should contain npm:aws-cdk")
		}
		if !strings.Contains(contentStr, `aws-cli = "latest"`) {
			t.Error("mise.toml should contain aws-cli")
		}
		if !strings.Contains(contentStr, `amp = "latest"`) {
			t.Error("mise.toml should contain amp")
		}
		if !strings.Contains(contentStr, `granted = "latest"`) {
			t.Error("mise.toml should contain granted")
		}
	})

	t.Run("writes mise.toml with custom config", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		cfg := MiseConfig{
			GoVersion:      "1.22",
			NodeVersion:    "20",
			AwsCdkVersion:  "2.0.0",
			AwsCliVersion:  "2.15.0",
			AmpVersion:     "1.0.0",
			GrantedVersion: "0.35.0",
		}
		err := writeMiseToml(tmpDir, cfg)
		if err != nil {
			t.Fatalf("writeMiseToml failed: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(tmpDir, "mise.toml"))
		if err != nil {
			t.Fatalf("failed to read mise.toml: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, `go = "1.22"`) {
			t.Errorf("mise.toml should contain go 1.22, got: %s", contentStr)
		}
		if !strings.Contains(contentStr, `node = "20"`) {
			t.Errorf("mise.toml should contain node 20, got: %s", contentStr)
		}
		if !strings.Contains(contentStr, `"npm:aws-cdk" = "2.0.0"`) {
			t.Errorf("mise.toml should contain aws-cdk 2.0.0, got: %s", contentStr)
		}
		if !strings.Contains(contentStr, `aws-cli = "2.15.0"`) {
			t.Errorf("mise.toml should contain aws-cli 2.15.0, got: %s", contentStr)
		}
		if !strings.Contains(contentStr, `amp = "1.0.0"`) {
			t.Errorf("mise.toml should contain amp 1.0.0, got: %s", contentStr)
		}
		if !strings.Contains(contentStr, `granted = "0.35.0"`) {
			t.Errorf("mise.toml should contain granted 0.35.0, got: %s", contentStr)
		}
	})
}

func TestCheckMiseInstalled(t *testing.T) {
	t.Parallel()

	t.Run("succeeds when mise is installed", func(t *testing.T) {
		t.Parallel()
		err := checkMiseInstalled(context.Background())
		if err != nil {
			t.Skip("mise is not installed, skipping test")
		}
	})
}

func TestInitGitRepo(t *testing.T) {
	t.Parallel()

	t.Run("initializes git repo in directory", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()

		exec := cmdexec.NewWithDir(tmpDir)
		err := exec.Run(context.Background(), "git", "init")
		if err != nil {
			t.Fatalf("git init failed: %v", err)
		}

		gitDir := filepath.Join(tmpDir, ".git")
		info, err := os.Stat(gitDir)
		if err != nil {
			t.Fatalf(".git directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected .git to be a directory")
		}
	})
}

func TestDoInit(t *testing.T) {
	t.Parallel()

	t.Run("full init without running mise install", func(t *testing.T) {
		t.Parallel()

		if err := checkMiseInstalled(context.Background()); err != nil {
			t.Skip("mise is not installed, skipping test")
		}

		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "newproject")

		opts := InitOptions{
			Dir:                 targetDir,
			MiseConfig:          DefaultMiseConfig(),
			CDKConfig:           DefaultCDKConfigFromDir(targetDir),
			BackendConfig:       DefaultBackendConfigFromDir(targetDir),
			RunInstall:          false,
			SkipAccountCreation: true,
			SkipCDKVerify:       true,
		}

		err := doInit(context.Background(), opts)
		if err != nil {
			t.Fatalf("doInit failed: %v", err)
		}

		info, err := os.Stat(targetDir)
		if err != nil {
			t.Fatalf("directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected a directory to be created")
		}

		miseToml := filepath.Join(targetDir, "mise.toml")
		if _, err := os.Stat(miseToml); err != nil {
			t.Fatalf("mise.toml was not created: %v", err)
		}
	})

	t.Run("full init with mise install", func(t *testing.T) {
		t.Parallel()

		if err := checkMiseInstalled(context.Background()); err != nil {
			t.Skip("mise is not installed, skipping test")
		}

		if testing.Short() {
			t.Skip("skipping mise install test in short mode")
		}

		tmpDir := t.TempDir()
		targetDir := filepath.Join(tmpDir, "newproject")

		opts := InitOptions{
			Dir:                 targetDir,
			MiseConfig:          DefaultMiseConfig(),
			CDKConfig:           DefaultCDKConfigFromDir(targetDir),
			BackendConfig:       DefaultBackendConfigFromDir(targetDir),
			RunInstall:          true,
			SkipAccountCreation: true,
			SkipCDKVerify:       true,
		}

		err := doInit(context.Background(), opts)
		if err != nil {
			t.Fatalf("doInit failed: %v", err)
		}
	})
}
