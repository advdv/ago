package agcdkutil_test

import (
	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
)

// Shared represents the shared infrastructure created once per region.
// It holds resources that are shared across all deployments in that region.
type Shared struct {
	Bucket awss3.Bucket
}

// Deployment represents deployment-specific infrastructure.
// Each deployment (Dev, Stag, Prod) gets its own instance.
type Deployment struct {
	// deployment-specific resources
}

// NewShared creates shared infrastructure in the given stack.
func NewShared(stack awscdk.Stack) *Shared {
	bucket := awss3.NewBucket(stack, jsii.String("SharedBucket"), &awss3.BucketProps{
		Versioned: jsii.Bool(true),
	})

	// Access config deep in construct tree without passing *Config explicitly
	if agcdkutil.IsPrimaryRegion(stack, *stack.Region()) {
		// Primary region specific setup
		_ = agcdkutil.BaseDomainName(stack)
	}

	return &Shared{Bucket: bucket}
}

// NewDeployment creates deployment-specific infrastructure.
func NewDeployment(stack awscdk.Stack, shared *Shared, deploymentIdent string) *Deployment {
	// Use shared.Bucket or other shared resources here
	_ = shared.Bucket
	_ = deploymentIdent

	// Can also get the full Config if needed
	cfg := agcdkutil.ConfigFromScope(stack)
	_ = cfg.AllRegions()

	return &Deployment{}
}

// Example_setupApp demonstrates how to use SetupApp to configure a multi-region,
// multi-deployment CDK application.
//
// The cdk.json context should include:
//
//	{
//	  "myapp-qualifier": "myapp",
//	  "myapp-primary-region": "us-east-1",
//	  "myapp-secondary-regions": ["eu-west-1"],
//	  "myapp-region-ident-us-east-1": "use1",
//	  "myapp-region-ident-eu-west-1": "euw1",
//	  "myapp-deployments": ["Dev", "Stag", "Prod"],
//	  "myapp-deployer-groups": "myapp-deployers",
//	  "myapp-base-domain-name": "example.com"
//	}
func Example_setupApp() {
	defer jsii.Close()

	ctx := map[string]any{
		"myapp-qualifier":              "myapp",
		"myapp-primary-region":         "us-east-1",
		"myapp-secondary-regions":      []any{"eu-west-1"},
		"myapp-region-ident-us-east-1": "use1",
		"myapp-region-ident-eu-west-1": "euw1",
		"myapp-deployments":            []any{"Dev", "Stag", "Prod"},
		"myapp-deployer-groups":        "myapp-deployers",
		"myapp-base-domain-name":       "example.com",
	}

	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &ctx,
	})

	agcdkutil.SetupApp(app, agcdkutil.AppConfig{
		Prefix:                "myapp-",
		DeployersGroup:        "myapp-deployers",
		RestrictedDeployments: []string{"Stag", "Prod"},
	},
		NewShared,
		func(stack awscdk.Stack, shared *Shared, deploymentIdent string) {
			NewDeployment(stack, shared, deploymentIdent)
		},
	)
	// Output:
}
