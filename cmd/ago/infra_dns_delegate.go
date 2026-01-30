package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/advdv/ago/agcdkutil"
	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func dnsDelegateCmd() *cli.Command {
	return &cli.Command{
		Name:  "delegate",
		Usage: "Set up DNS delegation from parent zone to project hosted zone",
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
			&cli.StringFlag{
				Name:  "management-profile",
				Usage: "AWS profile for the management account (defaults to context management-profile)",
			},
		},
		Action: config.RunWithConfig(runDNSDelegate),
	}
}

type dnsDelegateOptions struct {
	StackName         string
	Profile           string
	Region            string
	ManagementProfile string
	Output            io.Writer
}

func runDNSDelegate(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doDNSDelegate(ctx, cfg, dnsDelegateOptions{
		StackName:         cmd.String("stack-name"),
		Profile:           cmd.String("profile"),
		Region:            cmd.String("region"),
		ManagementProfile: cmd.String("management-profile"),
		Output:            os.Stdout,
	})
}

func doDNSDelegate(ctx context.Context, cfg config.Config, opts dnsDelegateOptions) error {
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
		return err
	}

	managementProfile := opts.ManagementProfile
	if managementProfile == "" {
		managementProfile, err = cdkContext.getString("management-profile")
		if err != nil {
			return errors.Wrap(err, "management profile not found in context (run 'ago init' or provide --management-profile)")
		}
	}

	baseDomainName, err := cdkContext.getString("base-domain-name")
	if err != nil {
		return err
	}

	qualifier, err := cdkContext.getString("qualifier")
	if err != nil {
		return err
	}

	parentZoneID, err := lookupParentZoneID(ctx, exec, managementProfile, region, baseDomainName)
	if err != nil {
		return err
	}

	nsList := strings.Split(nameServers, ",")

	writeOutputf(opts.Output, "Delegating %s to parent zone %s\n", baseDomainName, parentZoneID)
	writeOutputf(opts.Output, "Name servers:\n")
	for _, ns := range nsList {
		writeOutputf(opts.Output, "  %s\n", ns)
	}

	templatePath, cleanup, err := renderNSDelegationTemplate(nsDelegationData{
		Qualifier:      qualifier,
		BaseDomainName: baseDomainName,
		ParentZoneID:   parentZoneID,
		NameServers:    nsList,
	})
	if err != nil {
		return errors.Wrap(err, "failed to render NS delegation template")
	}
	defer cleanup()

	stackName = "ago-dns-delegate-" + qualifier

	writeOutputf(opts.Output, "\nDeploying stack %q to management account...\n", stackName)

	if err := exec.Mise(ctx, "aws", "cloudformation", "deploy",
		"--stack-name", stackName,
		"--template-file", templatePath,
		"--region", region,
		"--profile", managementProfile,
		"--no-fail-on-empty-changeset",
	); err != nil {
		return errors.Wrap(err, "failed to deploy NS delegation stack")
	}

	writeOutputf(opts.Output, "\nDNS delegation complete!\n")

	return nil
}

func getStackOutputValue(
	ctx context.Context, exec cmdexec.Executor, profile, region, stackName, outputKey string,
) (string, error) {
	output, err := exec.MiseOutput(ctx, "aws", "cloudformation", "describe-stacks",
		"--stack-name", stackName,
		"--region", region,
		"--profile", profile,
		"--query", "Stacks[0].Outputs",
		"--output", "json",
	)
	if err != nil {
		return "", errors.Wrapf(err, "failed to describe stack %q", stackName)
	}

	var outputs []struct {
		OutputKey   string `json:"OutputKey"`   //nolint:tagliatelle // AWS API uses PascalCase
		OutputValue string `json:"OutputValue"` //nolint:tagliatelle // AWS API uses PascalCase
	}
	if err := json.Unmarshal([]byte(output), &outputs); err != nil {
		return "", errors.Wrap(err, "failed to parse stack outputs")
	}

	for _, o := range outputs {
		if o.OutputKey == outputKey {
			return o.OutputValue, nil
		}
	}

	return "", errors.Errorf("output %q not found in stack %q", outputKey, stackName)
}

