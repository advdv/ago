package main

import (
	"slices"
	"strings"
	"testing"
)

func TestSupportedServices(t *testing.T) {
	t.Parallel()

	services := SupportedServices()

	if len(services) == 0 {
		t.Error("expected at least one supported service")
	}

	if !slices.IsSorted(services) {
		t.Error("expected services to be sorted")
	}

	required := []string{"lambda", "dynamodb", "s3", "iam", "cognito-idp"}
	for _, svc := range required {
		if !slices.Contains(services, svc) {
			t.Errorf("expected %q to be in supported services", svc)
		}
	}
}

func TestValidateServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		services []string
		wantErr  bool
	}{
		{
			name:     "valid services",
			services: []string{"lambda", "dynamodb", "s3"},
			wantErr:  false,
		},
		{
			name:     "empty list",
			services: []string{},
			wantErr:  false,
		},
		{
			name:     "unknown service",
			services: []string{"lambda", "not-a-service"},
			wantErr:  true,
		},
		{
			name:     "all default services",
			services: DefaultServices(),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateServices(tt.services)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateExecutionActions(t *testing.T) {
	t.Parallel()

	actions := GenerateExecutionActions([]string{"lambda", "dynamodb"})

	if len(actions) == 0 {
		t.Error("expected at least one action")
	}

	if !slices.IsSorted(actions) {
		t.Error("expected actions to be sorted")
	}

	for _, action := range actions {
		if !strings.Contains(action, ":") {
			t.Errorf("action %q should contain ':'", action)
		}
	}

	if !slices.Contains(actions, "lambda:*") {
		t.Error("expected lambda:* in execution actions")
	}

	if !slices.Contains(actions, "dynamodb:*") {
		t.Error("expected dynamodb:* in execution actions")
	}
}

func TestGenerateExecutionActions_IAM(t *testing.T) {
	t.Parallel()

	actions := GenerateExecutionActions([]string{"iam"})

	if slices.Contains(actions, "iam:*") {
		t.Error("IAM should not have wildcard execution actions")
	}

	expected := []string{"iam:CreateRole", "iam:DeleteRole", "iam:PassRole"}
	for _, exp := range expected {
		if !slices.Contains(actions, exp) {
			t.Errorf("expected %q in IAM execution actions", exp)
		}
	}
}

func TestGenerateConsoleActions(t *testing.T) {
	t.Parallel()

	actions := GenerateConsoleActions([]string{"lambda", "dynamodb"})

	if len(actions) == 0 {
		t.Error("expected at least one action")
	}

	if !slices.IsSorted(actions) {
		t.Error("expected actions to be sorted")
	}

	hasLambdaRead := false
	for _, action := range actions {
		if strings.HasPrefix(action, "lambda:Get") || strings.HasPrefix(action, "lambda:List") {
			hasLambdaRead = true
			break
		}
	}
	if !hasLambdaRead {
		t.Error("expected lambda Get*/List* in console actions")
	}

	expectedDDB := []string{"dynamodb:GetItem", "dynamodb:Query", "dynamodb:Scan"}
	for _, exp := range expectedDDB {
		if !slices.Contains(actions, exp) {
			t.Errorf("expected %q in console actions", exp)
		}
	}
}

func TestGenerateConsoleActions_IncludesConsoleOnlyServices(t *testing.T) {
	t.Parallel()

	actions := GenerateConsoleActions([]string{})

	consoleOnlyPrefixes := []string{"ce:", "health:", "resource-groups:", "tag:"}
	for _, prefix := range consoleOnlyPrefixes {
		found := false
		for _, action := range actions {
			if strings.HasPrefix(action, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected console-only service %s to be included", prefix)
		}
	}
}

func TestParseServicesFromContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		context map[string]any
		prefix  string
		want    []string
		wantErr bool
	}{
		{
			name:    "missing key returns defaults",
			context: map[string]any{},
			prefix:  "proj-",
			want:    DefaultServices(),
			wantErr: false,
		},
		{
			name: "valid services list",
			context: map[string]any{
				"proj-services": []any{"lambda", "s3"},
			},
			prefix:  "proj-",
			want:    []string{"lambda", "s3"},
			wantErr: false,
		},
		{
			name: "invalid service in list",
			context: map[string]any{
				"proj-services": []any{"lambda", "not-a-service"},
			},
			prefix:  "proj-",
			wantErr: true,
		},
		{
			name: "wrong type for services",
			context: map[string]any{
				"proj-services": "lambda,s3",
			},
			prefix:  "proj-",
			wantErr: true,
		},
		{
			name: "wrong type in array",
			context: map[string]any{
				"proj-services": []any{"lambda", 123},
			},
			prefix:  "proj-",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseServicesFromContext(tt.context, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseServicesFromContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !slices.Equal(got, tt.want) {
				t.Errorf("ParseServicesFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultServices_AllValid(t *testing.T) {
	t.Parallel()

	defaults := DefaultServices()
	if err := ValidateServices(defaults); err != nil {
		t.Errorf("DefaultServices() contains invalid services: %v", err)
	}
}

func TestServiceRegistry_AllHaveActions(t *testing.T) {
	t.Parallel()

	for svc, perms := range serviceRegistry {
		if len(perms.ExecutionActions) == 0 {
			t.Errorf("service %q has no execution actions", svc)
		}
		if len(perms.ConsoleActions) == 0 {
			t.Errorf("service %q has no console actions", svc)
		}
	}
}
