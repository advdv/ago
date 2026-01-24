package agcdkutil

import (
	"fmt"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// QualifierFromContext retrieves the CDK qualifier from context.
// The qualifier must be max 10 characters per AWS CDK limits.
func QualifierFromContext(scope constructs.Construct, prefix string) string {
	qual := StringContext(scope, prefix+"qualifier")
	if len(qual) > 10 { // https://github.com/aws/aws-cdk/pull/10121/files
		panic(fmt.Sprintf("CDK qualifier became too large (>10): '%s', adjust context.", qual))
	}

	return qual
}

// RegionAcronymIdentFromContext gets region-specific identifier from context.
func RegionAcronymIdentFromContext(scope constructs.Construct, prefix, region string) string {
	return StringContext(scope, prefix+"region-ident-"+region)
}

// StringContext retrieves a string context value, panicking if not set.
func StringContext(scope constructs.Construct, key string) string {
	qual, ok := scope.Node().GetContext(jsii.String(key)).(string)
	if !ok {
		panic("invalid '" + key + "', is it set?")
	}

	return qual
}

// BaseDomainName retrieves the base domain name from context.
func BaseDomainName(scope constructs.Construct, prefix string) *string {
	return jsii.String(StringContext(scope, prefix+"base-domain-name"))
}
