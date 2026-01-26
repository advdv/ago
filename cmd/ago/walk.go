package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var defaultSkipDirs = map[string]struct{}{
	"node_modules": {},
	".git":         {},
	".svn":         {},
	".hg":          {},
	"vendor":       {},
	".terraform":   {},
	"dist":         {},
	"build":        {},
	".next":        {},
	"__pycache__":  {},
}

type WalkOptions struct {
	SkipDirs   map[string]struct{}
	Extensions []string
}

func DefaultWalkOptions() WalkOptions {
	return WalkOptions{
		SkipDirs: defaultSkipDirs,
	}
}

func WalkFiles(root string, opts WalkOptions, callback func(path string, entry fs.DirEntry) error) error {
	skipDirs := opts.SkipDirs
	if skipDirs == nil {
		skipDirs = defaultSkipDirs
	}

	extSet := make(map[string]struct{}, len(opts.Extensions))
	for _, ext := range opts.Extensions {
		extSet[ext] = struct{}{}
	}

	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if _, skip := skipDirs[entry.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}

		if len(extSet) > 0 {
			ext := filepath.Ext(path)
			if _, ok := extSet[ext]; !ok {
				return nil
			}
		}

		return callback(path, entry)
	})
}

func FindFilesByExtension(root string, extensions ...string) ([]string, error) {
	var files []string
	opts := DefaultWalkOptions()
	opts.Extensions = extensions

	err := WalkFiles(root, opts, func(path string, _ fs.DirEntry) error {
		files = append(files, path)
		return nil
	})

	return files, err
}

// Shell script detection heuristics:
// 1. File must have at least one executable bit set (user, group, or other)
// 2. File must start with a recognized shell shebang (first 32 bytes checked)
// 3. Supported shebangs: #!/bin/bash, #!/bin/sh, #!/usr/bin/env bash, #!/usr/bin/env sh.
var defaultShellShebangs = [][]byte{
	[]byte("#!/bin/bash"),
	[]byte("#!/bin/sh"),
	[]byte("#!/usr/bin/env bash"),
	[]byte("#!/usr/bin/env sh"),
}

// FindShellScripts walks root and returns paths to executable shell scripts.
func FindShellScripts(root string) ([]string, error) {
	var scripts []string
	opts := DefaultWalkOptions()

	err := WalkFiles(root, opts, func(path string, entry fs.DirEntry) error {
		info, infoErr := entry.Info()
		if infoErr != nil {
			return nil //nolint:nilerr // skip files we can't stat
		}

		if info.Mode()&0o111 == 0 {
			return nil
		}

		if isShellScript(path) {
			scripts = append(scripts, path)
		}

		return nil
	})

	return scripts, err
}

func isShellScript(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	buf := make([]byte, 32)
	numBytes, err := io.ReadAtLeast(file, buf, 2)
	if err != nil {
		return false
	}

	buf = buf[:numBytes]

	for _, shebang := range defaultShellShebangs {
		if bytes.HasPrefix(buf, shebang) {
			return true
		}
	}

	return false
}
