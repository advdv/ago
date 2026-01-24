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

The `Shared` struct holds resources shared across all deployments in a region. Access config values anywhere in the construct tree using scope-based functions:

```go
type Shared struct {
	Bucket     awss3.Bucket
	HostedZone awsroute53.IHostedZone
}

func NewShared(stack awscdk.Stack) *Shared {
	bucket := awss3.NewBucket(stack, jsii.String("SharedBucket"), &awss3.BucketProps{
		Versioned: jsii.Bool(true),
	})
	
	// Access config deep in construct tree - no need to pass *Config explicitly
	zone := awsroute53.HostedZone_FromLookup(stack, jsii.String("Zone"), &awsroute53.HostedZoneProviderProps{
		DomainName: agcdkutil.BaseDomainNamePtr(stack),
	})
	
	return &Shared{Bucket: bucket, HostedZone: zone}
}
```

### 3. Create Deployment Infrastructure

The `Deployment` struct holds resources specific to each deployment (Dev, Stag, Prod):

```go
type Deployment struct{}

func NewDeployment(stack awscdk.Stack, shared *Shared, deploymentIdent string) *Deployment {
	// Access config anywhere in the construct tree
	if agcdkutil.IsPrimaryRegion(stack, *stack.Region()) {
		// Primary region specific setup
	}
	
	// Or get the full Config if needed
	cfg := agcdkutil.ConfigFromScope(stack)
	_ = cfg.AllRegions()
	
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

## API Reference

### Core Functions

| Function | Purpose |
|----------|---------|
| `SetupApp` | Main orchestrator for multi-region, multi-deployment apps |
| `NewConfig` | Reads and validates all context values upfront |
| `StoreConfig` | Store validated Config in construct tree (called by SetupApp) |
| `ConfigFromScope` | Retrieve Config from anywhere in construct tree |
| `NewStackFromConfig` | Creates stack using validated Config |
| `ReproducibleGoBundling` | Lambda bundling options for identical builds |
| `PreserveExport` | Preserve CloudFormation exports during refactoring |

### Scope-Based Convenience Functions

These functions retrieve Config from the construct tree automatically, providing ergonomic access deep in construct trees:

| Function | Purpose |
|----------|---------|
| `IsPrimaryRegion(scope, region)` | Check if region is primary |
| `IsPrimaryRegionStack(scope, stack)` | Check if stack is in primary region |
| `BaseDomainName(scope)` | Get base domain name string |
| `BaseDomainNamePtr(scope)` | Get base domain as jsii pointer |
| `AllRegions(scope)` | Get primary + all secondary regions |
| `RegionIdent(scope, region)` | Get acronym for a region |
| `Qualifier(scope)` | Get CDK qualifier |
| `PrimaryRegion(scope)` | Get primary region |

### Config Methods

When you have a `*Config` reference (via `ConfigFromScope`), these methods are available:

| Method | Purpose |
|--------|---------|
| `Config.Qualifier` | CDK qualifier (max 10 chars) |
| `Config.PrimaryRegion` | Primary AWS region |
| `Config.SecondaryRegions` | List of secondary regions |
| `Config.AllRegions()` | Primary + all secondary regions |
| `Config.RegionIdent(region)` | Get acronym for a region |
| `Config.IsPrimaryRegion(region)` | Check if region is primary |
| `Config.IsPrimaryRegionStack(stack)` | Check if stack is in primary region |
| `Config.BaseDomainName` | Base domain name string |
| `Config.BaseDomainNamePtr()` | Base domain as jsii pointer |
| `Config.Deployments` | All configured deployments |
| `Config.AllowedDeployments()` | Deployments current user can deploy |

### Deprecated Functions

These functions read context without upfront validation. Use the scope-based functions instead:

| Function | Replacement |
|----------|-------------|
| `NewStack` | `NewStackFromConfig` |
| `QualifierFromContext` | `Qualifier(scope)` |
| `RegionAcronymIdentFromContext` | `RegionIdent(scope, region)` |

## Deployment Authorization

The `DeployersGroup` in `AppConfig` controls who can deploy to restricted environments:

- Users in the deployers group can deploy to all environments
- Other users can only deploy to non-restricted environments (e.g., Dev)
- Set `RestrictedDeployments` to environments requiring elevated access

Pass deployer groups via context during deploy:
```bash
cdk deploy --context myapp-deployer-groups="myapp-deployers other-group"
```
