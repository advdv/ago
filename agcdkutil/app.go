package agcdkutil

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

// SharedConstructor creates shared infrastructure in a given stack.
// It returns the shared construct that will be passed to deployment constructors.
type SharedConstructor[S any] func(stack awscdk.Stack) S

// DeploymentConstructor creates deployment-specific infrastructure in a given stack.
// It receives the shared construct from the same region and the deployment identifier.
type DeploymentConstructor[S any] func(stack awscdk.Stack, shared S, deploymentIdent string)

// AppConfig configures the CDK app setup.
type AppConfig struct {
	// Prefix for context keys (e.g., "myapp-" for "myapp-qualifier", "myapp-primary-region", etc.)
	Prefix string
	// DeployersGroup is the IAM group that can deploy to all environments.
	DeployersGroup string
	// RestrictedDeployments are deployment identifiers that require DeployersGroup membership.
	RestrictedDeployments []string
}

// SetupApp configures a CDK app with multi-region, multi-deployment stacks.
//
// It creates:
//  1. A primary shared stack using the SharedConstructor
//  2. Secondary shared stacks for each secondary region (dependent on primary)
//  3. Deployment stacks for each allowed deployment in the primary region
//  4. Secondary deployment stacks for each secondary region (dependent on primary deployment)
//
// The type parameter S represents the shared construct type returned by SharedConstructor.
func SetupApp[S any](
	app awscdk.App,
	cfg AppConfig,
	newShared SharedConstructor[S],
	newDeployment DeploymentConstructor[S],
) {
	// Create shared primary region stack first
	primaryRegion := PrimaryRegion(app, cfg.Prefix)
	primarySharedStack := NewStack(app, cfg.Prefix, primaryRegion)
	primaryShared := newShared(primarySharedStack)

	// Create secondary shared region stacks with dependency on primary
	secondaryShared := map[string]S{}
	for _, region := range SecondaryRegions(app, cfg.Prefix) {
		secondarySharedStack := NewStack(app, cfg.Prefix, region)
		secondaryShared[region] = newShared(secondarySharedStack)
		secondarySharedStack.AddDependency(primarySharedStack, jsii.String("Primary region must deploy first"))
	}

	// Create stacks for each allowed deployment
	allowedDeployments := AllowedDeployments(app, cfg.Prefix, cfg.DeployersGroup, cfg.RestrictedDeployments)
	for _, deploymentIdent := range allowedDeployments {
		primaryDeploymentStack := NewStack(app, cfg.Prefix, primaryRegion, deploymentIdent)
		newDeployment(primaryDeploymentStack, primaryShared, deploymentIdent)
		primaryDeploymentStack.AddDependency(primarySharedStack,
			jsii.String("Primary shared stack must deploy first"))

		// Secondary region stacks for each deployment
		for _, region := range SecondaryRegions(app, cfg.Prefix) {
			secondaryDeploymentStack := NewStack(app, cfg.Prefix, region, deploymentIdent)
			newDeployment(secondaryDeploymentStack, secondaryShared[region], deploymentIdent)
			secondaryDeploymentStack.AddDependency(primaryDeploymentStack,
				jsii.String("Primary region deployment must deploy first"))
		}
	}
}
