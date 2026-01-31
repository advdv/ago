// Package agcdkparams provides utilities for storing and retrieving CDK construct
// values across AWS regions using AWS Systems Manager Parameter Store.
//
// This package enables cross-region resource sharing in multi-region CDK deployments:
//   - Primary region: Creates resources and stores identifiers in SSM Parameter Store
//   - Secondary regions: Retrieves stored values to reference existing resources
package agcdkparams

import (
	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/aws-cdk-go/awscdk/v2/customresources"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// ParameterName generates a hierarchical SSM parameter path.
// Returns a path like /{qualifier}/{namespace}/{name}.
func ParameterName(scope constructs.Construct, namespace string, name string) *string {
	qual := agcdkutil.Qualifier(scope)
	return jsii.Sprintf("/%s/%s/%s", qual, namespace, name)
}

// Store creates and stores a parameter in AWS SSM Parameter Store.
// Use this in the primary region to persist values for cross-region access.
func Store(scope constructs.Construct, id string, namespace string, name string, value *string) {
	awsssm.NewStringParameter(scope, jsii.String(id),
		&awsssm.StringParameterProps{
			ParameterName: ParameterName(scope, namespace, name),
			StringValue:   value,
		})
}

// Lookup retrieves a parameter stored in the primary region using a custom resource.
// Use this in secondary regions to access values created in the primary region.
// The physicalID should be a stable identifier for the custom resource (e.g., "user-pool-id-lookup").
func Lookup(scope constructs.Construct, id string, namespace string, name string, physicalID string) *string {
	lookup := customresources.NewAwsCustomResource(scope, jsii.String(id),
		&customresources.AwsCustomResourceProps{
			OnCreate: &customresources.AwsSdkCall{
				Service: jsii.String("SSM"),
				Action:  jsii.String("getParameter"),
				Parameters: map[string]any{
					"Name": ParameterName(scope, namespace, name),
				},
				Region:             jsii.String(agcdkutil.PrimaryRegion(scope)),
				PhysicalResourceId: customresources.PhysicalResourceId_Of(jsii.String(physicalID)),
			},
			Policy: customresources.AwsCustomResourcePolicy_FromSdkCalls(&customresources.SdkCallsPolicyOptions{
				Resources: customresources.AwsCustomResourcePolicy_ANY_RESOURCE(),
			}),
		})
	return lookup.GetResponseField(jsii.String("Parameter.Value"))
}
