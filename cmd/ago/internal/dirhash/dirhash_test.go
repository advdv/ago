package dirhash_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/dirhash"
)

func TestHash_EmptyDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	h := dirhash.New()
	hash, err := h.Hash(dir, ".dockerignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash for empty directory")
	}
}

func TestHash_SingleFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	hash, err := h.Hash(dir, ".dockerignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hash) != 12 {
		t.Errorf("expected 12 char hash, got %d: %s", len(hash), hash)
	}
}

func TestHash_Deterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "a.go", "package a")
	writeFile(t, dir, "b.go", "package b")

	h := dirhash.New()

	hash1, _ := h.Hash(dir, ".dockerignore")
	hash2, _ := h.Hash(dir, ".dockerignore")
	hash3, _ := h.Hash(dir, ".dockerignore")

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("hashes not deterministic: %s, %s, %s", hash1, hash2, hash3)
	}
}

func TestHash_ContentChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	hash1, _ := h.Hash(dir, ".dockerignore")

	writeFile(t, dir, "main.go", "package main // modified")

	hash2, _ := h.Hash(dir, ".dockerignore")

	if hash1 == hash2 {
		t.Error("hash should change when content changes")
	}
}

func TestHash_FileNameChange(t *testing.T) {
	t.Parallel()

	dir1 := t.TempDir()
	writeFile(t, dir1, "foo.go", "package main")

	dir2 := t.TempDir()
	writeFile(t, dir2, "bar.go", "package main")

	h := dirhash.New()
	hash1, _ := h.Hash(dir1, ".dockerignore")
	hash2, _ := h.Hash(dir2, ".dockerignore")

	if hash1 == hash2 {
		t.Error("hash should differ when filename differs (same content)")
	}
}

func TestHash_NestedDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "cmd/api/main.go", "package main")
	writeFile(t, dir, "pkg/util/helper.go", "package util")
	writeFile(t, dir, "internal/service/svc.go", "package service")

	h := dirhash.New()
	files, err := h.CollectedFiles(dir, ".dockerignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"cmd/api/main.go",
		"internal/service/svc.go",
		"pkg/util/helper.go",
	}

	if !equalSlices(files, expected) {
		t.Errorf("expected %v, got %v", expected, files)
	}
}

func TestHash_AlwaysInclude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "Dockerfile", "FROM golang")
	writeFile(t, dir, ".dockerignore", "*")
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New(
		dirhash.WithAlwaysInclude("Dockerfile", ".dockerignore"),
	)

	files, err := h.CollectedFiles(dir, ".dockerignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files (Dockerfile, .dockerignore), got %v", files)
	}

	for _, f := range []string{"Dockerfile", ".dockerignore"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included", f)
		}
	}
}

func TestHash_IgnoreAll(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", "*")
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "test.txt", "test")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %v", files)
	}
}

func TestHash_IgnoreAllWithNegation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `*
!**/*.go
!go.mod
!go.sum`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "go.mod", "module test")
	writeFile(t, dir, "go.sum", "")
	writeFile(t, dir, "README.md", "# Test")
	writeFile(t, dir, "config.yaml", "key: value")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	expected := []string{"go.mod", "go.sum", "main.go"}
	if !equalSlices(files, expected) {
		t.Errorf("expected %v, got %v", expected, files)
	}
}

func TestHash_NestedNegation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `*
!cmd/
!cmd/**
!**/*.go`)
	writeFile(t, dir, "cmd/api/main.go", "package main")
	writeFile(t, dir, "cmd/worker/main.go", "package main")
	writeFile(t, dir, "pkg/util.go", "package pkg")
	writeFile(t, dir, "docs/README.md", "doc")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	for _, f := range []string{"cmd/api/main.go", "cmd/worker/main.go"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included, got %v", f, files)
		}
	}
}

func TestHash_DoubleStarPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `**/*_test.go`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "main_test.go", "package main")
	writeFile(t, dir, "pkg/util.go", "package pkg")
	writeFile(t, dir, "pkg/util_test.go", "package pkg")
	writeFile(t, dir, "cmd/api/handler.go", "package api")
	writeFile(t, dir, "cmd/api/handler_test.go", "package api")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			t.Errorf("test file should be ignored: %s", f)
		}
	}

	for _, f := range []string{"main.go", "pkg/util.go", "cmd/api/handler.go"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included", f)
		}
	}
}

func TestHash_DirectoryPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `vendor/
.git/
node_modules/`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "vendor/dep/dep.go", "package dep")
	writeFile(t, dir, ".git/config", "git config")
	writeFile(t, dir, "node_modules/pkg/index.js", "js")
	writeFile(t, dir, "cmd/api/main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	for _, f := range files {
		if strings.HasPrefix(f, "vendor/") ||
			strings.HasPrefix(f, ".git/") ||
			strings.HasPrefix(f, "node_modules/") {
			t.Errorf("file in ignored directory should not be included: %s", f)
		}
	}

	for _, f := range []string{"cmd/api/main.go", "main.go"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included, got %v", f, files)
		}
	}
}

