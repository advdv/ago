package agcdkutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/go-playground/validator/v10"
)

// Scope-based convenience functions that retrieve Config from the construct tree.
// These provide ergonomic access deep in construct trees without passing *Config explicitly.

// IsPrimaryRegion checks if the given region is the primary region.
// Retrieves Config from the construct tree.
func IsPrimaryRegion(scope constructs.Construct, region string) bool {
	return ConfigFromScope(scope).IsPrimaryRegion(region)
}

// IsPrimaryRegionStack checks if the given stack is in the primary region.
// Retrieves Config from the construct tree.
func IsPrimaryRegionStack(scope constructs.Construct, stack awscdk.Stack) bool {
	return ConfigFromScope(scope).IsPrimaryRegionStack(stack)
}

// BaseDomainName returns the base domain name.
// Retrieves Config from the construct tree.
func BaseDomainName(scope constructs.Construct) string {
	return ConfigFromScope(scope).BaseDomainName
}

// BaseDomainNamePtr returns the base domain name as a jsii string pointer.
// Retrieves Config from the construct tree.
func BaseDomainNamePtr(scope constructs.Construct) *string {
	return ConfigFromScope(scope).BaseDomainNamePtr()
}

// AllRegions returns the primary region plus all secondary regions.
// Retrieves Config from the construct tree.
func AllRegions(scope constructs.Construct) []string {
	return ConfigFromScope(scope).AllRegions()
}

// RegionIdent returns the acronym identifier for a region.
// Retrieves Config from the construct tree.
func RegionIdent(scope constructs.Construct, region string) string {
	return ConfigFromScope(scope).RegionIdent(region)
}

// Qualifier returns the CDK qualifier.
// Retrieves Config from the construct tree.
func Qualifier(scope constructs.Construct) string {
	return ConfigFromScope(scope).Qualifier
}

// PrimaryRegion returns the primary region.
// Retrieves Config from the construct tree.
func PrimaryRegion(scope constructs.Construct) string {
	return ConfigFromScope(scope).PrimaryRegion
}

// Config holds all CDK context values validated upfront.
// It centralizes context reading and validation to provide clear error messages.
type Config struct {
	Prefix           string            `validate:"required"`
	Qualifier        string            `validate:"required,max=10"`
	PrimaryRegion    string            `validate:"required"`
	SecondaryRegions []string          `validate:"dive,required"`
	RegionIdents     map[string]string `validate:"required,dive,required"`
	Deployments      []string          `validate:"required,dive,required"`
	DeployerGroups   []string          // nil during bootstrap, optional
	BaseDomainName   string            `validate:"required,fqdn"`

	// From AppConfig (not context)
	DeployersGroup        string   `validate:"required"`
	RestrictedDeployments []string `validate:"dive,required"`
}

// AllRegions returns the primary region plus all secondary regions.
func (c *Config) AllRegions() []string {
	return append([]string{c.PrimaryRegion}, c.SecondaryRegions...)
}

// RegionIdent returns the acronym identifier for a region.
func (c *Config) RegionIdent(region string) string {
	return c.RegionIdents[region]
}

// IsPrimaryRegion checks if the given region is the primary region.
func (c *Config) IsPrimaryRegion(region string) bool {
	return region == c.PrimaryRegion
}

// IsPrimaryRegionStack checks if the given stack is in the primary region.
func (c *Config) IsPrimaryRegionStack(stack awscdk.Stack) bool {
	return *stack.Region() == c.PrimaryRegion
}

// BaseDomainNamePtr returns the base domain name as a jsii string pointer.
func (c *Config) BaseDomainNamePtr() *string {
	return jsii.String(c.BaseDomainName)
}

// configContextKey is the well-known key used to store validated Config in the construct tree.
const configContextKey = "__agcdkutil_config"

// StoreConfig stores a validated Config in the app's context so it can be retrieved
// anywhere in the construct tree via ConfigFromScope.
func StoreConfig(app awscdk.App, cfg *Config) {
	app.Node().SetContext(jsii.String(configContextKey), cfg)
}

// ConfigFromScope retrieves the validated Config from the construct tree.
// It panics if Config was not stored (i.e., SetupApp was not called).
func ConfigFromScope(scope constructs.Construct) *Config {
	val := scope.Node().TryGetContext(jsii.String(configContextKey))
	if val == nil {
		panic("agcdkutil.Config not found in construct tree - was SetupApp or StoreConfig called?")
	}
	cfg, ok := val.(*Config)
	if !ok {
		panic(fmt.Sprintf("agcdkutil.Config has unexpected type %T", val))
	}
	return cfg
}

// AllowedDeployments returns deployments the current deployer can access.
// Returns nil if DeployerGroups is nil (bootstrap mode).
func (c *Config) AllowedDeployments() []string {
	if c.DeployerGroups == nil {
		return nil
	}

	hasFullAccess := false
	for _, g := range c.DeployerGroups {
		if g == c.DeployersGroup {
			hasFullAccess = true
			break
		}
	}

	if hasFullAccess {
		return c.Deployments
	}

	allowed := make([]string, 0, len(c.Deployments))
	for _, d := range c.Deployments {
		isRestricted := false
		for _, r := range c.RestrictedDeployments {
			if d == r {
				isRestricted = true
				break
			}
		}
		if !isRestricted {
			allowed = append(allowed, d)
		}
	}
	return allowed
}

