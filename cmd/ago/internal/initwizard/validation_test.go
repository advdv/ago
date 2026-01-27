package initwizard_test

import (
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/initwizard"
)

func TestValidateProjectIdent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "myproject", wantErr: false},
		{name: "valid with hyphen", input: "my-project", wantErr: false},
		{name: "valid with numbers", input: "project123", wantErr: false},
		{name: "valid mixed", input: "my-project-123", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "too long", input: "abcdefghijklmnopqrstu", wantErr: true},
		{name: "uppercase", input: "MyProject", wantErr: true},
		{name: "spaces", input: "my project", wantErr: true},
		{name: "underscore", input: "my_project", wantErr: true},
		{name: "starts with hyphen", input: "-project", wantErr: true},
		{name: "ends with hyphen", input: "project-", wantErr: true},
		{name: "special chars", input: "project!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := initwizard.ValidateProjectIdent(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectIdent(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestIsValidIdentChar(t *testing.T) {
	t.Parallel()

	valid := []rune{'a', 'z', '0', '9', '-'}
	for _, c := range valid {
		if !initwizard.IsValidIdentChar(c) {
			t.Errorf("expected %q to be valid", c)
		}
	}

	invalid := []rune{'A', 'Z', '_', ' ', '!', '@'}
	for _, c := range invalid {
		if initwizard.IsValidIdentChar(c) {
			t.Errorf("expected %q to be invalid", c)
		}
	}
}
