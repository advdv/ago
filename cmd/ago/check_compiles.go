package main

import (
	"context"

	"github.com/bitfield/script"
	"github.com/urfave/cli/v3"
)

func checkCompiles(ctx context.Context, cmd *cli.Command) error {
	_, err := script.Exec("go build ./...").Stdout()
	return err
}
