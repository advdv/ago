// Package agcdkutil provides utilities for AWS CDK applications in Go.
//
// # Quick Start
//
// Use [SetupApp] to configure a multi-region, multi-deployment CDK application:
//
//	func main() {
//	    defer jsii.Close()
//	    app := awscdk.NewApp(nil)
//
//	    agcdkutil.SetupApp(app, agcdkutil.AppConfig{
//	        Prefix:                "myapp-",
//	        DeployersGroup:        "myapp-deployers",
//	        RestrictedDeployments: []string{"Stag", "Prod"},
//	    },
//	        func(stack awscdk.Stack) *Shared { return NewShared(stack) },
//	        func(stack awscdk.Stack, shared *Shared, deploymentIdent string) {
//	            NewDeployment(stack, shared, deploymentIdent)
//	        },
//	    )
//
//	    app.Synth(nil)
//	}
//
// # CDK Context Configuration
//
// The package reads configuration from CDK context (cdk.json). With prefix "myapp-":
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
//
// # Stack Creation Order
//
// [SetupApp] creates stacks with the following dependency order:
//  1. Primary shared stack
//  2. Secondary shared stacks (depend on primary shared)
//  3. Primary deployment stacks (depend on primary shared)
//  4. Secondary deployment stacks (depend on primary deployment)
//
// # Features
//
//   - [SetupApp]: Multi-region, multi-deployment app orchestration
//   - [NewStack]: Stack creation with qualifier and region naming
//   - [ReproducibleGoBundling]: Lambda bundling for identical builds
//   - [AllowedDeployments]: Role-based deployment authorization
//   - [PreserveExport]: CloudFormation export preservation
package agcdkutil
