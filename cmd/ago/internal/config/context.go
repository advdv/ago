package config

import (
	"context"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

type contextKey struct{}

type Config struct {
	Inner      InnerConfig
	ProjectDir string
}

// CDKDir returns the path to the CDK directory (infra/cdk/cdk).
func (c Config) CDKDir() string {
	return filepath.Join(c.ProjectDir, "infra", "cdk", "cdk")
}

// CDKContextPath returns the path to cdk.context.json.
func (c Config) CDKContextPath() string {
	return filepath.Join(c.CDKDir(), "cdk.context.json")
}

// CDKJSONPath returns the path to cdk.json.
func (c Config) CDKJSONPath() string {
	return filepath.Join(c.CDKDir(), "cdk.json")
}

func WithContext(ctx context.Context, cfg Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

func FromContext(ctx context.Context) (Config, bool) {
	cfg, ok := ctx.Value(contextKey{}).(Config)
	return cfg, ok
}

var defaultFinder = NewFinder(NewLoader())

// Ensure returns config from context if present, otherwise loads it from disk.
// This enables lazy config loading - config is only loaded when an action needs it.
func Ensure(ctx context.Context) (context.Context, Config, error) {
	if cfg, ok := FromContext(ctx); ok {
		return ctx, cfg, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ctx, Config{}, err
	}

	inner, projectDir, err := defaultFinder.Find(cwd)
	if err != nil {
		return ctx, Config{}, err
	}

	cfg := Config{Inner: inner, ProjectDir: projectDir}
	return WithContext(ctx, cfg), cfg, nil
}

// ActionFunc is a command action that receives the config.
type ActionFunc func(ctx context.Context, cmd *cli.Command, cfg Config) error

// RunWithConfig wraps an ActionFunc to lazily load config when the action runs.
// Config is only loaded when an actual command action executes, not when showing help.
func RunWithConfig(fn ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		ctx, cfg, err := Ensure(ctx)
		if err != nil {
			return err
		}
		return fn(ctx, cmd, cfg)
	}
}
