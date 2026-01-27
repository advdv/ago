package initwizard_test

import (
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/initwizard"
	"github.com/charmbracelet/huh"
	"github.com/cockroachdb/errors"
)

type mockRunner struct {
	runFunc func(*huh.Form) error
}

func (m *mockRunner) Run(form *huh.Form) error {
	if m.runFunc != nil {
		return m.runFunc(form)
	}
	return nil
}

func TestWizard_Run(t *testing.T) {
	t.Parallel()

	t.Run("returns result from successful form run", func(t *testing.T) {
		t.Parallel()
		builder := initwizard.NewFormBuilder()
		runner := &mockRunner{
			runFunc: func(_ *huh.Form) error {
				return nil
			},
		}

		wizard := initwizard.New(builder, runner)
		result, err := wizard.Run("testproject")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ProjectIdent != "testproject" {
			t.Errorf("expected project ident 'testproject', got %q", result.ProjectIdent)
		}
		if result.PrimaryRegion != "eu-central-1" {
			t.Errorf("expected primary region 'eu-central-1', got %q", result.PrimaryRegion)
		}
	})

	t.Run("propagates runner error", func(t *testing.T) {
		t.Parallel()
		builder := initwizard.NewFormBuilder()
		expectedErr := errors.New("user aborted")
		runner := &mockRunner{
			runFunc: func(_ *huh.Form) error {
				return expectedErr
			},
		}

		wizard := initwizard.New(builder, runner)
		_, err := wizard.Run("test")

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	})
}
