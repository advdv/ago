package agcdkutil

import (
	"slices"
	"strings"

	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// DeployerGroups returns the IAM groups of the current deployer, or nil if not set.
// This context is passed during deploy/diff operations.
// During bootstrap (run by admins), this context is not set and returns nil.
func DeployerGroups(scope constructs.Construct, prefix string) []string {
	val := scope.Node().TryGetContext(jsii.String(prefix + "deployer-groups"))
	if val == nil {
		return nil
	}

	str, ok := val.(string)
	if !ok || str == "" {
		return nil
	}

	return strings.Fields(str)
}

// HasDeployerGroup checks if the deployer has a specific group membership.
func HasDeployerGroup(scope constructs.Construct, prefix, group string) bool {
	return slices.Contains(DeployerGroups(scope, prefix), group)
}

// AllowedDeployments returns the list of deployments the current deployer is allowed to deploy.
// If deployer-groups context is not set (e.g., during bootstrap), no deployments are allowed
// since bootstrap only needs the CDK toolkit, not application stacks.
// Otherwise, restricted deployments require membership in the specified deployers group.
func AllowedDeployments(scope constructs.Construct, prefix, deployersGroup string, restrictedDeployments []string) []string {
	all := Deployments(scope, prefix)
	groups := DeployerGroups(scope, prefix)

	// No group context provided (e.g., during bootstrap), skip deployment stacks
	if groups == nil {
		return nil
	}

	isFull := HasDeployerGroup(scope, prefix, deployersGroup)
	if isFull {
		return all
	}

	// Filter out restricted deployments for non-full deployers
	allowed := make([]string, 0, len(all))
	for _, d := range all {
		if slices.Contains(restrictedDeployments, d) {
			continue
		}
		allowed = append(allowed, d)
	}
	return allowed
}

// Deployments retrieves all available deployments from context.
func Deployments(scope constructs.Construct, prefix string) []string {
	val := scope.Node().GetContext(jsii.String(prefix + "deployments"))
	if val == nil {
		panic("invalid '" + prefix + "deployments', is it set?")
	}

	slice, ok := val.([]any)
	if !ok {
		panic("invalid '" + prefix + "deployments', expected array")
	}

	regions := make([]string, 0, len(slice))
	for _, v := range slice {
		s, ok := v.(string)
		if !ok {
			panic("invalid '" + prefix + "deployments', expected array of strings")
		}
		regions = append(regions, s)
	}

	return regions
}
