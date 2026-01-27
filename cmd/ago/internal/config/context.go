package config

import (
	"context"
)

type contextKey struct{}

type Context struct {
	Config     Config
	ProjectDir string
}

func WithContext(ctx context.Context, cfgCtx Context) context.Context {
	return context.WithValue(ctx, contextKey{}, cfgCtx)
}

func FromContext(ctx context.Context) (Context, bool) {
	cfgCtx, ok := ctx.Value(contextKey{}).(Context)
	return cfgCtx, ok
}

func MustFromContext(ctx context.Context) Context {
	cfgCtx, ok := FromContext(ctx)
	if !ok {
		panic("config.Context not found in context")
	}
	return cfgCtx
}
