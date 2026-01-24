package agcdkutil

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// AllRegions returns the primary region plus all secondary regions.
func AllRegions(scope constructs.Construct, prefix string) []string {
	return append([]string{PrimaryRegion(scope, prefix)}, SecondaryRegions(scope, prefix)...)
}

// IsPrimaryRegion checks if the current scope is in the primary region.
func IsPrimaryRegion(scope constructs.Construct, prefix string) bool {
	return *awscdk.Stack_Of(scope).Region() == PrimaryRegion(scope, prefix)
}

// PrimaryRegion gets the primary region from context.
func PrimaryRegion(scope constructs.Construct, prefix string) string {
	return StringContext(scope, prefix+"primary-region")
}

// SecondaryRegions gets the array of secondary regions from context.
func SecondaryRegions(scope constructs.Construct, prefix string) []string {
	key := prefix + "secondary-regions"
	val := scope.Node().GetContext(jsii.String(key))
	if val == nil {
		panic("invalid '" + key + "', is it set?")
	}

	slice, ok := val.([]any)
	if !ok {
		panic("invalid '" + key + "', expected array")
	}

	regions := make([]string, 0, len(slice))
	for _, v := range slice {
		s, ok := v.(string)
		if !ok {
			panic("invalid '" + key + "', expected array of strings")
		}
		regions = append(regions, s)
	}

	return regions
}
