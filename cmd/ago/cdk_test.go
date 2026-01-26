package main

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "myproject", false},
		{"valid with numbers", "project123", false},
		{"valid single letter", "a", false},
		{"invalid starts with number", "123project", true},
		{"invalid uppercase", "MyProject", true},
		{"invalid with hyphen", "my-project", true},
		{"invalid with underscore", "my_project", true},
		{"invalid empty", "", true},
		{"invalid spaces", "my project", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateProjectName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProjectName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDeployerUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid capital start", "Adam", false},
		{"valid with numbers", "Adam123", false},
		{"valid single capital", "A", false},
		{"valid mixed case", "AdamSmith", false},
		{"invalid lowercase start", "adam", true},
		{"invalid all lowercase", "adamsmith", true},
		{"invalid starts with number", "123Adam", true},
		{"invalid empty", "", true},
		{"invalid spaces", "Adam Smith", true},
		{"invalid hyphen", "Adam-Smith", true},
		{"invalid underscore", "Adam_Smith", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDeployerUsername(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDeployerUsername(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseCommaList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"single item", "Adam", []string{"Adam"}},
		{"multiple items", "Adam,Bob,Charlie", []string{"Adam", "Bob", "Charlie"}},
		{"items with spaces", "Adam, Bob, Charlie", []string{"Adam", "Bob", "Charlie"}},
		{"trailing comma", "Adam,Bob,", []string{"Adam", "Bob"}},
		{"leading comma", ",Adam,Bob", []string{"Adam", "Bob"}},
		{"multiple commas", "Adam,,Bob", []string{"Adam", "Bob"}},
		{"only spaces", "  ,  ,  ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := parseCommaList(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseCommaList(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("parseCommaList(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
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
			name:       "standard prefix",
			context:    map[string]any{"myapp-qualifier": "bdoc"},
			wantPrefix: "myapp-",
			wantErr:    false,
		},
		{
			name:       "no qualifier key",
			context:    map[string]any{"other-key": "value"},
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "qualifier without prefix",
			context:    map[string]any{"qualifier": "bdoc"},
			wantPrefix: "",
			wantErr:    true,
		},
		{
			name:       "multiple keys with qualifier",
			context:    map[string]any{"foo": "bar", "bw-qualifier": "test", "other": 123},
			wantPrefix: "bw-",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prefix, err := detectPrefix(tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("detectPrefix() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if prefix != tt.wantPrefix {
				t.Errorf("detectPrefix() = %q, want %q", prefix, tt.wantPrefix)
			}
		})
	}
}

func TestExtractStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		context  map[string]any
		key      string
		expected []string
	}{
		{
			name:     "existing key with strings",
			context:  map[string]any{"regions": []any{"eu-central-1", "eu-north-1"}},
			key:      "regions",
			expected: []string{"eu-central-1", "eu-north-1"},
		},
		{
			name:     "missing key",
			context:  map[string]any{"other": "value"},
			key:      "regions",
			expected: nil,
		},
		{
			name:     "key with non-slice value",
			context:  map[string]any{"regions": "single-value"},
			key:      "regions",
			expected: nil,
		},
		{
			name:     "empty slice",
			context:  map[string]any{"regions": []any{}},
			key:      "regions",
			expected: []string{},
		},
		{
			name:     "mixed types in slice",
			context:  map[string]any{"regions": []any{"eu-central-1", 123, "eu-north-1"}},
			key:      "regions",
			expected: []string{"eu-central-1", "eu-north-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractStringSlice(tt.context, tt.key)
			if len(result) != len(tt.expected) {
				t.Errorf("extractStringSlice() = %v, want %v", result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("extractStringSlice()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestWriteOutputf(t *testing.T) {
	t.Parallel()

	t.Run("writes formatted output", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		writeOutputf(&buf, "Hello %s, you have %d messages", "World", 42)
		if got := buf.String(); got != "Hello World, you have 42 messages" {
			t.Errorf("writeOutputf() wrote %q, want %q", got, "Hello World, you have 42 messages")
		}
	})

	t.Run("handles nil writer", func(t *testing.T) {
		t.Parallel()
		writeOutputf(nil, "This should not panic")
	})
}

func TestPreBootstrapTemplateRendering(t *testing.T) {
	t.Parallel()

	testServices := []string{"lambda", "dynamodb", "s3"}

	t.Run("renders template with qualifier", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		if path == "" {
			t.Error("renderPreBootstrapTemplate() returned empty path")
		}
	})

	t.Run("template contains language extensions transform", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		content, err := readFileContent(path)
		if err != nil {
			t.Fatalf("failed to read rendered template: %v", err)
		}

		if !strings.Contains(content, "Transform: AWS::LanguageExtensions") {
			t.Error("template should contain AWS::LanguageExtensions transform")
		}
	})

	t.Run("template contains ForEach for deployers", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		content, err := readFileContent(path)
		if err != nil {
			t.Fatalf("failed to read rendered template: %v", err)
		}

		if !strings.Contains(content, "Fn::ForEach::DeployerUsers") {
			t.Error("template should contain Fn::ForEach::DeployerUsers")
		}
		if !strings.Contains(content, "Fn::ForEach::DevDeployerUsers") {
			t.Error("template should contain Fn::ForEach::DevDeployerUsers")
		}
	})

	t.Run("template contains deployer parameters", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		content, err := readFileContent(path)
		if err != nil {
			t.Fatalf("failed to read rendered template: %v", err)
		}

		if !strings.Contains(content, "Deployers:") {
			t.Error("template should contain Deployers parameter")
		}
		if !strings.Contains(content, "DevDeployers:") {
			t.Error("template should contain DevDeployers parameter")
		}
	})

	t.Run("template contains conditions for deployers", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		content, err := readFileContent(path)
		if err != nil {
			t.Fatalf("failed to read rendered template: %v", err)
		}

		if !strings.Contains(content, "HasDeployers:") {
			t.Error("template should contain HasDeployers condition")
		}
		if !strings.Contains(content, "HasDevDeployers:") {
			t.Error("template should contain HasDevDeployers condition")
		}
	})

	t.Run("template contains service-specific execution actions", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		content, err := readFileContent(path)
		if err != nil {
			t.Fatalf("failed to read rendered template: %v", err)
		}

		if !strings.Contains(content, "lambda:*") {
			t.Error("template should contain lambda:* in execution actions")
		}
		if !strings.Contains(content, "dynamodb:*") {
			t.Error("template should contain dynamodb:* in execution actions")
		}
		if !strings.Contains(content, "s3:*") {
			t.Error("template should contain s3:* in execution actions")
		}
	})

	t.Run("template contains console read actions", func(t *testing.T) {
		t.Parallel()
		path, cleanup, err := renderPreBootstrapTemplate("testproj", testServices)
		if err != nil {
			t.Fatalf("renderPreBootstrapTemplate() error = %v", err)
		}
		defer cleanup()

		content, err := readFileContent(path)
		if err != nil {
			t.Fatalf("failed to read rendered template: %v", err)
		}

		if !strings.Contains(content, "lambda:Get*") {
			t.Error("template should contain lambda:Get* in console actions")
		}
		if !strings.Contains(content, "dynamodb:Query") {
			t.Error("template should contain dynamodb:Query in console actions")
		}
	})
}

func readFileContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func TestReadWriteContextFile(t *testing.T) {
	t.Parallel()

	t.Run("writes and reads context file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		contextPath := tmpDir + "/cdk.context.json"

		original := map[string]any{
			"myproj-qualifier":  "myproj",
			"myproj-deployers":  []string{"Adam", "Bob"},
			"some-other-config": 123,
		}

		err := writeContextFile(contextPath, original)
		if err != nil {
			t.Fatalf("writeContextFile() error = %v", err)
		}

		readBack, err := readContextFile(contextPath)
		if err != nil {
			t.Fatalf("readContextFile() error = %v", err)
		}

		if readBack["myproj-qualifier"] != "myproj" {
			t.Errorf("qualifier = %v, want %v", readBack["myproj-qualifier"], "myproj")
		}
	})

	t.Run("readContextFile fails on missing file", func(t *testing.T) {
		t.Parallel()
		_, err := readContextFile("/nonexistent/path/cdk.context.json")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func TestRemoveProfileFromFile(t *testing.T) {
	t.Parallel()

	t.Run("removes profile section from file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		filePath := tmpDir + "/credentials"

		content := `[default]
aws_access_key_id=DEFAULT123
aws_secret_access_key=DEFAULTSECRET

[myproj-adam]
aws_access_key_id=ADAM123
aws_secret_access_key=ADAMSECRET

[myproj-bob]
aws_access_key_id=BOB123
aws_secret_access_key=BOBSECRET
`
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		err := removeProfileFromFile(filePath, "myproj-adam")
		if err != nil {
			t.Fatalf("removeProfileFromFile() error = %v", err)
		}

		result, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		resultStr := string(result)
		if strings.Contains(resultStr, "[myproj-adam]") {
			t.Error("file should not contain [myproj-adam] section")
		}
		if strings.Contains(resultStr, "ADAM123") {
			t.Error("file should not contain ADAM123")
		}
		if !strings.Contains(resultStr, "[default]") {
			t.Error("file should still contain [default] section")
		}
		if !strings.Contains(resultStr, "[myproj-bob]") {
			t.Error("file should still contain [myproj-bob] section")
		}
	})

	t.Run("handles missing file gracefully", func(t *testing.T) {
		t.Parallel()
		err := removeProfileFromFile("/nonexistent/credentials", "some-profile")
		if err != nil {
			t.Errorf("expected no error for missing file, got %v", err)
		}
	})

	t.Run("removes last profile leaving empty file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		filePath := tmpDir + "/credentials"

		content := `[myproj-adam]
aws_access_key_id=ADAM123
aws_secret_access_key=ADAMSECRET
`
		if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		err := removeProfileFromFile(filePath, "myproj-adam")
		if err != nil {
			t.Fatalf("removeProfileFromFile() error = %v", err)
		}

		result, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		if len(strings.TrimSpace(string(result))) != 0 {
			t.Errorf("expected empty file, got %q", string(result))
		}
	})
}

