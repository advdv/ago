package main

import (
	"context"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/cockroachdb/errors"
)

var goModuleDirs = []string{"infra", "backend"}

func runInGoModules(
	ctx context.Context,
	exec cmdexec.Executor,
	cmd string,
	args ...string,
) error {
	for _, subdir := range goModuleDirs {
		if err := exec.InSubdir(subdir).Run(ctx, cmd, args...); err != nil {
			return errors.Wrapf(err, "failed in %s", subdir)
		}
	}

	return nil
}
