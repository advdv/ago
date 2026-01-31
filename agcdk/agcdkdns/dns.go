// Package agcdkdns provides a reusable Route53 hosted zone construct for multi-region CDK deployments.
//
// The DNS construct creates a hosted zone in the primary region and stores its ID
// in SSM Parameter Store. Secondary regions look up the stored ID to reference
// the same zone without recreating it.
package agcdkdns

import (
	"github.com/advdv/ago/agcdk/agcdkparams"
	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// NameServersOutputKey is the CloudFormation output key for the hosted zone's NS records.
// Use this with `aws cloudformation describe-stacks` to retrieve the name servers.
const NameServersOutputKey = "HostedZoneNameServers"

const paramsNamespace = "dns"

// DNS provides access to a Route53 hosted zone that works across regions.
type DNS interface {
	// HostedZone returns the Route53 hosted zone.
	// In the primary region, this is the actual zone.
	// In secondary regions, this is a reference to the primary zone.
	HostedZone() awsroute53.IHostedZone
}

// Props configures the DNS construct.
type Props struct {
	// ZoneDomainName is the domain name for the hosted zone (e.g., "example.com").
	// If nil, uses the base domain name from config.
	ZoneDomainName *string
}

type dns struct {
	hostedZone awsroute53.IHostedZone
}

// New creates a DNS construct that manages a Route53 hosted zone.
//
// In the primary region: Creates a new hosted zone and stores the zone ID
// in SSM Parameter Store for cross-region access.
//
// In secondary regions: Looks up the zone ID from SSM and creates a reference
// to the existing hosted zone.
func New(scope constructs.Construct, props Props) DNS {
	scope = constructs.NewConstruct(scope, jsii.String("DNS"))
	con := &dns{}

	zoneName := props.ZoneDomainName
	if zoneName == nil {
		zoneName = agcdkutil.BaseDomainNamePtr(scope)
	}

	region := *awscdk.Stack_Of(scope).Region()

	if agcdkutil.IsPrimaryRegion(scope, region) {
		hostedZone := awsroute53.NewHostedZone(scope, jsii.String("HostedZone"),
			&awsroute53.HostedZoneProps{
				ZoneName: zoneName,
			})
		con.hostedZone = hostedZone

		agcdkparams.Store(scope, "HostedZoneIDParam", paramsNamespace, "hosted-zone-id",
			hostedZone.HostedZoneId())

		awscdk.NewCfnOutput(awscdk.Stack_Of(scope), jsii.String(NameServersOutputKey), &awscdk.CfnOutputProps{
			Value:       awscdk.Fn_Join(jsii.String(","), hostedZone.HostedZoneNameServers()),
			Description: jsii.String("Comma-separated list of NS records for DNS delegation"),
		})
	} else {
		hostedZoneID := agcdkparams.Lookup(scope, "LookupHostedZoneID",
			paramsNamespace, "hosted-zone-id", "hosted-zone-id-lookup")

		con.hostedZone = awsroute53.HostedZone_FromHostedZoneAttributes(scope, jsii.String("HostedZone"),
			&awsroute53.HostedZoneAttributes{
				HostedZoneId: hostedZoneID,
				ZoneName:     zoneName,
			})
	}

	return con
}

func (d *dns) HostedZone() awsroute53.IHostedZone {
	return d.hostedZone
}
