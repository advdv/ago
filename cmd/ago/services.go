package main

import (
	"slices"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
)

// ServicePermissions defines the IAM action patterns for a service.
type ServicePermissions struct {
	// ExecutionActions are actions allowed for the CDK execution role (deployment).
	// Use "*" for full access or specific action patterns.
	ExecutionActions []string
	// ConsoleActions are read-only actions for console users.
	ConsoleActions []string
}

// serviceRegistry maps AWS service namespaces to their permission patterns.
// Based on patterns from basewarphq/bstern pre-bootstrap.cfn.yaml.
var serviceRegistry = map[string]ServicePermissions{
	"apigateway": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"GET"},
	},
	"acm": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "Get*", "List*"},
	},
	"application-autoscaling": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "List*"},
	},
	"cloudfront": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "Get*", "List*"},
	},
	"cloudwatch": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "Get*", "List*"},
	},
	"cognito-idp": {
		ExecutionActions: []string{"*"},
		ConsoleActions: []string{
			"Describe*", "List*", "Get*",
			"AdminGetUser", "AdminListGroupsForUser",
		},
	},
	"dynamodb": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "List*", "GetItem", "BatchGetItem", "Query", "Scan"},
	},
	"ecr": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "Get*", "List*", "BatchGetImage"},
	},
	"events": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "List*"},
	},
	"iam": {
		ExecutionActions: []string{
			"CreateRole", "DeleteRole", "GetRole", "UpdateRole", "UpdateAssumeRolePolicy",
			"TagRole", "UntagRole", "ListRoleTags",
			"AttachRolePolicy", "DetachRolePolicy", "PutRolePolicy", "DeleteRolePolicy",
			"GetRolePolicy", "ListRolePolicies", "ListAttachedRolePolicies",
			"PassRole", "PutRolePermissionsBoundary",
		},
		ConsoleActions: []string{"Get*", "List*"},
	},
	"kms": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "Get*", "List*"},
	},
	"lambda": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Get*", "List*"},
	},
	"logs": {
		ExecutionActions: []string{
			"CreateLogGroup", "DeleteLogGroup", "DescribeLogGroups",
			"PutRetentionPolicy", "DeleteRetentionPolicy",
			"TagLogGroup", "UntagLogGroup", "ListTagsLogGroup",
			"TagResource", "UntagResource", "ListTagsForResource",
		},
		ConsoleActions: []string{
			"Describe*", "Get*", "List*",
			"StartQuery", "StopQuery", "FilterLogEvents",
		},
	},
	"route53": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Get*", "List*"},
	},
	"route53domains": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Get*", "List*"},
	},
	"s3": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Get*", "List*", "HeadObject", "HeadBucket"},
	},
	"secretsmanager": {
		ExecutionActions: []string{"*"},
		ConsoleActions: []string{
			"DescribeSecret", "GetSecretValue", "ListSecrets",
			"ListSecretVersionIds", "GetResourcePolicy", "BatchGetSecretValue",
		},
	},
	"sns": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Get*", "List*"},
	},
	"sqs": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Get*", "List*"},
	},
	"ssm": {
		ExecutionActions: []string{
			"PutParameter", "DeleteParameter", "GetParameter", "GetParameters",
			"AddTagsToResource", "RemoveTagsFromResource", "ListTagsForResource",
		},
		ConsoleActions: []string{"Describe*", "Get*", "List*"},
	},
	"states": {
		ExecutionActions: []string{"*"},
		ConsoleActions:   []string{"Describe*", "Get*", "List*"},
	},
}

// consoleOnlyServices are services that only appear in console policies (read-only).
// These provide helpful console functionality but aren't used for deployment.
var consoleOnlyServices = map[string][]string{
	"ce":              {"Get*", "Describe*"},
	"health":          {"Describe*"},
	"resource-groups": {"Get*", "List*", "Search*"},
	"servicecatalog":  {"Describe*", "Get*", "List*", "Search*"},
	"tag":             {"Get*"},
}

// SupportedServices returns a sorted list of all supported service namespaces.
func SupportedServices() []string {
	services := make([]string, 0, len(serviceRegistry))
	for svc := range serviceRegistry {
		services = append(services, svc)
	}
	sort.Strings(services)
	return services
}

// ValidateServices checks that all provided service names are supported.
func ValidateServices(services []string) error {
	var unknown []string
	for _, svc := range services {
		if _, ok := serviceRegistry[svc]; !ok {
			unknown = append(unknown, svc)
		}
	}
	if len(unknown) > 0 {
		return errors.Errorf("unknown services: %s (supported: %s)",
			strings.Join(unknown, ", "),
			strings.Join(SupportedServices(), ", "))
	}
	return nil
}

// GenerateExecutionActions generates IAM actions for the CDK execution role.
// Returns a list of actions in the format "service:action".
func GenerateExecutionActions(services []string) []string {
	actionSet := make(map[string]struct{})

	for _, svc := range services {
		perms, ok := serviceRegistry[svc]
		if !ok {
			continue
		}
		for _, action := range perms.ExecutionActions {
			actionSet[svc+":"+action] = struct{}{}
		}
	}

	actions := make([]string, 0, len(actionSet))
	for action := range actionSet {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

// GenerateConsoleActions generates IAM actions for console read-only access.
// Returns a list of actions in the format "service:action".
// Includes console-only services (ce, health, resource-groups, etc.) automatically.
func GenerateConsoleActions(services []string) []string {
	actionSet := make(map[string]struct{})

	// Add console actions for requested services
	for _, svc := range services {
		perms, ok := serviceRegistry[svc]
		if !ok {
			continue
		}
		for _, action := range perms.ConsoleActions {
			actionSet[svc+":"+action] = struct{}{}
		}
	}

	// Always include console-only services for better console experience
	for svc, actions := range consoleOnlyServices {
		for _, action := range actions {
			actionSet[svc+":"+action] = struct{}{}
		}
	}

	actions := make([]string, 0, len(actionSet))
	for action := range actionSet {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	return actions
}

// DefaultServices returns the default set of services for a new project.
func DefaultServices() []string {
	return []string{
		"apigateway",
		"cloudfront",
		"cloudwatch",
		"cognito-idp",
		"dynamodb",
		"ecr",
		"events",
		"iam",
		"kms",
		"lambda",
		"logs",
		"route53",
		"s3",
		"secretsmanager",
		"sns",
		"sqs",
		"ssm",
		"states",
		"acm",
	}
}

// ParseServicesFromContext extracts the services list from CDK context.
// The context key is "{prefix}services" and the value is a list of service names.
func ParseServicesFromContext(context map[string]any, prefix string) ([]string, error) {
	key := prefix + "services"
	val, ok := context[key]
	if !ok {
		return DefaultServices(), nil
	}

	switch contextValue := val.(type) {
	case []any:
		services := make([]string, 0, len(contextValue))
		for _, item := range contextValue {
			if s, ok := item.(string); ok {
				services = append(services, s)
			} else {
				return nil, errors.Errorf("invalid service in %s: expected string, got %T", key, item)
			}
		}
		if err := ValidateServices(services); err != nil {
			return nil, err
		}
		return services, nil
	case []string:
		if err := ValidateServices(contextValue); err != nil {
			return nil, err
		}
		return slices.Clone(contextValue), nil
	default:
		return nil, errors.Errorf("invalid %s: expected array, got %T", key, val)
	}
}
