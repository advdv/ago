---
name: setting-up-cdk-app
description: Sets up AWS CDK applications in Go using agcdkutil package. Use when creating CDK infrastructure, multi-region stacks, or deployment pipelines.
---

# Setting Up CDK Applications with agcdkutil

Creates multi-region, multi-deployment AWS CDK applications using the `github.com/advdv/ago/agcdkutil` package.

## When to Use

- Creating a new CDK application in Go
- Setting up multi-region infrastructure
- Configuring deployment pipelines with Dev/Stag/Prod environments
- Building Lambda functions with reproducible builds

## Quick Start

### 1. Create the CDK Entry Point

Create `cdk.go` in your infrastructure directory:

```go
package main

import (
	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func main() {
	defer jsii.Close()
	app := awscdk.NewApp(nil)

	agcdkutil.SetupApp(app, agcdkutil.AppConfig{
		Prefix:                "myapp-",
		DeployersGroup:        "myapp-deployers",
		RestrictedDeployments: []string{"Stag", "Prod"},
	},
		func(stack awscdk.Stack) *Shared { return NewShared(stack) },
		func(stack awscdk.Stack, shared *Shared, deploymentIdent string) {
			NewDeployment(stack, shared, deploymentIdent)
		},
	)

	app.Synth(nil)
}
```

### 2. Create Shared Infrastructure

The `Shared` struct holds resources shared across all deployments in a region:

```go
type Shared struct {
	Bucket     awss3.Bucket
	HostedZone awsroute53.IHostedZone
}

func NewShared(stack awscdk.Stack) *Shared {
	bucket := awss3.NewBucket(stack, jsii.String("SharedBucket"), &awss3.BucketProps{
		Versioned: jsii.Bool(true),
	})
	
	zone := awsroute53.HostedZone_FromLookup(stack, jsii.String("Zone"), &awsroute53.HostedZoneProviderProps{
		DomainName: agcdkutil.BaseDomainName(stack, "myapp-"),
	})
	
	return &Shared{Bucket: bucket, HostedZone: zone}
}
```

### 3. Create Deployment Infrastructure

The `Deployment` struct holds resources specific to each deployment (Dev, Stag, Prod):

```go
type Deployment struct{}

func NewDeployment(stack awscdk.Stack, shared *Shared, deploymentIdent string) *Deployment {
	// Use shared resources and deploymentIdent to create deployment-specific infra
	// Example: create an API Gateway that uses the shared bucket
	return &Deployment{}
}
```

### 4. Configure cdk.json

Create `cdk.json` with context values matching your prefix:

```json
{
  "app": "go mod download && go run cdk.go",
  "context": {
    "myapp-qualifier": "myapp",
    "myapp-primary-region": "us-east-1",
    "myapp-secondary-regions": ["eu-west-1"],
    "myapp-region-ident-us-east-1": "use1",
    "myapp-region-ident-eu-west-1": "euw1",
    "myapp-deployments": ["Dev", "Stag", "Prod"],
    "myapp-base-domain-name": "example.com"
  }
}
```

## Stack Naming and Dependencies

`SetupApp` creates stacks with automatic naming and dependency management:

| Stack Type | Name Pattern | Depends On |
|------------|--------------|------------|
| Primary Shared | `{qualifier}{regionAcronym}Shared` | - |
| Secondary Shared | `{qualifier}{regionAcronym}Shared` | Primary Shared |
| Primary Deployment | `{qualifier}{regionAcronym}{Deployment}` | Primary Shared |
| Secondary Deployment | `{qualifier}{regionAcronym}{Deployment}` | Primary Deployment |

Example with qualifier `myapp` and region acronym `use1`:
- `myappUse1Shared`
- `myappUse1Dev`
- `myappUse1Prod`

## Lambda Bundling

Use `ReproducibleGoBundling()` for Lambda functions to ensure identical builds:

```go
awscdklambdagoalpha.NewGoFunction(stack, jsii.String("Handler"), &awscdklambdagoalpha.GoFunctionProps{
	Entry:    jsii.String("./cmd/handler"),
	Bundling: agcdkutil.ReproducibleGoBundling(),
})
```

## Utility Functions

| Function | Purpose |
|----------|---------|
| `SetupApp` | Main orchestrator for multi-region, multi-deployment apps |
| `NewStack` | Creates stack with qualifier and region naming |
| `ReproducibleGoBundling` | Lambda bundling options for identical builds |
| `PrimaryRegion` | Get primary region from context |
| `SecondaryRegions` | Get secondary regions from context |
| `IsPrimaryRegion` | Check if current stack is in primary region |
| `AllowedDeployments` | Get deployments current user can deploy |
| `BaseDomainName` | Get base domain name from context |
| `PreserveExport` | Preserve CloudFormation exports during refactoring |

## Deployment Authorization

The `DeployersGroup` in `AppConfig` controls who can deploy to restricted environments:

- Users in the deployers group can deploy to all environments
- Other users can only deploy to non-restricted environments (e.g., Dev)
- Set `RestrictedDeployments` to environments requiring elevated access

Pass deployer groups via context during deploy:
```bash
cdk deploy --context myapp-deployer-groups="myapp-deployers other-group"
```
