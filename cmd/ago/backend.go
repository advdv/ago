package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/cockroachdb/errors"
	"github.com/urfave/cli/v3"
)

func backendCmd() *cli.Command {
	return &cli.Command{
		Name:  "backend",
		Usage: "Backend development commands",
		Commands: []*cli.Command{
			{
				Name:  "build-and-push",
				Usage: "Build and push backend container images to ECR using depot",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "deployment",
						Usage: "Deployment identifier (e.g., dev, stag, prod)",
						Value: "dev",
					},
					&cli.StringFlag{
						Name:  "profile",
						Usage: "AWS profile for ECR access (defaults to cdk.json profile)",
					},
					&cli.StringFlag{
						Name:  "region",
						Usage: "AWS region (defaults to primary region from context)",
					},
					&cli.StringFlag{
						Name:  "stack-name",
						Usage: "CloudFormation stack name containing the ECR repository (defaults to {qualifier}-Shared-{region-ident})",
					},
					&cli.StringFlag{
						Name:  "platform",
						Usage: "Target platform for the build",
						Value: "linux/arm64",
					},
				},
				Action: config.RunWithConfig(runBackendBuildAndPush),
			},
		},
	}
}

func runBackendBuildAndPush(ctx context.Context, cmd *cli.Command, cfg config.Config) error {
	return doBackendBuildAndPush(ctx, cfg, backendBuildAndPushOptions{
		Deployment: cmd.String("deployment"),
		Profile:    cmd.String("profile"),
		Region:     cmd.String("region"),
		StackName:  cmd.String("stack-name"),
		Platform:   cmd.String("platform"),
		Output:     os.Stdout,
		ErrOut:     os.Stderr,
	})
}

type backendBuildAndPushOptions struct {
	Deployment string
	Profile    string
	Region     string
	StackName  string
	Platform   string
	Output     io.Writer
	ErrOut     io.Writer
}

func doBackendBuildAndPush(ctx context.Context, cfg config.Config, opts backendBuildAndPushOptions) error {
	exec := cmdexec.New(cfg).WithOutput(opts.Output, opts.ErrOut)
	backendExec := exec.InSubdir("backend")

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

	repoURI, err := getStackOutputValue(ctx, exec, profile, region, stackName, "RepositoryURI")
	if err != nil {
		return errors.Wrap(err, "failed to get ECR repository URI from stack outputs")
	}

	if err := loginToECR(ctx, exec, profile, region); err != nil {
		return err
	}

	cmdDir := filepath.Join(backendExec.Dir(), "cmd")
	entries, err := os.ReadDir(cmdDir)
	if err != nil {
		return errors.Wrap(err, "failed to read backend/cmd directory")
	}

	var cmdNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			cmdNames = append(cmdNames, entry.Name())
		}
	}

	if len(cmdNames) == 0 {
		return errors.New("no commands found in backend/cmd")
	}

	for _, cmdName := range cmdNames {
		writeOutputf(opts.Output, "\nBuilding %s...\n", cmdName)

		tag, err := buildAndPushImage(ctx, backendExec, buildImageOptions{
			CmdName:    cmdName,
			Deployment: opts.Deployment,
			RepoURI:    repoURI,
			Platform:   opts.Platform,
		})
		if err != nil {
			return errors.Wrapf(err, "failed to build and push %s", cmdName)
		}

		writeOutputf(opts.Output, "Pushed %s:%s\n", repoURI, tag)
	}

	return nil
}

type buildImageOptions struct {
	CmdName    string
	Deployment string
	RepoURI    string
	Platform   string
}

func buildAndPushImage(ctx context.Context, exec cmdexec.Executor, opts buildImageOptions) (string, error) {
	metadataFile, err := os.CreateTemp("", "depot-metadata-*.json")
	if err != nil {
		return "", errors.Wrap(err, "failed to create metadata temp file")
	}
	metadataPath := metadataFile.Name()
	metadataFile.Close()
	defer os.Remove(metadataPath)

	if err := exec.Mise(ctx, "depot", "build",
		"--file", "Dockerfile",
		"--build-arg", "CMD_NAME="+opts.CmdName,
		"--platform", opts.Platform,
		"--metadata-file", metadataPath,
		"--save",
		".",
	); err != nil {
		return "", errors.Wrap(err, "depot build failed")
	}

	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", errors.Wrap(err, "failed to read depot metadata file")
	}

	var depotMeta struct {
		Digest  string `json:"containerimage.digest"` //nolint:tagliatelle // depot API uses dot notation
		BuildID string `json:"depot.build.id"`        //nolint:tagliatelle // depot API uses dot notation
	}
	if err := json.Unmarshal(metadata, &depotMeta); err != nil {
		return "", errors.Wrap(err, "failed to parse depot metadata")
	}

	if depotMeta.Digest == "" {
		return "", errors.New("no digest found in depot metadata")
	}

	shortDigest := extractShortDigest(depotMeta.Digest)
	tag := fmt.Sprintf("%s-%s-%s", opts.CmdName, opts.Deployment, shortDigest)
	fullImageRef := fmt.Sprintf("%s:%s", opts.RepoURI, tag)

	if err := exec.Mise(ctx, "depot", "push",
		"--tag", fullImageRef,
		depotMeta.BuildID,
	); err != nil {
		return "", errors.Wrap(err, "depot push failed")
	}

	return tag, nil
}

func extractShortDigest(digest string) string {
	digest = strings.TrimPrefix(digest, "sha256:")

	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

func loginToECR(ctx context.Context, exec cmdexec.Executor, profile, region string) error {
	password, err := exec.MiseOutput(ctx, "aws", "ecr", "get-login-password",
		"--profile", profile,
		"--region", region,
	)
	if err != nil {
		return errors.Wrap(err, "failed to get ECR login password")
	}

	accountID, err := getAWSAccountID(ctx, exec, profile)
	if err != nil {
		return err
	}

	registryURL := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region)

	if err := exec.RunWithStdin(ctx, strings.NewReader(password), "docker", "login",
		"--username", "AWS",
		"--password-stdin",
		registryURL,
	); err != nil {
		return errors.Wrap(err, "docker login to ECR failed")
	}

	return nil
}

func getAWSAccountID(ctx context.Context, exec cmdexec.Executor, profile string) (string, error) {
	output, err := exec.MiseOutput(ctx, "aws", "sts", "get-caller-identity",
		"--profile", profile,
		"--query", "Account",
		"--output", "text",
	)
	if err != nil {
		return "", errors.Wrap(err, "failed to get AWS account ID")
	}

	return strings.TrimSpace(output), nil
}
