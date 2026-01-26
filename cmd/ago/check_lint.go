package main

import (
	"context"

	"github.com/bitfield/script"
	"github.com/urfave/cli/v3"
)

func checkLint(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("golangci-lint run ./...").Stdout()
	return err
}
