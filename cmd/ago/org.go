package main

import "github.com/urfave/cli/v3"

func orgCmd() *cli.Command {
	return &cli.Command{
		Name:  "org",
		Usage: "Organization and management account operations",
		Commands: []*cli.Command{
			orgCreateAccountCmd(),
			orgDestroyAccountCmd(),
			orgDNSDelegateCmd(),
			orgDNSUndelegateCmd(),
			orgDNSVerifyCmd(),
		},
	}
}
