package main

import (
	"context"

	"github.com/bitfield/script"
	"github.com/urfave/cli/v3"
)

func checkTests(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("go test ./...").Stdout()
	return err
}
