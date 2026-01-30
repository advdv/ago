package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func orgDNSUndelegateCmd() *cli.Command {
	return &cli.Command{
		Name:  "dns-undelegate",
		Usage: "Remove DNS delegation from parent zone",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "profile",
				Usage: "AWS profile for the project account (defaults to cdk.json profile)",
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: "AWS region where the delegation stack is deployed (defaults to primary region from context)",
			},
			&cli.StringFlag{
				Name:  "management-profile",
				Usage: "AWS profile for the management account (defaults to context management-profile)",
			},
			&cli.StringFlag{
				Name:     "confirm",
				Usage:    "Confirm undelegation by specifying the qualifier",
				Required: true,
			},
		},
		Action: config.RunWithConfig(runDNSUndelegate),
	}
}

type dnsUndelegateOptions struct {
	Profile           string
	Region            string
	ManagementProfile string
	Confirm           string
	Output            io.Writer
}

func runDNSUndelegate(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doDNSUndelegate(ctx, cfg, dnsUndelegateOptions{
		Profile:           cmd.String("profile"),
		Region:            cmd.String("region"),
		ManagementProfile: cmd.String("management-profile"),
		Confirm:           cmd.String("confirm"),
		Output:            os.Stdout,
	})
}

func doDNSUndelegate(ctx context.Context, cfg config.Config, opts dnsUndelegateOptions) error {
	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.Output)

	cdkContext, err := readCDKContext(cfg)
	if err != nil {
		return err
	}

	qualifier, err := cdkContext.getString("qualifier")
	if err != nil {
		return err
	}

	if opts.Confirm != qualifier {
		return errors.Errorf(
			"confirmation %q does not match qualifier %q", opts.Confirm, qualifier)
	}

	region := opts.Region
	if region == "" {
		region, err = cdkContext.getString("primary-region")
		if err != nil {
			return err
		}
	}

	managementProfile := opts.ManagementProfile
	if managementProfile == "" {
		managementProfile, err = cdkContext.getString("management-profile")
		if err != nil {
			return errors.Wrap(err, "management profile not found in context (provide --management-profile)")
		}
	}

	baseDomainName, err := cdkContext.getString("base-domain-name")
	if err != nil {
		return err
	}

	stackName := "ago-dns-delegate-" + qualifier

	exists, err := stackExists(ctx, exec, managementProfile, region, stackName)
	if err != nil {
		return err
	}

	if !exists {
		writeOutputf(opts.Output, "Stack %q does not exist. Nothing to delete.\n", stackName)
		return nil
	}

	writeOutputf(opts.Output, "Deleting DNS delegation stack %q...\n", stackName)
	writeOutputf(opts.Output, "  Domain: %s\n", baseDomainName)
	writeOutputf(opts.Output, "  Region: %s\n", region)
	writeOutputf(opts.Output, "  Profile: %s\n\n", managementProfile)

	if err := deleteDNSDelegationStack(ctx, exec, managementProfile, region, stackName); err != nil {
		return err
	}

	writeOutputf(opts.Output, "\nDNS delegation stack deleted successfully.\n")
	printDNSDelegatedWarning(opts.Output, cdkContext.prefix)

	return nil
}

func stackExists(
	ctx context.Context, exec cmdexec.Executor, profile, region, stackName string,
) (bool, error) {
	_, err := exec.MiseOutput(ctx, "aws", "cloudformation", "describe-stacks",
		"--stack-name", stackName,
		"--region", region,
		"--profile", profile,
		"--output", "json",
	)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "does not exist") ||
			strings.Contains(errStr, "ValidationError") {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check if stack %q exists", stackName)
	}
	return true, nil
}

func deleteDNSDelegationStack(
	ctx context.Context, exec cmdexec.Executor, profile, region, stackName string,
) error {
	if err := exec.Mise(ctx, "aws", "cloudformation", "delete-stack",
		"--stack-name", stackName,
		"--region", region,
		"--profile", profile,
	); err != nil {
		return errors.Wrap(err, "failed to delete DNS delegation stack")
	}

	if err := exec.Mise(ctx, "aws", "cloudformation", "wait", "stack-delete-complete",
		"--stack-name", stackName,
		"--region", region,
		"--profile", profile,
	); err != nil {
		return errors.Wrap(err, "failed waiting for stack deletion")
	}

	return nil
}

func printDNSDelegatedWarning(output io.Writer, prefix string) {
	writeOutputf(output, `
================================================================================
                              IMPORTANT NOTICE
================================================================================

The '%sdns-delegated' flag in cdk.context.json has NOT been changed.

WARNING: If you manually set this flag to false and then run 'cdk deploy',
CDK will DESTROY resources that depend on DNS validation (certificates,
API Gateway custom domains, CloudFront distributions, etc.).

Only set the flag to false if you understand and accept these consequences.

To restore DNS delegation, run: ago org dns-delegate

================================================================================
`, prefix)
}