// NewConfig reads and validates all CDK context values.
// Returns an error if any required value is missing or invalid.
func NewConfig(scope constructs.Construct, cfg AppConfig) (*Config, error) {
	var readErrs []string

	c := &Config{
		Prefix:                cfg.Prefix,
		DeployersGroup:        cfg.DeployersGroup,
		RestrictedDeployments: cfg.RestrictedDeployments,
	}

	c.Qualifier, readErrs = readContextString(scope, cfg.Prefix+"qualifier", readErrs)
	c.PrimaryRegion, readErrs = readContextString(scope, cfg.Prefix+"primary-region", readErrs)
	c.SecondaryRegions, readErrs = readContextStringSlice(scope, cfg.Prefix+"secondary-regions", readErrs)
	c.Deployments, readErrs = readContextStringSlice(scope, cfg.Prefix+"deployments", readErrs)
	c.BaseDomainName, readErrs = readContextString(scope, cfg.Prefix+"base-domain-name", readErrs)

	// Read region idents for all known regions
	c.RegionIdents = make(map[string]string)
	regions := []string{}
	if c.PrimaryRegion != "" {
		regions = append(regions, c.PrimaryRegion)
	}
	regions = append(regions, c.SecondaryRegions...)

	for _, region := range regions {
		key := cfg.Prefix + "region-ident-" + region
		ident, errs := readContextString(scope, key, nil)
		if len(errs) > 0 {
			readErrs = append(readErrs, errs...)
		} else {
			c.RegionIdents[region] = ident
		}
	}

	// DeployerGroups is optional (nil during bootstrap)
	c.DeployerGroups = readOptionalDeployerGroups(scope, cfg.Prefix)

	if len(readErrs) > 0 {
		return nil, fmt.Errorf("CDK context read errors:\n  - %s", strings.Join(readErrs, "\n  - "))
	}

	// Validate using struct tags and struct-level validation
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterStructValidation(validateConfigRegionIdents, Config{})

	if err := validate.Struct(c); err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			msgs := make([]string, 0, len(validationErrs))
			for _, e := range validationErrs {
				msgs = append(msgs, formatValidationError(e))
			}
			return nil, fmt.Errorf("CDK context validation errors:\n  - %s", strings.Join(msgs, "\n  - "))
		}
		return nil, fmt.Errorf("CDK context validation failed: %w", err)
	}

	return c, nil
}

// validateConfigRegionIdents ensures RegionIdents has entries for all regions.
func validateConfigRegionIdents(sl validator.StructLevel) {
	cfg := sl.Current().Interface().(Config)

	// Check primary region has ident
	if cfg.PrimaryRegion != "" {
		if _, ok := cfg.RegionIdents[cfg.PrimaryRegion]; !ok {
			sl.ReportError(cfg.RegionIdents, "RegionIdents", "RegionIdents",
				"missing_region_ident", cfg.PrimaryRegion)
		}
	}

	// Check all secondary regions have idents
	for _, region := range cfg.SecondaryRegions {
		if _, ok := cfg.RegionIdents[region]; !ok {
			sl.ReportError(cfg.RegionIdents, "RegionIdents", "RegionIdents",
				"missing_region_ident", region)
		}
	}
}

func formatValidationError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "max":
		return fmt.Sprintf("%s exceeds maximum length of %s (got %q)", e.Field(), e.Param(), e.Value())
	case "fqdn":
		return fmt.Sprintf("%s must be a valid domain name (got %q)", e.Field(), e.Value())
	case "missing_region_ident":
		return fmt.Sprintf("%s is missing entry for region %q", e.Field(), e.Param())
	default:
		return fmt.Sprintf("%s failed validation %q", e.Field(), e.Tag())
	}
}

func readContextString(scope constructs.Construct, key string, errs []string) (string, []string) {
	val := scope.Node().TryGetContext(jsii.String(key))
	if val == nil {
		return "", append(errs, fmt.Sprintf("context key %q is not set", key))
	}
	s, ok := val.(string)
	if !ok {
		return "", append(errs, fmt.Sprintf("context key %q must be a string, got %T", key, val))
	}
	return s, errs
}

func readContextStringSlice(scope constructs.Construct, key string, errs []string) ([]string, []string) {
	val := scope.Node().TryGetContext(jsii.String(key))
	if val == nil {
		return nil, append(errs, fmt.Sprintf("context key %q is not set", key))
	}

	slice, ok := val.([]any)
	if !ok {
		return nil, append(errs, fmt.Sprintf("context key %q must be an array, got %T", key, val))
	}

	result := make([]string, 0, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			return nil, append(errs, fmt.Sprintf("context key %q[%d] must be a string, got %T", key, i, v))
		}
		result = append(result, s)
	}
	return result, errs
}

func readOptionalDeployerGroups(scope constructs.Construct, prefix string) []string {
	val := scope.Node().TryGetContext(jsii.String(prefix + "deployer-groups"))
	if val == nil {
		return nil
	}
	str, ok := val.(string)
	if !ok || str == "" {
		return nil
	}
	return strings.Fields(str)
}
