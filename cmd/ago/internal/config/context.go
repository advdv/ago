package config

import (
	"context"

	"github.com/urfave/cli/v3"
)

type contextKey struct{}

type Config struct {
	Inner      InnerConfig
	ProjectDir string
}

func WithContext(ctx context.Context, cfg Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

func FromContext(ctx context.Context) (Config, bool) {
	cfg, ok := ctx.Value(contextKey{}).(Config)
	return cfg, ok
}

func MustFromContext(ctx context.Context) Config {
	cfg, ok := FromContext(ctx)
	if !ok {
		panic("config.Config not found in context")
	}
	return cfg
}

// ActionFunc is a command action that receives the config.
type ActionFunc func(ctx context.Context, cmd *cli.Command, cfg Config) error

// WithConfig wraps an ActionFunc to automatically extract config from context.
func WithConfig(fn ActionFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		cfg := MustFromContext(ctx)
		return fn(ctx, cmd, cfg)
	}
}