func setupTestCDKDir(t *testing.T, context map[string]any) (tmpDir, cdkDir string) {
	t.Helper()
	tmpDir = t.TempDir()
	cdkDir = tmpDir + "/infra/cdk/cdk"
	if err := os.MkdirAll(cdkDir, 0o755); err != nil {
		t.Fatalf("failed to create cdk dir: %v", err)
	}

	cdkJSON := map[string]any{"app": "go mod download && go run ."}
	if err := writeContextFile(cdkDir+"/cdk.json", cdkJSON); err != nil {
		t.Fatalf("failed to write cdk.json: %v", err)
	}

	if err := writeContextFile(cdkDir+"/cdk.context.json", context); err != nil {
		t.Fatalf("failed to write cdk.context.json: %v", err)
	}

	return tmpDir, cdkDir
}

func TestDoAddDeployer(t *testing.T) {
	t.Parallel()

	t.Run("adds deployer and deployment to context", func(t *testing.T) {
		t.Parallel()
		tmpDir, cdkDir := setupTestCDKDir(t, map[string]any{
			"myproj-qualifier":     "myproj",
			"myproj-deployers":     []any{},
			"myproj-dev-deployers": []any{},
			"myproj-deployments":   []any{"Prod", "Stag"},
		})

		var buf strings.Builder
		opts := deployerOptions{
			ProjectDir: tmpDir,
			Username:   "Adam",
			DevOnly:    false,
			Output:     &buf,
		}

		err := doAddDeployer(t.Context(), opts)
		if err != nil {
			t.Fatalf("doAddDeployer() error = %v", err)
		}

		result, err := readContextFile(cdkDir + "/cdk.context.json")
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		deployers := extractStringSlice(result, "myproj-deployers")
		if len(deployers) != 1 || deployers[0] != "Adam" {
			t.Errorf("deployers = %v, want [Adam]", deployers)
		}

		deployments := extractStringSlice(result, "myproj-deployments")
		if len(deployments) != 3 {
			t.Errorf("deployments = %v, want [Prod Stag DevAdam]", deployments)
		}
		if !slices.Contains(deployments, "DevAdam") {
			t.Errorf("deployments should contain DevAdam, got %v", deployments)
		}

		output := buf.String()
		if !strings.Contains(output, "Added \"Adam\" to deployers") {
			t.Errorf("output should mention adding to deployers: %s", output)
		}
		if !strings.Contains(output, "Added \"DevAdam\" to deployments") {
			t.Errorf("output should mention adding deployment: %s", output)
		}
	})

	t.Run("adds dev deployer and deployment to context", func(t *testing.T) {
		t.Parallel()
		tmpDir, cdkDir := setupTestCDKDir(t, map[string]any{
			"myproj-qualifier":     "myproj",
			"myproj-deployers":     []any{},
			"myproj-dev-deployers": []any{},
			"myproj-deployments":   []any{"Prod", "Stag"},
		})

		var buf strings.Builder
		opts := deployerOptions{
			ProjectDir: tmpDir,
			Username:   "Bob",
			DevOnly:    true,
			Output:     &buf,
		}

		err := doAddDeployer(t.Context(), opts)
		if err != nil {
			t.Fatalf("doAddDeployer() error = %v", err)
		}

		result, err := readContextFile(cdkDir + "/cdk.context.json")
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		devDeployers := extractStringSlice(result, "myproj-dev-deployers")
		if len(devDeployers) != 1 || devDeployers[0] != "Bob" {
			t.Errorf("dev-deployers = %v, want [Bob]", devDeployers)
		}

		deployments := extractStringSlice(result, "myproj-deployments")
		if !slices.Contains(deployments, "DevBob") {
			t.Errorf("deployments should contain DevBob, got %v", deployments)
		}
	})

	t.Run("does not duplicate existing deployment", func(t *testing.T) {
		t.Parallel()
		tmpDir, cdkDir := setupTestCDKDir(t, map[string]any{
			"myproj-qualifier":     "myproj",
			"myproj-deployers":     []any{},
			"myproj-dev-deployers": []any{},
			"myproj-deployments":   []any{"Prod", "Stag", "DevAdam"},
		})

		var buf strings.Builder
		opts := deployerOptions{
			ProjectDir: tmpDir,
			Username:   "Adam",
			DevOnly:    false,
			Output:     &buf,
		}

		err := doAddDeployer(t.Context(), opts)
		if err != nil {
			t.Fatalf("doAddDeployer() error = %v", err)
		}

		result, err := readContextFile(cdkDir + "/cdk.context.json")
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		deployments := extractStringSlice(result, "myproj-deployments")
		count := 0
		for _, d := range deployments {
			if d == "DevAdam" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("DevAdam should appear exactly once, got %d times in %v", count, deployments)
		}

		output := buf.String()
		if strings.Contains(output, "Added \"DevAdam\" to deployments") {
			t.Errorf("should not report adding existing deployment: %s", output)
		}
	})
}

