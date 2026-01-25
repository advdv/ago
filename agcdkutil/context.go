package agcdkutil

import (
	"fmt"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// QualifierFromContext retrieves the CDK qualifier from context.
// The qualifier must be max 10 characters per AWS CDK limits.
//
// Deprecated: Use NewConfig and Config.Qualifier instead for upfront validation.
func QualifierFromContext(scope constructs.Construct, prefix string) string {
	qual := stringContext(scope, prefix+"qualifier")
	if len(qual) > 10 { // https://github.com/aws/aws-cdk/pull/10121/files
		panic(fmt.Sprintf("CDK qualifier became too large (>10): '%s', adjust context.", qual))
	}

	return qual
}

// RegionAcronymIdentFromContext returns the 4-character identifier for a region.
//
// Deprecated: Use RegionIdentFor(region) directly instead.
func RegionAcronymIdentFromContext(_ constructs.Construct, _, region string) string {
	return RegionIdentFor(region)
}

// stringContext retrieves a string context value, panicking if not set.
func stringContext(scope constructs.Construct, key string) string {
	qual, ok := scope.Node().GetContext(jsii.String(key)).(string)
	if !ok {
		panic("invalid '" + key + "', is it set?")
	}

	return qual
}
