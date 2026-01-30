package main

import "github.com/urfave/cli/v3"

func dnsCmd() *cli.Command {
	return &cli.Command{
		Name:  "dns",
		Usage: "DNS management commands",
		Commands: []*cli.Command{
			dnsDelegateCmd(),
			dnsUndelegateCmd(),
		},
	}
}
