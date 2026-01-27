package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

// Version is set via ldflags at build time.
var Version = "dev"

func main() {
	cmd := &cli.Command{
		Name:    "ago",
		Usage:   "Development task runner for the ago project",
		Version: Version,
		Commands: []*cli.Command{
			infraCmd(),
			checkCmd(),
			devCmd(),
			initCmd(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
