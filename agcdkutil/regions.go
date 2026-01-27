package agcdkutil

import (
	"slices"
	"strings"
)

// RegionIdents maps AWS region codes to 4-character identifiers for CDK construct IDs.
// All identifiers are exactly 4 characters: 2-letter geo + 1-letter direction + 1-digit number.
var RegionIdents = map[string]string{
	"us-east-1":     "Use1",
	"us-east-2":     "Use2",
	"us-west-1":     "Usw1",
	"us-west-2":     "Usw2",
	"us-gov-east-1": "Uge1",
	"us-gov-west-1": "Ugw1",

	"eu-west-1":      "Euw1",
	"eu-west-2":      "Euw2",
	"eu-west-3":      "Euw3",
	"eu-central-1":   "Euc1",
	"eu-central-2":   "Euc2",
	"eu-north-1":     "Eun1",
	"eu-south-1":     "Eus1",
	"eu-south-2":     "Eus2",
	"eusc-de-east-1": "Ede1",

	"ap-east-1":      "Ape1",
	"ap-south-1":     "Aps1",
	"ap-south-2":     "Aps2",
	"ap-northeast-1": "Apn1",
	"ap-northeast-2": "Apn2",
	"ap-northeast-3": "Apn3",
	"ap-southeast-1": "Ase1",
	"ap-southeast-2": "Ase2",
	"ap-southeast-3": "Ase3",
	"ap-southeast-4": "Ase4",
	"ap-southeast-5": "Ase5",

	"sa-east-1": "Sae1",

	"me-south-1":   "Mes1",
	"me-central-1": "Mec1",

	"af-south-1": "Afs1",

	"ca-central-1": "Cac1",
	"ca-west-1":    "Caw1",

	"il-central-1": "Ilc1",

	"cn-north-1":     "Cnn1",
	"cn-northwest-1": "Cnw1",
}

// RegionIdentFor returns the 4-character identifier for an AWS region.
// It panics if the region is unknown. Use IsKnownRegion to check first if needed.
func RegionIdentFor(region string) string {
	ident, ok := RegionIdents[region]
	if !ok {
		panic("unknown AWS region: " + region + ". Please add it to agcdkutil.RegionIdents")
	}
	return ident
}

// IsKnownRegion returns true if the region has a known identifier.
func IsKnownRegion(region string) bool {
	_, ok := RegionIdents[region]
	return ok
}

// RegionIdentLower returns the lowercase version of the region identifier.
// This is useful for resource naming where lowercase is preferred.
func RegionIdentLower(region string) string {
	return strings.ToLower(RegionIdentFor(region))
}

// AllKnownRegions returns a sorted slice of all known AWS region codes.
func AllKnownRegions() []string {
	regions := make([]string, 0, len(RegionIdents))
	for region := range RegionIdents {
		regions = append(regions, region)
	}
	slices.Sort(regions)
	return regions
}
