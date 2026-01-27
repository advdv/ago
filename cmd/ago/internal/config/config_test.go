package config_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/config"
)

func TestLoader(t *testing.T) {
	t.Parallel()

	t.Run("loads valid config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.FileName)
		if err := os.WriteFile(path, []byte("version: \"1\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		loader := config.NewLoader()
		cfg, err := loader.Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Version != "1" {
			t.Errorf("expected version '1', got %q", cfg.Version)
		}
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.FileName)
		if err := os.WriteFile(path, []byte("invalid: yaml: content:"), 0o644); err != nil {
			t.Fatal(err)
		}

		loader := config.NewLoader()
		_, err := loader.Load(path)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns error for invalid version", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.FileName)
		if err := os.WriteFile(path, []byte("version: \"2\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		loader := config.NewLoader()
		_, err := loader.Load(path)
		if err == nil {
			t.Fatal("expected error for invalid version, got nil")
		}
	})

	t.Run("returns error for missing version", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.FileName)
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		loader := config.NewLoader()
		_, err := loader.Load(path)
		if err == nil {
			t.Fatal("expected error for missing version, got nil")
		}
	})

	t.Run("strict mode rejects unknown fields", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.FileName)
		content := "version: \"1\"\nunknown_field: value\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		loader := config.NewLoader()
		_, err := loader.Load(path)
		if err == nil {
			t.Fatal("expected error for unknown field, got nil")
		}
	})
}

func TestWriter(t *testing.T) {
	t.Parallel()

	t.Run("writes config to writer", func(t *testing.T) {
		t.Parallel()
		cfg := config.Config{Version: "1"}
		w := config.NewWriter()

		var buf bytes.Buffer
		if err := w.Write(&buf, cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if buf.Len() == 0 {
			t.Error("expected non-empty output")
		}
	})
}

func TestFinder(t *testing.T) {
	t.Parallel()

	t.Run("finds config in current directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, config.FileName)
		if err := os.WriteFile(path, []byte("version: \"1\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		finder := config.NewFinder(config.NewLoader())
		cfg, projectDir, err := finder.Find(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projectDir != dir {
			t.Errorf("expected projectDir %q, got %q", dir, projectDir)
		}
		if cfg.Version != "1" {
			t.Errorf("expected version '1', got %q", cfg.Version)
		}
	})

	t.Run("finds config in parent directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		subDir := filepath.Join(root, "sub", "deep")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(root, config.FileName)
		if err := os.WriteFile(path, []byte("version: \"1\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		finder := config.NewFinder(config.NewLoader())
		cfg, projectDir, err := finder.Find(subDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projectDir != root {
			t.Errorf("expected projectDir %q, got %q", root, projectDir)
		}
		if cfg.Version != "1" {
			t.Errorf("expected version '1', got %q", cfg.Version)
		}
	})

	t.Run("returns error when config not found", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		finder := config.NewFinder(config.NewLoader())
		_, _, err := finder.Find(dir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestWriteToFile(t *testing.T) {
	t.Parallel()

	t.Run("writes config to file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		cfg := config.Config{Version: "1"}

		if err := config.WriteToFile(dir, cfg, config.NewWriter()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		finder := config.NewFinder(config.NewLoader())
		readCfg, _, err := finder.Find(dir)
		if err != nil {
			t.Fatalf("failed to read written config: %v", err)
		}
		if readCfg.Version != cfg.Version {
			t.Errorf("expected version %q, got %q", cfg.Version, readCfg.Version)
		}
	})
}
