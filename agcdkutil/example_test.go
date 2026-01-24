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
	return &Shared{Bucket: bucket}
}

// NewDeployment creates deployment-specific infrastructure.
func NewDeployment(stack awscdk.Stack, shared *Shared, deploymentIdent string) *Deployment {
	// Use shared.Bucket or other shared resources here
	_ = shared.Bucket
	_ = deploymentIdent
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
//	  "myapp-deployer-groups": "myapp-deployers"
//	}
func Example_setupApp() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	agcdkutil.SetupApp(app, agcdkutil.AppConfig{
		// Prefix for all context keys (e.g., "myapp-qualifier", "myapp-primary-region")
		Prefix: "myapp-",
		// IAM group that can deploy to all environments including restricted ones
		DeployersGroup: "myapp-deployers",
		// Deployments that require DeployersGroup membership
		RestrictedDeployments: []string{"Stag", "Prod"},
	},
		// SharedConstructor: called once per region to create shared infrastructure
		func(stack awscdk.Stack) *Shared {
			return NewShared(stack)
		},
		// DeploymentConstructor: called for each deployment in each region
		func(stack awscdk.Stack, shared *Shared, deploymentIdent string) {
			NewDeployment(stack, shared, deploymentIdent)
		},
	)

	app.Synth(nil)
}