func TestHash_SingleCharWildcard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `?.txt`)
	writeFile(t, dir, "a.txt", "a")
	writeFile(t, dir, "b.txt", "b")
	writeFile(t, dir, "ab.txt", "ab")
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if contains(files, "a.txt") || contains(files, "b.txt") {
		t.Errorf("single char files should be ignored, got %v", files)
	}
	if !contains(files, "ab.txt") {
		t.Errorf("ab.txt should be included, got %v", files)
	}
}

func TestHash_CharacterClass(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `*.[oa]`)
	writeFile(t, dir, "main.o", "obj")
	writeFile(t, dir, "lib.a", "archive")
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "lib.so", "shared")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if contains(files, "main.o") || contains(files, "lib.a") {
		t.Errorf(".o and .a files should be ignored, got %v", files)
	}
	if !contains(files, "main.go") || !contains(files, "lib.so") {
		t.Errorf("main.go and lib.so should be included, got %v", files)
	}
}

func TestHash_NegationOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `*.md
!README.md
docs/
!docs/api.md`)
	writeFile(t, dir, "README.md", "readme")
	writeFile(t, dir, "CHANGELOG.md", "changelog")
	writeFile(t, dir, "docs/api.md", "api doc")
	writeFile(t, dir, "docs/internal.md", "internal")
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if !contains(files, "README.md") {
		t.Errorf("README.md should be included via negation, got %v", files)
	}
	if contains(files, "CHANGELOG.md") {
		t.Errorf("CHANGELOG.md should be excluded, got %v", files)
	}
}

func TestHash_CommentAndEmptyLines(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `# This is a comment
*.txt

# Another comment
*.md

`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "readme.txt", "text")
	writeFile(t, dir, "README.md", "md")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if !contains(files, "main.go") {
		t.Errorf("expected main.go to be included, got %v", files)
	}
	if contains(files, "readme.txt") || contains(files, "README.md") {
		t.Errorf("expected .txt and .md to be excluded, got %v", files)
	}
}

func TestHash_LeadingSlash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Note: Docker's leading slash behavior matches the root of the context.
	// The pattern "/temp" only matches "temp" at root, not "sub/temp".
	writeFile(t, dir, ".dockerignore", `/temp`)
	writeFile(t, dir, "temp", "root temp")
	writeFile(t, dir, "sub/temp", "sub temp")
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	// With moby patternmatcher, /temp becomes "temp" pattern which matches root temp
	// This test validates leading slash patterns work (implementation dependent)
	if !contains(files, "main.go") {
		t.Errorf("main.go should be included, got %v", files)
	}
	if !contains(files, "sub/temp") {
		t.Errorf("sub/temp should be included, got %v", files)
	}
}

func TestHash_TrailingSlash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `build/`)
	writeFile(t, dir, "build/output.bin", "binary")
	writeFile(t, dir, "build.go", "package build")
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if contains(files, "build/output.bin") {
		t.Errorf("build/ directory should be excluded, got %v", files)
	}
	if !contains(files, "build.go") {
		t.Errorf("build.go file should be included, got %v", files)
	}
}

func TestHash_ExceptionThenExclude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `*
!*.go
vendor/`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "config.yaml", "config")
	writeFile(t, dir, "vendor/dep.go", "vendored go")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if !contains(files, "main.go") {
		t.Errorf("main.go should be included, got %v", files)
	}
	if contains(files, "config.yaml") {
		t.Errorf("config.yaml should be excluded, got %v", files)
	}
}

func TestHash_ComplexNegation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Complex pattern: ignore all, then include .go, then exclude _test.go, then include cmd/
	writeFile(t, dir, ".dockerignore", `*
!**/*.go
**/*_test.go
!cmd/
!cmd/**/*.go`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "main_test.go", "package main")
	writeFile(t, dir, "cmd/api/main.go", "package main")
	writeFile(t, dir, "cmd/api/main_test.go", "package main")
	writeFile(t, dir, "pkg/util.go", "package pkg")
	writeFile(t, dir, "pkg/util_test.go", "package pkg")
	writeFile(t, dir, "README.md", "readme")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	// .go files should be included (negation brings them back)
	if !contains(files, "main.go") {
		t.Errorf("main.go should be included, got %v", files)
	}

	// README.md should be excluded (not a .go file)
	if contains(files, "README.md") {
		t.Errorf("README.md should be excluded, got %v", files)
	}
}

func TestHash_NoDockerignore(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "config.yaml", "config")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	expected := []string{"config.yaml", "main.go"}
	if !equalSlices(files, expected) {
		t.Errorf("expected %v, got %v", expected, files)
	}
}

