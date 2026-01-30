// Package agcdksharedbase provides the foundational shared infrastructure construct
// for multi-region CDK deployments.
//
// SharedBase encapsulates resources that must be deployed and validated before
// other shared or deployment resources can work. Currently this includes:
//   - DNS: Route53 hosted zone (must be delegated before dependent resources deploy)
//   - Certificate: ACM wildcard certificate (only created after DNS is validated)
//
// The construct checks validation flags from context (e.g., "dns-delegated"):
//   - When not all validated: Only creates foundational resources, returns early.
//   - When all validated: Full infrastructure available.
package agcdksharedbase

import (
	"github.com/advdv/ago/agcdk/agcdkcert"
	"github.com/advdv/ago/agcdk/agcdkdns"
	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// SharedBase provides access to foundational shared infrastructure.
type SharedBase interface {
	// DNS returns the DNS construct.
	// Always created, even before validation.
	DNS() agcdkdns.DNS

	// Certificate returns the Certificate construct, or nil if not yet validated.
	// Only available after IsValidated() returns true.
	Certificate() agcdkcert.Certificate

	// IsValidated returns true if DNS has been validated and all
	// foundational resources are available.
	IsValidated() bool
}

// Props configures the SharedBase construct.
type Props struct {
	// DNSProps configures the DNS construct.
	// Optional: defaults will use base domain name from config.
	DNSProps *agcdkdns.Props
}

type sharedBase struct {
	dns         agcdkdns.DNS
	certificate agcdkcert.Certificate
	validated   bool
}

// New creates a SharedBase construct with foundational infrastructure.
//
// The construct checks validation flags to determine if all foundational
// infrastructure is ready. Currently requires:
//   - DNS delegation complete (dns-delegated context flag)
//
// Consumers should check IsValidated() before creating dependent resources.
func New(scope constructs.Construct, props Props) SharedBase {
	scope = constructs.NewConstruct(scope, jsii.String("SharedBase"))
	base := &sharedBase{}

	dnsProps := agcdkdns.Props{}
	if props.DNSProps != nil {
		dnsProps = *props.DNSProps
	}
	base.dns = agcdkdns.New(scope, dnsProps)

	if !isValidated(scope) {
		return base
	}

	base.validated = true

	base.certificate = agcdkcert.New(scope, agcdkcert.Props{
		HostedZone: base.dns.HostedZone(),
	})

	return base
}

// isValidated checks all required validation flags.
// Add additional checks here as more foundational infrastructure is added.
func isValidated(scope constructs.Construct) bool {
	return agcdkutil.DNSDelegated(scope)
}

func (s *sharedBase) DNS() agcdkdns.DNS {
	return s.dns
}

func (s *sharedBase) Certificate() agcdkcert.Certificate {
	return s.certificate
}

func (s *sharedBase) IsValidated() bool {
	return s.validated
}
