package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/advdv/ago/cmd/ago/internal/cmdexec"
	"github.com/advdv/ago/cmd/ago/internal/config"
	"github.com/advdv/ago/cmd/ago/internal/dirhash"
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
			{
				Name:  "hash",
				Usage: "Compute content-based hash of backend source (respects .dockerignore)",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "debug",
						Usage: "Print visited files to stderr",
					},
				},
				Action: config.RunWithConfig(runBackendHash),
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

	repoName := extractRepoName(repoURI)

	for _, cmdName := range cmdNames {
		writeOutputf(opts.Output, "\nBuilding %s...\n", cmdName)

		tag, err := buildAndPushImage(ctx, backendExec, buildImageOptions{
			CmdName:    cmdName,
			Deployment: opts.Deployment,
			RepoURI:    repoURI,
			RepoName:   repoName,
			Platform:   opts.Platform,
			Profile:    profile,
			Region:     region,
			RootExec:   exec,
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
	RepoName   string
	Platform   string
	Profile    string
	Region     string
	RootExec   cmdexec.Executor
}

func buildAndPushImage(ctx context.Context, exec cmdexec.Executor, opts buildImageOptions) (string, error) {
	sourceHash, err := getGitTreeHash(ctx, opts.RootExec)
	if err != nil {
		return "", err
	}

	tag := fmt.Sprintf("%s-%s-%s", opts.CmdName, opts.Deployment, sourceHash)
	fullImageRef := fmt.Sprintf("%s:%s", opts.RepoURI, tag)

	exists, err := ecrTagExists(ctx, exec, opts.Profile, opts.Region, opts.RepoName, tag)
	if err != nil {
		return "", errors.Wrap(err, "failed to check if tag exists")
	}

	if exists {
		return tag + " (already exists)", nil
	}

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
		Digest string `json:"containerimage.digest"` //nolint:tagliatelle // depot API uses dot notation
		Build  struct {
			BuildID string `json:"buildID"` //nolint:tagliatelle // depot API uses camelCase
		} `json:"depot.build"` //nolint:tagliatelle // depot API uses dot notation
	}
	if err := json.Unmarshal(metadata, &depotMeta); err != nil {
		return "", errors.Wrap(err, "failed to parse depot metadata")
	}

	if depotMeta.Digest == "" {
		return "", errors.New("no digest found in depot metadata")
	}

	if depotMeta.Build.BuildID == "" {
		return "", errors.New("no build ID found in depot metadata")
	}

	if err := exec.Mise(ctx, "depot", "push",
		"--tag", fullImageRef,
		depotMeta.Build.BuildID,
	); err != nil {
		return "", errors.Wrap(err, "depot push failed")
	}

	return tag, nil
}

func getGitTreeHash(ctx context.Context, exec cmdexec.Executor) (string, error) {
	output, err := exec.Output(ctx, "git", "rev-parse", "HEAD^{tree}")
	if err != nil {
		return "", errors.Wrap(err, "failed to get git tree hash")
	}
	treeHash := strings.TrimSpace(output)

	isDirty, err := isGitDirty(ctx, exec)
	if err != nil {
		return "", err
	}

	if isDirty {
		diffHash, err := getDiffHash(ctx, exec)
		if err != nil {
			return "", err
		}
		combined := treeHash + "-" + diffHash
		if len(combined) > 12 {
			return combined[:12], nil
		}
		return combined, nil
	}

	if len(treeHash) > 12 {
		return treeHash[:12], nil
	}
	return treeHash, nil
}

func isGitDirty(ctx context.Context, exec cmdexec.Executor) (bool, error) {
	output, err := exec.Output(ctx, "git", "status", "--porcelain")
	if err != nil {
		return false, errors.Wrap(err, "failed to check git status")
	}
	return strings.TrimSpace(output) != "", nil
}

func getDiffHash(ctx context.Context, exec cmdexec.Executor) (string, error) {
	diff, err := exec.Output(ctx, "git", "diff", "--no-ext-diff", "HEAD")
	if err != nil {
		return "", errors.Wrap(err, "failed to get git diff")
	}
	untrackedDiff, _ := exec.Output(ctx, "git", "ls-files", "--others", "--exclude-standard")
	combined := diff + untrackedDiff

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(combined)))
	if len(hash) > 8 {
		return hash[:8], nil
	}
	return hash, nil
}

func extractRepoName(repoURI string) string {
	parts := strings.Split(repoURI, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return repoURI
}

func ecrTagExists(ctx context.Context, exec cmdexec.Executor, profile, region, repoName, tag string) (bool, error) {
	_, err := exec.MiseOutput(ctx, "aws", "ecr", "describe-images",
		"--profile", profile,
		"--region", region,
		"--repository-name", repoName,
		"--image-ids", fmt.Sprintf("imageTag=%s", tag),
		"--query", "imageDetails[0].imageTags",
		"--output", "text",
	)
	if err != nil {
		if strings.Contains(err.Error(), "ImageNotFoundException") {
			return false, nil
		}
		return false, nil
	}
	return true, nil
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

func runBackendHash(_ context.Context, cmd *cli.Command, cfg config.Config) error {
	backendDir := filepath.Join(cfg.ProjectDir, "backend")

	opts := []dirhash.Option{
		dirhash.WithAlwaysInclude("Dockerfile", ".dockerignore"),
	}

	if cmd.Bool("debug") {
		opts = append(opts, dirhash.WithLogger(&dirhash.DebugLogger{W: os.Stderr}))
	}

	h := dirhash.New(opts...)
	hash, err := h.Hash(backendDir, ".dockerignore")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, hash)
	return nil
}
