---
name: building-ago-commands
description: Creates CLI commands for the ago tool following established patterns. Use when adding new subcommands to ago, implementing command actions, or working with the cmdexec package.
---

# Building Ago Commands

Creates CLI commands for the `ago` tool using consistent patterns for command execution, configuration access, and testing.

## Config Loading Architecture

Config is loaded **lazily** when an action runs, not in a global `Before` hook. This allows:
- `ago` (no args) → shows help without requiring config
- `ago dev` → shows subcommand help without requiring config
- `ago dev fmt` → loads config when the action executes

The `config.RunWithConfig()` wrapper handles lazy loading via `config.Ensure()`.

## Command Structure

### Standard Command (with config)

Commands that require project config use `config.RunWithConfig`:

```go
// cmd/ago/example.go
package main

import (
    "context"
    "os"

    "github.com/advdv/ago/cmd/ago/internal/cmdexec"
    "github.com/advdv/ago/cmd/ago/internal/config"
    "github.com/urfave/cli/v3"
)

func exampleCmd() *cli.Command {
    return &cli.Command{
        Name:   "example",
        Usage:  "Example command description",
        Action: config.RunWithConfig(runExample),
    }
}

func runExample(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
    return doExample(ctx, cfg, exampleOptions{
        SomeFlag: cmd.Bool("some-flag"),
        Output:   os.Stdout,
    })
}

type exampleOptions struct {
    SomeFlag bool
    Output   io.Writer
}

func doExample(ctx context.Context, cfg config.Config, opts exampleOptions) error {
    exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.Output)
    return exec.Run(ctx, "some-command", "arg1", "arg2")
}
```

### Init-style Command (no config)

Commands that run before config exists use `cmdexec.NewWithDir`:

```go
func runInit(ctx context.Context, cmd *cli.Command) error {
    dir := cmd.Args().First()
    if dir == "" {
        dir, _ = os.Getwd()
    }
    
    exec := cmdexec.NewWithDir(dir).WithOutput(os.Stdout, os.Stderr)
    return exec.Run(ctx, "git", "init")
}
```

## cmdexec Package

The `cmdexec` package provides consistent command execution with proper working directory handling.

### Constructors

| Function | Use Case |
|----------|----------|
| `cmdexec.New(cfg config.Context)` | Standard commands with config |
| `cmdexec.NewWithDir(dir string)` | Init-like commands, tests |

### Configuration Methods (immutable, return new Executor)

```go
exec := cmdexec.New(cfg).
    WithOutput(os.Stdout, os.Stderr).  // Set stdout/stderr
    InSubdir("infra/cdk/cdk")          // Change to subdirectory
```

### Execution Methods

| Method | Returns | Use Case |
|--------|---------|----------|
| `Run(ctx, name, args...)` | `error` | Stream output to configured writers |
| `Output(ctx, name, args...)` | `(string, error)` | Capture stdout as trimmed string |
| `Mise(ctx, name, args...)` | `error` | Run via `mise exec --` |
| `MiseOutput(ctx, name, args...)` | `(string, error)` | Capture output from mise command |

### Directory Access

```go
exec.Dir()  // Returns the working directory
```

## File Organization

```
cmd/ago/
├── main.go                 # CLI setup (no Before hook, config loaded lazily)
├── example.go              # Parent command with subcommands
├── example_action.go       # Individual action implementation
└── example_test.go         # Tests
```

### Parent Command Pattern

```go
// cmd/ago/check.go
func checkCmd() *cli.Command {
    return &cli.Command{
        Name:  "check",
        Usage: "Run various checks",
        Commands: []*cli.Command{
            {
                Name:   "tests",
                Usage:  "Run Go tests",
                Action: config.RunWithConfig(checkTests),
            },
            {
                Name:   "lint", 
                Usage:  "Lint Go code",
                Action: config.RunWithConfig(checkLint),
            },
        },
    }
}
```

## Testing Pattern

Separate CLI parsing (`runXxx`) from business logic (`doXxx`) for testability:

```go
// cmd/ago/example_test.go
func TestDoExample(t *testing.T) {
    t.Parallel()
    
    dir := t.TempDir()
    cfg := config.Context{ProjectDir: dir}
    
    var output bytes.Buffer
    opts := exampleOptions{
        SomeFlag: true,
        Output:   &output,
    }
    
    err := doExample(context.Background(), cfg, opts)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}
```

For init-style commands without config:

```go
func TestInitCommand(t *testing.T) {
    dir := t.TempDir()
    exec := cmdexec.NewWithDir(dir)
    
    err := exec.Run(context.Background(), "git", "init")
    // ...
}
```

## Code Style Rules

1. **Always pass context through**: Use `ctx` parameter, never `context.Background()` in command handlers
2. **Line length**: Break function signatures at 120 characters
   ```go
   func longFunctionName(
       ctx context.Context, exec cmdexec.Executor, opts someOptions,
   ) error {
   ```
3. **No `--project-dir` flags**: Commands get `cfg.ProjectDir` from context
4. **Options structs**: Include `Output io.Writer` for testable output
5. **Error wrapping**: Use `github.com/cockroachdb/errors`

## Registering New Commands

Add to `main.go`:

```go
cmd := &cli.Command{
    Name: "ago",
    Commands: []*cli.Command{
        cdkCmd(),
        checkCmd(),
        devCmd(),
        initCmd(),
        exampleCmd(),  // Add new command here
    },
    // ...
}
```

## Common Patterns

### Running Go tools
```go
exec.Run(ctx, "go", "test", "./...")
exec.Run(ctx, "go", "build", "./...")
exec.Run(ctx, "golangci-lint", "run", "./...")
```

### Running AWS CLI via mise
```go
exec.Mise(ctx, "aws", "cloudformation", "deploy", "--stack-name", name, ...)
output, err := exec.MiseOutput(ctx, "aws", "sts", "get-caller-identity", ...)
```

### Running CDK commands
```go
cdkExec := exec.InSubdir("infra/cdk/cdk")
cdkExec.Mise(ctx, "cdk", "deploy", "--profile", profile, ...)
```

### Finding shell scripts
```go
shellFiles, err := FindShellScripts(exec.Dir())
if len(shellFiles) > 0 {
    exec.Run(ctx, "shellcheck", shellFiles...)
}
```
