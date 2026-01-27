package initwizard_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/initwizard"
	"github.com/charmbracelet/huh"
)

func TestAccessibleRunner(t *testing.T) {
	t.Parallel()

	t.Run("runs form in accessible mode", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		input := strings.NewReader("testvalue\n")

		var value string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Test input").Value(&value),
			),
		)

		runner := initwizard.NewAccessibleRunner(&output, input)
		err := runner.Run(form)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if value != "testvalue" {
			t.Errorf("expected value 'testvalue', got %q", value)
		}
		if !strings.Contains(output.String(), "Test input") {
			t.Errorf("expected output to contain 'Test input', got %q", output.String())
		}
	})
}
