package initwizard

import (
	"github.com/advdv/ago/agcdkutil"
	"github.com/charmbracelet/huh"
	"github.com/cockroachdb/errors"
)

type FormBuilder interface {
	Build(defaultIdent string, result *Result) *huh.Form
}

type formBuilder struct{}

func NewFormBuilder() FormBuilder {
	return &formBuilder{}
}

func (b *formBuilder) Build(defaultIdent string, result *Result) *huh.Form {
	*result = DefaultResult(defaultIdent)
	return huh.NewForm(
		huh.NewGroup(
			b.managementProfileInput(&result.ManagementProfile),
			b.projectIdentInput(&result.ProjectIdent),
			b.primaryRegionSelect(&result.PrimaryRegion),
			b.secondaryRegionsSelect(&result.PrimaryRegion, &result.SecondaryRegions),
			b.initialDeployerInput(&result.InitialDeployer),
			b.terraformCloudOrgInput(&result.TerraformCloudOrg),
		),
	)
}

func (b *formBuilder) projectIdentInput(value *string) *huh.Input {
	return huh.NewInput().
		Title("Project identifier").
		Description("Used as prefix for AWS resources and stack names").
		Value(value).
		Validate(ValidateProjectIdent)
}

func (b *formBuilder) primaryRegionSelect(value *string) *huh.Select[string] {
	regions := agcdkutil.AllKnownRegions()
	return huh.NewSelect[string]().
		Title("Primary AWS region").
		Description("Main region for deployments").
		Options(huh.NewOptions(regions...)...).
		Value(value)
}

func (b *formBuilder) secondaryRegionsSelect(primaryRegion *string, value *[]string) *huh.MultiSelect[string] {
	return huh.NewMultiSelect[string]().
		Title("Secondary AWS regions").
		Description("Additional regions for multi-region deployments (optional)").
		OptionsFunc(func() []huh.Option[string] {
			var opts []huh.Option[string]
			for _, r := range agcdkutil.AllKnownRegions() {
				if r != *primaryRegion {
					opts = append(opts, huh.NewOption(r, r))
				}
			}
			return opts
		}, primaryRegion).
		Value(value)
}

func (b *formBuilder) managementProfileInput(value *string) *huh.Input {
	return huh.NewInput().
		Title("Management profile").
		Description("AWS profile for the management account (used to create project account)").
		Value(value)
}

func (b *formBuilder) initialDeployerInput(value *string) *huh.Input {
	return huh.NewInput().
		Title("Initial deployer").
		Description("Username for the first deployer to add to the project").
		Value(value)
}

func (b *formBuilder) terraformCloudOrgInput(value *string) *huh.Input {
	return huh.NewInput().
		Title("Terraform Cloud organization").
		Description("Organization name in Terraform Cloud for remote state management").
		Value(value)
}

func ValidateProjectIdent(s string) error {
	if s == "" {
		return errors.New("project identifier is required")
	}
	if len(s) > 20 {
		return errors.New("project identifier must be 20 characters or less")
	}
	for _, c := range s {
		if !IsValidIdentChar(c) {
			return errors.Newf("invalid character %q: use lowercase letters, numbers, and hyphens only", c)
		}
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return errors.New("project identifier cannot start or end with a hyphen")
	}
	return nil
}

func IsValidIdentChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}
