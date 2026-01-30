package main

import "github.com/urfave/cli/v3"

func infraCmd() *cli.Command {
	return &cli.Command{
		Name:  "infra",
		Usage: "Infrastructure and cloud account management",
		Commands: []*cli.Command{
			createAWSAccountCmd(),
			destroyAWSAccountCmd(),
			cdkCmd(),
			tfCmd(),
			dnsCmd(),
		},
	}
}
