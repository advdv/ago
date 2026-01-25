//nolint:paralleltest // this test doesn't need parallel execution
package agcdkutil

import (
	"testing"
)

func TestRegionIdentFor(t *testing.T) {
	tests := []struct {
		region    string
		wantIdent string
	}{
		{"us-east-1", "Use1"},
		{"us-west-2", "Usw2"},
		{"eu-west-1", "Euw1"},
		{"eu-central-1", "Euc1"},
		{"ap-northeast-1", "Apn1"},
		{"ap-southeast-1", "Ase1"},
		{"sa-east-1", "Sae1"},
		{"eusc-de-east-1", "Ede1"},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			got := RegionIdentFor(tt.region)
			if got != tt.wantIdent {
				t.Errorf("RegionIdentFor(%q) = %q, want %q", tt.region, got, tt.wantIdent)
			}
		})
	}
}

func TestRegionIdentForPanicsOnUnknown(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unknown region")
		}
	}()

	RegionIdentFor("unknown-region-1")
}

func TestIsKnownRegion(t *testing.T) {
	if !IsKnownRegion("us-east-1") {
		t.Error("us-east-1 should be known")
	}
	if IsKnownRegion("unknown-region-1") {
		t.Error("unknown-region-1 should not be known")
	}
}

func TestRegionIdentLower(t *testing.T) {
	if got := RegionIdentLower("us-east-1"); got != "use1" {
		t.Errorf("RegionIdentLower(us-east-1) = %q, want %q", got, "use1")
	}
	if got := RegionIdentLower("eu-west-1"); got != "euw1" {
		t.Errorf("RegionIdentLower(eu-west-1) = %q, want %q", got, "euw1")
	}
}
