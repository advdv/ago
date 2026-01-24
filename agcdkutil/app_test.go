//nolint:paralleltest // jsii runtime doesn't support parallel tests
package agcdkutil_test

import (
	"testing"

	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

type testShared struct {
	Region string
}

func TestSetupApp_NoSecondaryRegions(t *testing.T) {
	defer jsii.Close()

	ctx := map[string]any{
		"myapp-qualifier":              "myapp",
		"myapp-primary-region":         "us-east-1",
		"myapp-secondary-regions":      []any{},
		"myapp-region-ident-us-east-1": "use1",
		"myapp-deployments":            []any{"Dev", "Prod"},
		"myapp-deployer-groups":        "myapp-deployers",
		"myapp-base-domain-name":       "example.com",
	}

	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &ctx,
	})

	var sharedCalls []string
	var deploymentCalls []struct{ Region, Deployment string }

	agcdkutil.SetupApp(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "myapp-deployers",
	},
		func(stack awscdk.Stack) *testShared {
			sharedCalls = append(sharedCalls, *stack.Region())
			return &testShared{Region: *stack.Region()}
		},
		func(stack awscdk.Stack, shared *testShared, deploymentIdent string) {
			deploymentCalls = append(deploymentCalls, struct{ Region, Deployment string }{
				Region:     *stack.Region(),
				Deployment: deploymentIdent,
			})
		},
	)

	// Should have exactly one shared call (primary region only)
	if len(sharedCalls) != 1 {
		t.Fatalf("expected 1 shared call, got %d: %v", len(sharedCalls), sharedCalls)
	}
	if sharedCalls[0] != "us-east-1" {
		t.Errorf("shared call region = %q, want %q", sharedCalls[0], "us-east-1")
	}

	// Should have deployment calls for Dev and Prod in primary region only
	if len(deploymentCalls) != 2 {
		t.Fatalf("expected 2 deployment calls, got %d: %v", len(deploymentCalls), deploymentCalls)
	}

	expectedDeployments := []struct{ Region, Deployment string }{
		{"us-east-1", "Dev"},
		{"us-east-1", "Prod"},
	}
	for i, want := range expectedDeployments {
		if deploymentCalls[i] != want {
			t.Errorf("deployment call %d = %+v, want %+v", i, deploymentCalls[i], want)
		}
	}
}

func TestSetupApp_WithSecondaryRegions(t *testing.T) {
	defer jsii.Close()

	ctx := map[string]any{
		"myapp-qualifier":              "myapp",
		"myapp-primary-region":         "us-east-1",
		"myapp-secondary-regions":      []any{"eu-west-1"},
		"myapp-region-ident-us-east-1": "use1",
		"myapp-region-ident-eu-west-1": "euw1",
		"myapp-deployments":            []any{"Dev"},
		"myapp-deployer-groups":        "myapp-deployers",
		"myapp-base-domain-name":       "example.com",
	}

	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &ctx,
	})

	var sharedCalls []string
	var deploymentCalls []struct{ Region, Deployment string }

	agcdkutil.SetupApp(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "myapp-deployers",
	},
		func(stack awscdk.Stack) *testShared {
			sharedCalls = append(sharedCalls, *stack.Region())
			return &testShared{Region: *stack.Region()}
		},
		func(stack awscdk.Stack, shared *testShared, deploymentIdent string) {
			deploymentCalls = append(deploymentCalls, struct{ Region, Deployment string }{
				Region:     *stack.Region(),
				Deployment: deploymentIdent,
			})
		},
	)

	// Should have two shared calls (primary + secondary)
	if len(sharedCalls) != 2 {
		t.Fatalf("expected 2 shared calls, got %d: %v", len(sharedCalls), sharedCalls)
	}
	if sharedCalls[0] != "us-east-1" {
		t.Errorf("shared call 0 region = %q, want %q", sharedCalls[0], "us-east-1")
	}
	if sharedCalls[1] != "eu-west-1" {
		t.Errorf("shared call 1 region = %q, want %q", sharedCalls[1], "eu-west-1")
	}

	// Should have deployment calls for Dev in both regions
	if len(deploymentCalls) != 2 {
		t.Fatalf("expected 2 deployment calls, got %d: %v", len(deploymentCalls), deploymentCalls)
	}

	expectedDeployments := []struct{ Region, Deployment string }{
		{"us-east-1", "Dev"},
		{"eu-west-1", "Dev"},
	}
	for i, want := range expectedDeployments {
		if deploymentCalls[i] != want {
			t.Errorf("deployment call %d = %+v, want %+v", i, deploymentCalls[i], want)
		}
	}
}
