package main

import "github.com/urfave/cli/v3"

func tfCmd() *cli.Command {
	return &cli.Command{
		Name:  "tf",
		Usage: "Terraform infrastructure management",
		Commands: []*cli.Command{
			tfInitCmd(),
			tfPlanCmd(),
			tfApplyCmd(),
		},
	}
}
