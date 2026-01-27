package cmdexec_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
)

func TestNew(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		ProjectDir: "/test/project",
	}

	exec := cmdexec.New(cfg)
	if exec.Dir() != "/test/project" {
		t.Errorf("expected dir /test/project, got %s", exec.Dir())
	}
}

func TestNewWithDir(t *testing.T) {
	t.Parallel()

	exec := cmdexec.NewWithDir("/custom/dir")
	if exec.Dir() != "/custom/dir" {
		t.Errorf("expected dir /custom/dir, got %s", exec.Dir())
	}
}

func TestInSubdir(t *testing.T) {
	t.Parallel()

	exec := cmdexec.NewWithDir("/project")
	subExec := exec.InSubdir("infra/cdk")

	if subExec.Dir() != "/project/infra/cdk" {
		t.Errorf("expected dir /project/infra/cdk, got %s", subExec.Dir())
	}

	// Original should be unchanged
	if exec.Dir() != "/project" {
		t.Errorf("original executor dir changed to %s", exec.Dir())
	}
}

func TestRun(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var stdout, stderr bytes.Buffer

	exec := cmdexec.NewWithDir(dir).WithOutput(&stdout, &stderr)
	err := exec.Run(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.String() != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", stdout.String())
	}
}

func TestRunInCorrectDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	var stdout bytes.Buffer

	exec := cmdexec.NewWithDir(dir).WithOutput(&stdout, nil)
	err := exec.Run(context.Background(), "pwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks for macOS /private/var -> /var
	expectedDir, _ := filepath.EvalSymlinks(dir)
	gotDir, _ := filepath.EvalSymlinks(stdout.String()[:len(stdout.String())-1])

	if gotDir != expectedDir {
		t.Errorf("expected dir %s, got %s", expectedDir, gotDir)
	}
}

func TestOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exec := cmdexec.NewWithDir(dir)

	output, err := exec.Output(context.Background(), "echo", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "hello world" {
		t.Errorf("expected 'hello world', got %q", output)
	}
}

func TestRunError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exec := cmdexec.NewWithDir(dir)

	err := exec.Run(context.Background(), "false")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestOutputError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exec := cmdexec.NewWithDir(dir)

	_, err := exec.Output(context.Background(), "false")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWithOutputImmutability(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	exec1 := cmdexec.NewWithDir(dir)

	var buf bytes.Buffer
	exec2 := exec1.WithOutput(&buf, nil)

	// Run on exec2 should write to buf
	_ = exec2.Run(context.Background(), "echo", "test")

	if buf.Len() == 0 {
		t.Error("expected output in buffer")
	}
}

func TestMiseOutput(t *testing.T) {
	t.Parallel()

	// Skip if mise is not available
	if _, err := cmdexec.NewWithDir(".").Output(context.Background(), "which", "mise"); err != nil {
		t.Skip("mise not available")
	}

	dir := t.TempDir()

	// Create a mise.toml so mise has a valid project
	if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("[tools]\n"), 0o644); err != nil {
		t.Fatalf("failed to create mise.toml: %v", err)
	}

	// Trust the mise config first
	exec := cmdexec.NewWithDir(dir)
	if err := exec.Run(context.Background(), "mise", "trust"); err != nil {
		t.Skip("mise trust failed, skipping test")
	}

	output, err := exec.MiseOutput(context.Background(), "echo", "from mise")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output != "from mise" {
		t.Errorf("expected 'from mise', got %q", output)
	}
}
