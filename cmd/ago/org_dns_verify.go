package main

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func orgDNSVerifyCmd() *cli.Command {
	return &cli.Command{
		Name:  "dns-verify",
		Usage: "Verify DNS delegation is working and update the dns-delegated flag",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "stack-name",
				Usage: "CloudFormation stack name containing the hosted zone (defaults to {qualifier}-Shared-{region-ident})",
			},
			&cli.StringFlag{
				Name:  "profile",
				Usage: "AWS profile for the project account (defaults to cdk.json profile)",
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: "AWS region where the shared stack is deployed (defaults to primary region from context)",
			},
			&cli.BoolFlag{
				Name:  "wait",
				Usage: "Wait for DNS propagation instead of checking once",
				Value: false,
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Usage: "Timeout for DNS propagation verification (only used with --wait)",
				Value: time.Hour,
			},
		},
		Action: config.RunWithConfig(runDNSVerify),
	}
}

type dnsVerifyOptions struct {
	StackName string
	Profile   string
	Region    string
	Wait      bool
	Timeout   time.Duration
	Output    io.Writer
}

func runDNSVerify(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doDNSVerify(ctx, cfg, dnsVerifyOptions{
		StackName: cmd.String("stack-name"),
		Profile:   cmd.String("profile"),
		Region:    cmd.String("region"),
		Wait:      cmd.Bool("wait"),
		Timeout:   cmd.Duration("timeout"),
		Output:    os.Stdout,
	})
}

func doDNSVerify(ctx context.Context, cfg config.Config, opts dnsVerifyOptions) error {
	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.Output)

	cdkContext, err := readCDKContext(cfg)
	if err != nil {
		return err
	}

	profile := opts.Profile
	if profile == "" {
		profile, err = getCDKProfile(cfg)
		if err != nil {
			return err
		}
	}

	region := opts.Region
	if region == "" {
		region, err = cdkContext.getString("primary-region")
		if err != nil {
			return err
		}
	}

	stackName := opts.StackName
	if stackName == "" {
		stackName, err = deriveSharedStackName(cdkContext, region)
		if err != nil {
			return err
		}
	}

	nameServers, err := getStackOutputValue(ctx, exec, profile, region, stackName, "HostedZoneNameServers")
	if err != nil {
		return errors.Wrap(err, "failed to get name servers from stack (is the shared stack deployed?)")
	}

	baseDomainName, err := cdkContext.getString("base-domain-name")
	if err != nil {
		return err
	}

	nsList := strings.Split(nameServers, ",")

	writeOutputf(opts.Output, "Verifying DNS delegation for %s\n", baseDomainName)
	writeOutputf(opts.Output, "Expected name servers:\n")
	for _, ns := range nsList {
		writeOutputf(opts.Output, "  %s\n", ns)
	}
	writeOutputf(opts.Output, "\n")

	if opts.Wait {
		if err := waitForDNSPropagation(ctx, opts.Output, baseDomainName, nsList, opts.Timeout); err != nil {
			return err
		}
	} else {
		verified, err := checkDNSOnce(ctx, baseDomainName, nsList)
		if err != nil {
			return errors.Wrap(err, "DNS lookup failed")
		}
		if !verified {
			writeOutputf(opts.Output, "DNS delegation NOT verified. NS records do not match expected values.\n")
			writeOutputf(opts.Output, "Run with --wait to poll until propagation completes.\n")
			return errors.New("DNS delegation not verified")
		}
		writeOutputf(opts.Output, "DNS records verified via %s\n", publicDNSServer)
	}

	if err := setDNSDelegatedFlag(cfg, cdkContext.prefix); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Updated cdk.context.json: dns-delegated = true\n")
	writeOutputf(opts.Output, "\nDNS verification complete.\n")

	return nil
}

func checkDNSOnce(ctx context.Context, baseDomainName string, expectedNS []string) (bool, error) {
	resolver := newPublicDNSResolver()

	expectedSet := make(map[string]bool, len(expectedNS))
	for _, ns := range expectedNS {
		normalized := strings.TrimSuffix(strings.ToLower(ns), ".") + "."
		expectedSet[normalized] = true
	}

	nsRecords, err := lookupNSWithRetry(ctx, resolver, baseDomainName)
	if err != nil {
		return false, err
	}

	return nsRecordsMatch(nsRecords, expectedSet), nil
}
