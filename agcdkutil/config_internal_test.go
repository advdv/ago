//nolint:paralleltest // this test doesn't need parallel execution
package agcdkutil

import (
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
)

func TestValidateConfigRegionIdents(t *testing.T) {
	tests := []struct {
		name               string
		config             Config
		wantErr            bool
		wantMissingRegions []string
	}{
		{
			name: "valid - all regions have idents",
			config: Config{
				Prefix:           "test-",
				Qualifier:        "testq",
				PrimaryRegion:    "us-east-1",
				SecondaryRegions: []string{"eu-west-1"},
				RegionIdents: map[string]string{
					"us-east-1": "use1",
					"eu-west-1": "euw1",
				},
				Deployments:    []string{"Dev"},
				BaseDomainName: "example.com",
				DeployersGroup: "deployers",
			},
			wantErr: false,
		},
		{
			name: "invalid - primary region missing from RegionIdents",
			config: Config{
				Prefix:           "test-",
				Qualifier:        "testq",
				PrimaryRegion:    "us-east-1",
				SecondaryRegions: []string{},
				RegionIdents:     map[string]string{},
				Deployments:      []string{"Dev"},
				BaseDomainName:   "example.com",
				DeployersGroup:   "deployers",
			},
			wantErr:            true,
			wantMissingRegions: []string{"us-east-1"},
		},
		{
			name: "invalid - secondary region missing from RegionIdents",
			config: Config{
				Prefix:           "test-",
				Qualifier:        "testq",
				PrimaryRegion:    "us-east-1",
				SecondaryRegions: []string{"eu-west-1", "ap-southeast-1"},
				RegionIdents: map[string]string{
					"us-east-1": "use1",
					"eu-west-1": "euw1",
					// missing ap-southeast-1
				},
				Deployments:    []string{"Dev"},
				BaseDomainName: "example.com",
				DeployersGroup: "deployers",
			},
			wantErr:            true,
			wantMissingRegions: []string{"ap-southeast-1"},
		},
		{
			name: "invalid - multiple regions missing from RegionIdents",
			config: Config{
				Prefix:           "test-",
				Qualifier:        "testq",
				PrimaryRegion:    "us-east-1",
				SecondaryRegions: []string{"eu-west-1", "ap-southeast-1"},
				RegionIdents:     map[string]string{},
				Deployments:      []string{"Dev"},
				BaseDomainName:   "example.com",
				DeployersGroup:   "deployers",
			},
			wantErr:            true,
			wantMissingRegions: []string{"us-east-1", "eu-west-1", "ap-southeast-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validate := validator.New(validator.WithRequiredStructEnabled())
			validate.RegisterStructValidation(validateConfigRegionIdents, Config{})

			err := validate.Struct(tt.config)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return
			}

			if err == nil {
				t.Fatalf("expected error but got nil")
			}

			var validationErrs validator.ValidationErrors
			if !errors.As(err, &validationErrs) {
				t.Fatalf("expected ValidationErrors, got %T", err)
			}

			formatted := formatValidationErrors(validationErrs)

			for _, region := range tt.wantMissingRegions {
				if !strings.Contains(formatted, region) {
					t.Errorf("formatted error %q should contain region %q", formatted, region)
				}
			}

			if !strings.Contains(formatted, "RegionIdents") {
				t.Errorf("formatted error %q should contain 'RegionIdents'", formatted)
			}
			if !strings.Contains(formatted, "missing entry for region") {
				t.Errorf("formatted error %q should contain 'missing entry for region'", formatted)
			}
		})
	}
}

func formatValidationErrors(errs validator.ValidationErrors) string {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, formatValidationError(e))
	}

	return strings.Join(msgs, "\n")
}