type cdkContextData struct {
	prefix string
	data   map[string]any
}

func readCDKContext(cfg config.Config) (*cdkContextData, error) {
	contextPath := cfg.CDKContextPath()

	data, err := os.ReadFile(contextPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read cdk.context.json")
	}

	var context map[string]any
	if err := json.Unmarshal(data, &context); err != nil {
		return nil, errors.Wrap(err, "failed to parse cdk.context.json")
	}

	prefix, err := findCDKPrefix(context)
	if err != nil {
		return nil, err
	}

	return &cdkContextData{prefix: prefix, data: context}, nil
}

func findCDKPrefix(context map[string]any) (string, error) {
	for key := range context {
		if prefix, found := strings.CutSuffix(key, "qualifier"); found {
			return prefix, nil
		}
	}
	return "", errors.New("could not determine CDK prefix from context (no *qualifier key found)")
}

func (c *cdkContextData) getString(name string) (string, error) {
	key := c.prefix + name
	val, ok := c.data[key]
	if !ok {
		return "", errors.Errorf("context key %q not found", key)
	}
	s, ok := val.(string)
	if !ok {
		return "", errors.Errorf("context key %q is not a string", key)
	}
	return s, nil
}

func deriveSharedStackName(cdkCtx *cdkContextData, region string) (string, error) {
	qualifier, err := cdkCtx.getString("qualifier")
	if err != nil {
		return "", err
	}

	regionIdent := agcdkutil.RegionIdentFor(region)

	return agcdkutil.SharedStackName(qualifier, regionIdent), nil
}

func getCDKProfile(cfg config.Config) (string, error) {
	cdkJSONPath := cfg.CDKJSONPath()

	data, err := os.ReadFile(cdkJSONPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read cdk.json")
	}

	var cdkJSON map[string]any
	if err := json.Unmarshal(data, &cdkJSON); err != nil {
		return "", errors.Wrap(err, "failed to parse cdk.json")
	}

	profile, ok := cdkJSON["profile"].(string)
	if !ok || profile == "" {
		return "", errors.New("profile not found in cdk.json")
	}

	return profile, nil
}

func extractParentDomain(baseDomainName string) (string, error) {
	parts := strings.Split(baseDomainName, ".")
	if len(parts) < 3 {
		return "", errors.Errorf(
			"base domain %q has no parent domain (need at least 3 labels like 'sub.example.com')",
			baseDomainName)
	}
	return strings.Join(parts[1:], "."), nil
}

func lookupParentZoneID(
	ctx context.Context, exec cmdexec.Executor, managementProfile, region, baseDomainName string,
) (string, error) {
	parentDomain, err := extractParentDomain(baseDomainName)
	if err != nil {
		return "", err
	}

	dnsName := parentDomain + "."

	output, err := exec.MiseOutput(ctx, "aws", "route53", "list-hosted-zones-by-name",
		"--dns-name", parentDomain,
		"--max-items", "1",
		"--profile", managementProfile,
		"--region", region,
		"--output", "json",
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to list hosted zones in management account")
	}

	var result struct {
		HostedZones []struct {
			ID     string `json:"Id"`   //nolint:tagliatelle // AWS API uses PascalCase
			Name   string `json:"Name"` //nolint:tagliatelle // AWS API uses PascalCase
			Config struct {
				PrivateZone bool `json:"PrivateZone"` //nolint:tagliatelle // AWS API uses PascalCase
			} `json:"Config"` //nolint:tagliatelle // AWS API uses PascalCase
		} `json:"HostedZones"` //nolint:tagliatelle // AWS API uses PascalCase
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return "", errors.Wrap(err, "failed to parse hosted zones response")
	}

	for _, zone := range result.HostedZones {
		if zone.Name == dnsName && !zone.Config.PrivateZone {
			zoneID := strings.TrimPrefix(zone.ID, "/hostedzone/")
			return zoneID, nil
		}
	}

	return "", errors.Errorf(
		"no public hosted zone found for %q in management account (profile: %s)",
		parentDomain, managementProfile)
}