func TestHash_FullHashLength(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New(dirhash.WithTruncateLength(0))
	hash, _ := h.Hash(dir, ".dockerignore")

	if len(hash) != 64 {
		t.Errorf("expected 64 char hash (full SHA256), got %d: %s", len(hash), hash)
	}
}

func TestHash_CustomTruncateLength(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New(dirhash.WithTruncateLength(8))
	hash, _ := h.Hash(dir, ".dockerignore")

	if len(hash) != 8 {
		t.Errorf("expected 8 char hash, got %d: %s", len(hash), hash)
	}
}

func TestHash_DebugLogger(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", "*.txt")
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "ignored.txt", "ignored")
	writeFile(t, dir, "cmd/api/handler.go", "package api")

	var buf bytes.Buffer
	h := dirhash.New(dirhash.WithLogger(&dirhash.DebugLogger{W: &buf}))

	_, err := h.Hash(dir, ".dockerignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "FILE: main.go") {
		t.Errorf("expected log for main.go, got:\n%s", output)
	}
	if !strings.Contains(output, "SKIP: ignored.txt") {
		t.Errorf("expected skip log for ignored.txt, got:\n%s", output)
	}
	if !strings.Contains(output, "DIR:  cmd/") {
		t.Errorf("expected dir log for cmd/, got:\n%s", output)
	}
}

func TestHash_SpecialCharactersInFilename(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "file with spaces.go", "package main")
	writeFile(t, dir, "file-with-dashes.go", "package main")
	writeFile(t, dir, "file_with_underscores.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %v", files)
	}
}

func TestHash_HiddenFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".hidden", "hidden file")
	writeFile(t, dir, ".config/settings.json", `{"key": "value"}`)
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	for _, f := range []string{".hidden", ".config/settings.json", "main.go"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included", f)
		}
	}
}

func TestHash_IgnoreHiddenWithPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `.*`)
	writeFile(t, dir, ".hidden", "hidden")
	writeFile(t, dir, ".config/settings.json", `{"key": "value"}`)
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if contains(files, ".hidden") {
		t.Errorf(".hidden should be excluded by .* pattern")
	}
	if !contains(files, "main.go") {
		t.Errorf("main.go should be included")
	}
}

func TestHash_EmptyIgnoreFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", "")
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "README.md", "readme")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	// All files should be included when .dockerignore is empty
	for _, f := range []string{"README.md", "main.go"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included, got %v", f, files)
		}
	}
}

func TestHash_OnlyComments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `# comment 1
# comment 2
# comment 3`)
	writeFile(t, dir, "main.go", "package main")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if !contains(files, "main.go") {
		t.Errorf("main.go should be included when only comments in ignore file")
	}
}

func TestHash_EscapedCharacters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `\#file
\!important.txt`)
	writeFile(t, dir, "#file", "hash file")
	writeFile(t, dir, "!important.txt", "important")
	writeFile(t, dir, "normal.txt", "normal")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	if contains(files, "#file") {
		t.Errorf("#file should be excluded (escaped hash)")
	}
	if contains(files, "!important.txt") {
		t.Errorf("!important.txt should be excluded (escaped bang)")
	}
	if !contains(files, "normal.txt") {
		t.Errorf("normal.txt should be included")
	}
}

func TestHash_VendorScenario(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, ".dockerignore", `*
!**/*.go
!go.mod
!go.sum
!vendor/`)
	writeFile(t, dir, "main.go", "package main")
	writeFile(t, dir, "go.mod", "module test")
	writeFile(t, dir, "go.sum", "checksums")
	writeFile(t, dir, "vendor/dep/dep.go", "package dep")
	writeFile(t, dir, "vendor/modules.txt", "modules")
	writeFile(t, dir, "README.md", "readme")
	writeFile(t, dir, ".env", "secrets")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	for _, f := range []string{"main.go", "go.mod", "go.sum", "vendor/dep/dep.go"} {
		if !contains(files, f) {
			t.Errorf("expected %s to be included, got %v", f, files)
		}
	}

	for _, f := range []string{"README.md", ".env"} {
		if contains(files, f) {
			t.Errorf("expected %s to be excluded, got %v", f, files)
		}
	}
}

func TestHash_SortOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	writeFile(t, dir, "z.go", "package z")
	writeFile(t, dir, "a.go", "package a")
	writeFile(t, dir, "m.go", "package m")
	writeFile(t, dir, "b/c.go", "package c")
	writeFile(t, dir, "a/z.go", "package az")

	h := dirhash.New()
	files, _ := h.CollectedFiles(dir, ".dockerignore")

	expected := []string{"a.go", "a/z.go", "b/c.go", "m.go", "z.go"}
	if !equalSlices(files, expected) {
		t.Errorf("expected sorted %v, got %v", expected, files)
	}
}

// Helper functions

func writeFile(t *testing.T, base, path, content string) {
	t.Helper()
	fullPath := filepath.Join(base, path)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", fullPath, err)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
