package config_test

import (
	"context"
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/config"
)

func TestContext(t *testing.T) {
	t.Parallel()

	t.Run("WithContext and FromContext", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cfgCtx := config.Context{
			Config:     config.Config{Version: "1"},
			ProjectDir: "/test/dir",
		}

		ctx = config.WithContext(ctx, cfgCtx)
		got, ok := config.FromContext(ctx)

		if !ok {
			t.Fatal("expected config context to be found")
		}
		if got.Config.Version != cfgCtx.Config.Version {
			t.Errorf("expected version %q, got %q", cfgCtx.Config.Version, got.Config.Version)
		}
		if got.ProjectDir != cfgCtx.ProjectDir {
			t.Errorf("expected projectDir %q, got %q", cfgCtx.ProjectDir, got.ProjectDir)
		}
	})

	t.Run("FromContext returns false when not set", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		_, ok := config.FromContext(ctx)
		if ok {
			t.Error("expected config context to not be found")
		}
	})

	t.Run("MustFromContext panics when not set", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic, got none")
			}
		}()

		config.MustFromContext(ctx)
	})

	t.Run("MustFromContext returns config when set", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cfgCtx := config.Context{
			Config:     config.Config{Version: "1"},
			ProjectDir: "/test/dir",
		}

		ctx = config.WithContext(ctx, cfgCtx)
		got := config.MustFromContext(ctx)

		if got.Config.Version != cfgCtx.Config.Version {
			t.Errorf("expected version %q, got %q", cfgCtx.Config.Version, got.Config.Version)
		}
	})
}
