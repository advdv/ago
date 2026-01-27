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
		cfg := config.Config{
			Inner:      config.InnerConfig{Version: "1"},
			ProjectDir: "/test/dir",
		}

		ctx = config.WithContext(ctx, cfg)
		got, ok := config.FromContext(ctx)

		if !ok {
			t.Fatal("expected config to be found")
		}
		if got.Inner.Version != cfg.Inner.Version {
			t.Errorf("expected version %q, got %q", cfg.Inner.Version, got.Inner.Version)
		}
		if got.ProjectDir != cfg.ProjectDir {
			t.Errorf("expected projectDir %q, got %q", cfg.ProjectDir, got.ProjectDir)
		}
	})

	t.Run("FromContext returns false when not set", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		_, ok := config.FromContext(ctx)
		if ok {
			t.Error("expected config to not be found")
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
		cfg := config.Config{
			Inner:      config.InnerConfig{Version: "1"},
			ProjectDir: "/test/dir",
		}

		ctx = config.WithContext(ctx, cfg)
		got := config.MustFromContext(ctx)

		if got.Inner.Version != cfg.Inner.Version {
			t.Errorf("expected version %q, got %q", cfg.Inner.Version, got.Inner.Version)
		}
	})
}
