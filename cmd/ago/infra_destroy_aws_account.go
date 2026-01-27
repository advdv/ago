package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

// removeAWSProfile and removeProfileFromFile are defined in infra_cdk_bootstrap.go

func destroyAWSAccountCmd() *cli.Command {
	return &cli.Command{
		Name:  "destroy-aws-account",
		Usage: "Close and remove an AWS account from the organization",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "management-profile",
				Usage:    "AWS profile for the management account",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "confirm",
				Usage:    "Confirm destruction by specifying the project name",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "region",
				Usage: "AWS region for the CloudFormation stack",
				Value: "eu-central-1",
			},
		},
		Action: config.RunWithConfig(runDestroyProjectAccount),
	}
}

type destroyAccountOptions struct {
	ProjectName       string
	ManagementProfile string
	Region            string
	ConfirmName       string
	Output            io.Writer
}

func runDestroyProjectAccount(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	projectName := filepath.Base(cfg.ProjectDir)
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	return doDestroyProjectAccount(ctx, cfg, destroyAccountOptions{
		ProjectName:       projectName,
		ManagementProfile: cmd.String("management-profile"),
		Region:            cmd.String("region"),
		ConfirmName:       cmd.String("confirm"),
		Output:            os.Stdout,
	})
}

func doDestroyProjectAccount(ctx context.Context, cfg config.Config, opts destroyAccountOptions) error {
	if opts.ConfirmName != opts.ProjectName {
		return errors.Errorf(
			"confirmation name %q does not match project name %q", opts.ConfirmName, opts.ProjectName)
	}

	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.Output)
	stackName := "ago-account-" + opts.ProjectName

	accountID, err := getAccountStackOutput(ctx, exec, createAccountOptions{
		ManagementProfile: opts.ManagementProfile,
		Region:            opts.Region,
	}, stackName, "AccountId")
	if err != nil {
		return errors.Wrap(err, "failed to get account ID from stack")
	}

	writeOutputf(opts.Output, "Closing AWS account %s...\n", accountID)

	if err := closeAWSAccount(ctx, exec, opts, accountID); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Deleting CloudFormation stack %q...\n", stackName)

	if err := deleteAccountStack(ctx, exec, opts, stackName); err != nil {
		return err
	}

	profileName := opts.ProjectName + "-admin"

	writeOutputf(opts.Output, "Removing AWS profile %q from ~/.aws/config and ~/.aws/credentials...\n", profileName)

	if err := removeAWSProfile(profileName); err != nil {
		return err
	}

	writeOutputf(opts.Output, "Account %s closed and removed successfully.\n", accountID)
	writeOutputf(opts.Output, "Note: The account enters a 90-day post-closure period before permanent deletion.\n")

	return nil
}

func closeAWSAccount(
	ctx context.Context, exec cmdexec.Executor, opts destroyAccountOptions, accountID string,
) error {
	return exec.Mise(ctx, "aws", "organizations", "close-account",
		"--account-id", accountID,
		"--profile", opts.ManagementProfile,
	)
}

func deleteAccountStack(
	ctx context.Context, exec cmdexec.Executor, opts destroyAccountOptions, stackName string,
) error {
	if err := exec.Mise(ctx, "aws", "cloudformation", "delete-stack",
		"--stack-name", stackName,
		"--region", opts.Region,
		"--profile", opts.ManagementProfile,
	); err != nil {
		return errors.Wrap(err, "failed to delete stack")
	}

	return exec.Mise(ctx, "aws", "cloudformation", "wait", "stack-delete-complete",
		"--stack-name", stackName,
		"--region", opts.Region,
		"--profile", opts.ManagementProfile,
	)
}
