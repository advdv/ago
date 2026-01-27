package initwizard_test

import (
	"testing"

	"github.com/advdv/ago/cmd/ago/internal/initwizard"
)

func TestFormBuilder_Build(t *testing.T) {
	t.Parallel()

	t.Run("creates form with default values", func(t *testing.T) {
		t.Parallel()
		builder := initwizard.NewFormBuilder()
		var result initwizard.Result
		form := builder.Build("myproject", &result)

		if form == nil {
			t.Fatal("expected form to be created")
		}
		if result.ProjectIdent != "myproject" {
			t.Errorf("expected default project ident 'myproject', got %q", result.ProjectIdent)
		}
		if result.PrimaryRegion != "eu-central-1" {
			t.Errorf("expected default primary region 'eu-central-1', got %q", result.PrimaryRegion)
		}
		if len(result.SecondaryRegions) != 1 || result.SecondaryRegions[0] != "eu-north-1" {
			t.Errorf("expected default secondary regions ['eu-north-1'], got %v", result.SecondaryRegions)
		}
	})

	t.Run("uses provided default ident", func(t *testing.T) {
		t.Parallel()
		builder := initwizard.NewFormBuilder()
		var result initwizard.Result
		builder.Build("custom-project", &result)

		if result.ProjectIdent != "custom-project" {
			t.Errorf("expected project ident 'custom-project', got %q", result.ProjectIdent)
		}
	})
}
