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

	t.Run("Ensure returns existing config from context", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cfg := config.Config{
			Inner:      config.InnerConfig{Version: "1"},
			ProjectDir: "/test/dir",
		}

		ctx = config.WithContext(ctx, cfg)
		newCtx, got, err := config.Ensure(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ProjectDir != cfg.ProjectDir {
			t.Errorf("expected projectDir %q, got %q", cfg.ProjectDir, got.ProjectDir)
		}
		if newCtx != ctx {
			t.Error("expected same context when config already present")
		}
	})
}
