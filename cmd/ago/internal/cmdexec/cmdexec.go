package cmdexec

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
)

// Executor provides a common interface for executing external commands.
type Executor interface {
	// WithOutput returns a new Executor that writes to the given stdout/stderr.
	WithOutput(stdout, stderr io.Writer) Executor

	// InSubdir returns a new Executor that runs commands in a subdirectory.
	InSubdir(subdir string) Executor

	// WithEnv returns a new Executor with an additional environment variable.
	WithEnv(key, value string) Executor

	// Dir returns the working directory for this executor.
	Dir() string

	// Run executes a command and streams output to configured writers.
	Run(ctx context.Context, name string, args ...string) error

	// RunWithStdin executes a command with stdin from a reader.
	RunWithStdin(ctx context.Context, stdin io.Reader, name string, args ...string) error

	// Output executes a command and returns stdout as a string.
	Output(ctx context.Context, name string, args ...string) (string, error)

	// Mise executes a command wrapped with "mise exec --".
	Mise(ctx context.Context, name string, args ...string) error

	// MiseOutput executes a mise-wrapped command and returns stdout as a string.
	MiseOutput(ctx context.Context, name string, args ...string) (string, error)
}

// executor is the default implementation of Executor.
type executor struct {
	dir    string
	stdout io.Writer
	stderr io.Writer
	env    []string
}

// New creates an Executor from config.Config.
func New(cfg config.Config) Executor {
	return &executor{
		dir: cfg.ProjectDir,
	}
}

// NewWithDir creates an Executor with an explicit working directory.
// Use this for commands like init where no config exists yet.
func NewWithDir(dir string) Executor {
	return &executor{
		dir: dir,
	}
}

func (e *executor) WithOutput(stdout, stderr io.Writer) Executor {
	return &executor{
		dir:    e.dir,
		stdout: stdout,
		stderr: stderr,
		env:    e.env,
	}
}

func (e *executor) InSubdir(subdir string) Executor {
	return &executor{
		dir:    filepath.Join(e.dir, subdir),
		stdout: e.stdout,
		stderr: e.stderr,
		env:    e.env,
	}
}

func (e *executor) WithEnv(key, value string) Executor {
	newEnv := make([]string, len(e.env), len(e.env)+1)
	copy(newEnv, e.env)
	newEnv = append(newEnv, key+"="+value)

	return &executor{
		dir:    e.dir,
		stdout: e.stdout,
		stderr: e.stderr,
		env:    newEnv,
	}
}

func (e *executor) Dir() string {
	return e.dir
}

func (e *executor) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = e.dir
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	e.applyEnv(cmd)

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "%s failed", name)
	}

	return nil
}

func (e *executor) RunWithStdin(ctx context.Context, stdin io.Reader, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = e.dir
	cmd.Stdin = stdin
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr
	e.applyEnv(cmd)

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "%s failed", name)
	}

	return nil
}

func (e *executor) Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = e.dir
	e.applyEnv(cmd)

	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "%s failed", name)
	}

	return strings.TrimSpace(string(output)), nil
}

func (e *executor) Mise(ctx context.Context, name string, args ...string) error {
	miseArgs := make([]string, 0, 2+len(args))
	miseArgs = append(miseArgs, "exec", "--", name)
	miseArgs = append(miseArgs, args...)

	return e.Run(ctx, "mise", miseArgs...)
}

func (e *executor) MiseOutput(ctx context.Context, name string, args ...string) (string, error) {
	miseArgs := make([]string, 0, 2+len(args))
	miseArgs = append(miseArgs, "exec", "--", name)
	miseArgs = append(miseArgs, args...)

	return e.Output(ctx, "mise", miseArgs...)
}

func (e *executor) applyEnv(cmd *exec.Cmd) {
	if len(e.env) > 0 {
		cmd.Env = append(os.Environ(), e.env...)
	}
}
