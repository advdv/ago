package main

import (
	"context"

	"github.com/bitfield/script"
	"github.com/urfave/cli/v3"
)

func devFmt(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("golangci-lint fmt ./...").Stdout()
	return err
}
