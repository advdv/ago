package main

import (
	"strings"
	"testing"
)

func TestCheckDeploymentPermission(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		deployment  string
		isFullDep   bool
		wantErr     bool
		errContains string
	}{
		{
			name:       "dev deployment without full deployer",
			deployment: "DevAdam",
			isFullDep:  false,
			wantErr:    false,
		},
		{
			name:       "dev deployment with full deployer",
			deployment: "DevAdam",
			isFullDep:  true,
			wantErr:    false,
		},
		{
			name:       "prod deployment with full deployer",
			deployment: "Prod",
			isFullDep:  true,
			wantErr:    false,
		},
		{
			name:        "prod deployment without full deployer",
			deployment:  "Prod",
			isFullDep:   false,
			wantErr:     true,
			errContains: "requires full deployer",
		},
		{
			name:       "stag deployment with full deployer",
			deployment: "Stag",
			isFullDep:  true,
			wantErr:    false,
		},
		{
			name:        "stag deployment without full deployer",
			deployment:  "Stag",
			isFullDep:   false,
			wantErr:     true,
			errContains: "requires full deployer",
		},
		{
			name:        "production deployment without full deployer",
			deployment:  "Production",
			isFullDep:   false,
			wantErr:     true,
			errContains: "requires full deployer",
		},
		{
			name:        "staging deployment without full deployer",
			deployment:  "Staging",
			isFullDep:   false,
			wantErr:     true,
			errContains: "requires full deployer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkDeploymentPermission(tt.deployment, tt.isFullDep)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q but got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBuildCDKArgs(t *testing.T) {
	t.Parallel()
	cdkContext := map[string]any{
		"admin-profile":   "test-admin",
		"myapp-qualifier": "myapp",
	}

	args := buildCDKArgs("test-adam", "myapp", "myapp-", cdkContext)

	expected := []string{
		"--profile", "test-adam",
		"--qualifier", "myapp",
		"--toolkit-stack-name", "myappBootstrap",
		"-c", "myapp-deployers-group=myapp-deployers",
		"-c", "myapp-dev-deployers-group=myapp-dev-deployers",
	}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}

	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d]: expected %q, got %q", i, expected[i], arg)
		}
	}
}

func TestValidateDeployerUsername(t *testing.T) {
	t.Parallel()
	tests := []struct {
		username string
		wantErr  bool
	}{
		{"Adam", false},
		{"Bob", false},
		{"Alice123", false},
		{"adam", true},
		{"123Adam", true},
		{"Adam-Smith", true},
		{"Adam_Smith", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			t.Parallel()
			err := validateDeployerUsername(tt.username)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for username %q but got nil", tt.username)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for username %q: %v", tt.username, err)
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"myproject", false},
		{"test123", false},
		{"a", false},
		{"MyProject", true},
		{"my-project", true},
		{"my_project", true},
		{"123project", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateProjectName(tt.name)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for project name %q but got nil", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for project name %q: %v", tt.name, err)
			}
		})
	}
}

func TestDetectPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		context    map[string]any
		wantPrefix string
		wantErr    bool
	}{
		{
			name:       "finds qualifier",
			context:    map[string]any{"myapp-qualifier": "myapp"},
			wantPrefix: "myapp-",
			wantErr:    false,
		},
		{
			name:       "finds qualifier with different prefix",
			context:    map[string]any{"other-qualifier": "other"},
			wantPrefix: "other-",
			wantErr:    false,
		},
		{
			name:       "no qualifier",
			context:    map[string]any{"something": "value"},
			wantPrefix: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prefix, err := detectPrefix(tt.context)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if prefix != tt.wantPrefix {
					t.Errorf("expected prefix %q, got %q", tt.wantPrefix, prefix)
				}
			}
		})
	}
}

func TestExtractStringSlice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		context map[string]any
		key     string
		want    []string
	}{
		{
			name:    "extracts string slice",
			context: map[string]any{"items": []any{"a", "b", "c"}},
			key:     "items",
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "missing key",
			context: map[string]any{},
			key:     "items",
			want:    nil,
		},
		{
			name:    "wrong type",
			context: map[string]any{"items": "not a slice"},
			key:     "items",
			want:    nil,
		},
		{
			name:    "mixed types in slice",
			context: map[string]any{"items": []any{"a", 123, "c"}},
			key:     "items",
			want:    []string{"a", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractStringSlice(tt.context, tt.key)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestParseCommaList(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b, c", []string{"a", "b", "c"}},
		{"  a  ,  b  ,  c  ", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", nil},
		{"a,,b", []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := parseCommaList(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}
