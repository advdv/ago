// Package agcdkcerts provides a reusable ACM wildcard certificate construct
// for multi-region CDK deployments.
//
// The certificate uses DNS validation via the provided Route53 hosted zone.
// This construct should only be created after DNS has been validated and is
// operational (i.e., after SharedBase validation is complete).
package agcdkcerts

import (
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// Certificates provides access to a wildcard ACM certificate.
type Certificates interface {
	// WildcardCertificate returns the ACM wildcard certificate (*.domain.com).
	// Use this for CloudFront, API Gateway, ALB, etc.
	WildcardCertificate() awscertificatemanager.ICertificate
}

// Props configures the Certificates construct.
type Props struct {
	// HostedZone is the Route53 hosted zone used for DNS validation.
	// Required.
	HostedZone awsroute53.IHostedZone
}

type certificates struct {
	certificate awscertificatemanager.ICertificate
}

// New creates a Certificates construct with a wildcard ACM certificate.
//
// The certificate is created for *.{zoneName} and uses DNS validation
// via the provided hosted zone. DNS validation requires the hosted zone
// to be properly delegated and operational.
//
// Each region gets its own certificate since ACM certificates are regional.
// The certificate validates against the same Route53 hosted zone.
func New(scope constructs.Construct, props Props) Certificates {
	scope = constructs.NewConstruct(scope, jsii.String("Certificates"))
	con := &certificates{}

	con.certificate = awscertificatemanager.NewCertificate(scope, jsii.String("WildcardCertificate"),
		&awscertificatemanager.CertificateProps{
			DomainName: jsii.String("*." + *props.HostedZone.ZoneName()),
			Validation: awscertificatemanager.CertificateValidation_FromDns(props.HostedZone),
		})

	return con
}

func (c *certificates) WildcardCertificate() awscertificatemanager.ICertificate {
	return c.certificate
}
