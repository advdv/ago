---
name: setting-up-projects
description: "Initializes a new project with required tooling and infrastructure scaffolding. Use when setting up a project from scratch or bootstrapping a new codebase."
---

Sets up a new project with tooling and infrastructure using mise.

# Prerequisites
Make sure the following is pre-setup, if not ask the user to do so:
- mise must be installed and in PATH: https://mise.jdx.dev 
- a mise.toml must be present, amp (ampcode.com) must be installed using mise, if not ask the user to do so

# Behavior
- When asked to setup a project, do not DO anything else except follow the steps below. 
- If you want to do things that are not in the steps below, you MUST ask the user for permission.
- It might be that everything is already setup, in that case: do not do anything.

# Steps
Perform the following steps. For each step first check if the step was already performed. If so, skip it but
make sure that steps still need to be performed can be performed still.

1. Use mise to install Go: `mise u go`
2. Use `go mod init` to init the main module
3. Use mise to install Node.js 22 (and npm): `mise u node@22`
4. Use mise to install the AWS CDK toolkit: `mise u npm:aws-cdk`
5. Use mise to install the AWS CLI: `mise u aws-cli`

## Setup infrastructure-as-code using AWS CDK
6. Use the AWS CDK cli to initialize a Go CDK project in the `/infra/cdk/cdk` directory (use `cdk init` with appropriate options for a Go app)
7. Make sure it is a dedicated go module with its own go.mod
8. Clean up the CDK project by removing:
    - the _test file that was created as part of the setup
    - the README.md that was created as part of the setup
9. Use the "setting-up-cdk-app" skill to replace the boilerpalte CDK code. 
    - If no existing "Shared" and "Deployment" structs exist, create empty ones in /infra/cdk
