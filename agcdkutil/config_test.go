package agcdkutil_test

import (
	"strings"
	"testing"

	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
)

func TestNewConfig(t *testing.T) {
	defer jsii.Close()

	tests := []struct {
		name        string
		context     map[string]any
		appConfig   agcdkutil.AppConfig
		wantErr     bool
		errContains []string
	}{
		{
			name: "valid config",
			context: map[string]any{
				"myapp-qualifier":            "myapp",
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{"eu-west-1"},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-region-ident-eu-west-1": "euw1",
				"myapp-deployments":          []any{"Dev", "Stag", "Prod"},
				"myapp-deployer-groups":      "myapp-deployers",
				"myapp-base-domain-name":     "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:                "myapp-",
				DeployersGroup:        "myapp-deployers",
				RestrictedDeployments: []string{"Stag", "Prod"},
			},
			wantErr: false,
		},
		{
			name: "valid config without secondary regions",
			context: map[string]any{
				"myapp-qualifier":            "myapp",
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":          []any{"Dev"},
				"myapp-base-domain-name":     "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr: false,
		},
		{
			name: "missing qualifier",
			context: map[string]any{
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":          []any{"Dev"},
				"myapp-base-domain-name":     "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"myapp-qualifier", "is not set"},
		},
		{
			name: "qualifier too long",
			context: map[string]any{
				"myapp-qualifier":            "thisqualifieristoolong",
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":          []any{"Dev"},
				"myapp-base-domain-name":     "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"Qualifier", "exceeds maximum length"},
		},
		{
			name: "invalid base domain name",
			context: map[string]any{
				"myapp-qualifier":            "myapp",
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":          []any{"Dev"},
				"myapp-base-domain-name":     "not a valid domain",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"BaseDomainName", "valid domain"},
		},
		{
			name: "missing region ident context keys",
			context: map[string]any{
				"myapp-qualifier":         "myapp",
				"myapp-primary-region":    "us-east-1",
				"myapp-secondary-regions": []any{"eu-west-1"},
				// missing myapp-region-ident-us-east-1 and myapp-region-ident-eu-west-1
				"myapp-deployments":      []any{"Dev"},
				"myapp-base-domain-name": "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"region-ident-us-east-1", "is not set"},
		},
		{
			name: "region ident missing for secondary region",
			context: map[string]any{
				"myapp-qualifier":              "myapp",
				"myapp-primary-region":         "us-east-1",
				"myapp-secondary-regions":      []any{"eu-west-1", "ap-southeast-1"},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-region-ident-eu-west-1": "euw1",
				// missing myapp-region-ident-ap-southeast-1
				"myapp-deployments":      []any{"Dev"},
				"myapp-base-domain-name": "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"region-ident-ap-southeast-1", "is not set"},
		},
		{
			name: "multiple errors",
			context: map[string]any{
				"myapp-secondary-regions": []any{},
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"myapp-qualifier", "myapp-primary-region", "myapp-deployments"},
		},
		{
			name: "wrong type for qualifier",
			context: map[string]any{
				"myapp-qualifier":            123, // should be string
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":          []any{"Dev"},
				"myapp-base-domain-name":     "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"myapp-qualifier", "must be a string"},
		},
		{
			name: "wrong type for deployments",
			context: map[string]any{
				"myapp-qualifier":            "myapp",
				"myapp-primary-region":       "us-east-1",
				"myapp-secondary-regions":    []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":          "Dev", // should be array
				"myapp-base-domain-name":     "example.com",
			},
			appConfig: agcdkutil.AppConfig{
				Prefix:         "myapp-",
				DeployersGroup: "myapp-deployers",
			},
			wantErr:     true,
			errContains: []string{"myapp-deployments", "must be an array"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := awscdk.NewApp(&awscdk.AppProps{
				Context: &tt.context,
			})

			cfg, err := agcdkutil.NewConfig(app, tt.appConfig)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				for _, contains := range tt.errContains {
					if !strings.Contains(err.Error(), contains) {
						t.Errorf("error %q should contain %q", err.Error(), contains)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify config values
			if cfg.Qualifier != tt.context["myapp-qualifier"] {
				t.Errorf("Qualifier = %q, want %q", cfg.Qualifier, tt.context["myapp-qualifier"])
			}
			if cfg.PrimaryRegion != tt.context["myapp-primary-region"] {
				t.Errorf("PrimaryRegion = %q, want %q", cfg.PrimaryRegion, tt.context["myapp-primary-region"])
			}
		})
	}
}

func TestConfig_AllRegions(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &map[string]any{
			"myapp-qualifier":              "myapp",
			"myapp-primary-region":         "us-east-1",
			"myapp-secondary-regions":      []any{"eu-west-1", "ap-southeast-1"},
			"myapp-region-ident-us-east-1": "use1",
			"myapp-region-ident-eu-west-1": "euw1",
			"myapp-region-ident-ap-southeast-1": "apse1",
			"myapp-deployments":            []any{"Dev"},
			"myapp-base-domain-name":       "example.com",
		},
	})

	cfg, err := agcdkutil.NewConfig(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "deployers",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	regions := cfg.AllRegions()
	if len(regions) != 3 {
		t.Fatalf("AllRegions() = %v, want 3 regions", regions)
	}
	if regions[0] != "us-east-1" {
		t.Errorf("AllRegions()[0] = %q, want %q", regions[0], "us-east-1")
	}
	if regions[1] != "eu-west-1" {
		t.Errorf("AllRegions()[1] = %q, want %q", regions[1], "eu-west-1")
	}
	if regions[2] != "ap-southeast-1" {
		t.Errorf("AllRegions()[2] = %q, want %q", regions[2], "ap-southeast-1")
	}
}

func TestConfig_AllowedDeployments(t *testing.T) {
	tests := []struct {
		name           string
		deployerGroups string
		wantLen        int
		wantDeployments []string
	}{
		{
			name:           "full deployer has all",
			deployerGroups: "myapp-deployers",
			wantLen:        3,
			wantDeployments: []string{"Dev", "Stag", "Prod"},
		},
		{
			name:           "limited deployer has only Dev",
			deployerGroups: "limited-group",
			wantLen:        1,
			wantDeployments: []string{"Dev"},
		},
		{
			name:           "no groups returns nil",
			deployerGroups: "",
			wantLen:        0,
			wantDeployments: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer jsii.Close()

			ctx := map[string]any{
				"myapp-qualifier":              "myapp",
				"myapp-primary-region":         "us-east-1",
				"myapp-secondary-regions":      []any{},
				"myapp-region-ident-us-east-1": "use1",
				"myapp-deployments":            []any{"Dev", "Stag", "Prod"},
				"myapp-base-domain-name":       "example.com",
			}
			if tt.deployerGroups != "" {
				ctx["myapp-deployer-groups"] = tt.deployerGroups
			}

			app := awscdk.NewApp(&awscdk.AppProps{
				Context: &ctx,
			})

			cfg, err := agcdkutil.NewConfig(app, agcdkutil.AppConfig{
				Prefix:                "myapp-",
				DeployersGroup:        "myapp-deployers",
				RestrictedDeployments: []string{"Stag", "Prod"},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			allowed := cfg.AllowedDeployments()

			if tt.wantDeployments == nil {
				if allowed != nil {
					t.Fatalf("AllowedDeployments() = %v, want nil", allowed)
				}
				return
			}

			if len(allowed) != tt.wantLen {
				t.Fatalf("AllowedDeployments() = %v (len %d), want len %d", allowed, len(allowed), tt.wantLen)
			}

			for i, want := range tt.wantDeployments {
				if allowed[i] != want {
					t.Errorf("AllowedDeployments()[%d] = %q, want %q", i, allowed[i], want)
				}
			}
		})
	}
}

func TestConfig_RegionIdent(t *testing.T) {
	defer jsii.Close()

	app := awscdk.NewApp(&awscdk.AppProps{
		Context: &map[string]any{
			"myapp-qualifier":              "myapp",
			"myapp-primary-region":         "us-east-1",
			"myapp-secondary-regions":      []any{"eu-west-1"},
			"myapp-region-ident-us-east-1": "use1",
			"myapp-region-ident-eu-west-1": "euw1",
			"myapp-deployments":            []any{"Dev"},
			"myapp-base-domain-name":       "example.com",
		},
	})

	cfg, err := agcdkutil.NewConfig(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "deployers",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ident := cfg.RegionIdent("us-east-1"); ident != "use1" {
		t.Errorf("RegionIdent(us-east-1) = %q, want %q", ident, "use1")
	}
	if ident := cfg.RegionIdent("eu-west-1"); ident != "euw1" {
		t.Errorf("RegionIdent(eu-west-1) = %q, want %q", ident, "euw1")
	}
}