func TestDoRemoveDeployer(t *testing.T) {
	t.Parallel()

	t.Run("removes deployer and deployment from context", func(t *testing.T) {
		t.Parallel()
		tmpDir, cdkDir := setupTestCDKDir(t, map[string]any{
			"myproj-qualifier":     "myproj",
			"myproj-deployers":     []any{"Adam", "Bob"},
			"myproj-dev-deployers": []any{},
			"myproj-deployments":   []any{"Prod", "Stag", "DevAdam", "DevBob"},
		})

		var buf strings.Builder
		opts := deployerOptions{
			ProjectDir: tmpDir,
			Username:   "Adam",
			Output:     &buf,
		}

		err := doRemoveDeployer(t.Context(), opts)
		if err != nil {
			t.Fatalf("doRemoveDeployer() error = %v", err)
		}

		result, err := readContextFile(cdkDir + "/cdk.context.json")
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		deployers := extractStringSlice(result, "myproj-deployers")
		if len(deployers) != 1 || deployers[0] != "Bob" {
			t.Errorf("deployers = %v, want [Bob]", deployers)
		}

		deployments := extractStringSlice(result, "myproj-deployments")
		if slices.Contains(deployments, "DevAdam") {
			t.Errorf("deployments should not contain DevAdam, got %v", deployments)
		}
		if !slices.Contains(deployments, "DevBob") {
			t.Errorf("deployments should still contain DevBob, got %v", deployments)
		}

		output := buf.String()
		if !strings.Contains(output, "Removed \"Adam\" from deployers") {
			t.Errorf("output should mention removing from deployers: %s", output)
		}
		if !strings.Contains(output, "Removed \"DevAdam\" from deployments") {
			t.Errorf("output should mention removing deployment: %s", output)
		}
	})

	t.Run("removes dev deployer and deployment from context", func(t *testing.T) {
		t.Parallel()
		tmpDir, cdkDir := setupTestCDKDir(t, map[string]any{
			"myproj-qualifier":     "myproj",
			"myproj-deployers":     []any{},
			"myproj-dev-deployers": []any{"Charlie"},
			"myproj-deployments":   []any{"Prod", "Stag", "DevCharlie"},
		})

		var buf strings.Builder
		opts := deployerOptions{
			ProjectDir: tmpDir,
			Username:   "Charlie",
			Output:     &buf,
		}

		err := doRemoveDeployer(t.Context(), opts)
		if err != nil {
			t.Fatalf("doRemoveDeployer() error = %v", err)
		}

		result, err := readContextFile(cdkDir + "/cdk.context.json")
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}

		devDeployers := extractStringSlice(result, "myproj-dev-deployers")
		if len(devDeployers) != 0 {
			t.Errorf("dev-deployers = %v, want []", devDeployers)
		}

		deployments := extractStringSlice(result, "myproj-deployments")
		if slices.Contains(deployments, "DevCharlie") {
			t.Errorf("deployments should not contain DevCharlie, got %v", deployments)
		}
	})

	t.Run("does not fail if deployment does not exist", func(t *testing.T) {
		t.Parallel()
		tmpDir, _ := setupTestCDKDir(t, map[string]any{
			"myproj-qualifier":     "myproj",
			"myproj-deployers":     []any{"Adam"},
			"myproj-dev-deployers": []any{},
			"myproj-deployments":   []any{"Prod", "Stag"},
		})

		var buf strings.Builder
		opts := deployerOptions{
			ProjectDir: tmpDir,
			Username:   "Adam",
			Output:     &buf,
		}

		err := doRemoveDeployer(t.Context(), opts)
		if err != nil {
			t.Fatalf("doRemoveDeployer() error = %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "Removed \"DevAdam\" from deployments") {
			t.Errorf("should not report removing non-existent deployment: %s", output)
		}
	})
}

func TestListDeployerProfiles(t *testing.T) {
	t.Run("parses profiles with qualifier prefix", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		awsDir := home + "/.aws"
		if err := os.MkdirAll(awsDir, 0o755); err != nil {
			t.Fatalf("failed to create .aws dir: %v", err)
		}

		content := `[default]
aws_access_key_id=DEFAULT

[myproj-adam]
aws_access_key_id=ADAM

[myproj-bob]
aws_access_key_id=BOB

[otherproj-charlie]
aws_access_key_id=CHARLIE
`
		if err := os.WriteFile(awsDir+"/credentials", []byte(content), 0o600); err != nil {
			t.Fatalf("failed to write credentials: %v", err)
		}

		profiles, err := listDeployerProfiles(t.Context(), t.TempDir(), "myproj")
		if err != nil {
			t.Fatalf("listDeployerProfiles() error = %v", err)
		}

		if len(profiles) != 2 {
			t.Errorf("expected 2 profiles, got %d: %v", len(profiles), profiles)
		}
	})
}
