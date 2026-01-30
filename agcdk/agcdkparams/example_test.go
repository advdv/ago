package agcdkparams_test

import (
	"github.com/advdv/ago/agcdk/agcdkparams"
	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscognito"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/jsii-runtime-go"
)

// Example_dnsConstruct demonstrates storing and looking up DNS-related parameters.
// The namespace "dns" groups all DNS-related values together.
func Example_dnsConstruct() {
	defer jsii.Close()

	ctx := map[string]any{
		"myapp-qualifier":         "myapp",
		"myapp-primary-region":    "us-east-1",
		"myapp-secondary-regions": []any{"eu-west-1"},
		"myapp-deployments":       []any{"Dev"},
		"myapp-base-domain-name":  "example.com",
	}

	app := awscdk.NewApp(&awscdk.AppProps{Context: &ctx})
	cfg, _ := agcdkutil.NewConfig(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "deployers",
	})
	agcdkutil.StoreConfig(app, cfg)

	stack := awscdk.NewStack(app, jsii.String("DnsStack"), &awscdk.StackProps{
		Env: &awscdk.Environment{Region: jsii.String("us-east-1")},
	})

	const namespace = "dns"

	if agcdkutil.IsPrimaryRegion(stack, "us-east-1") {
		zone := awsroute53.NewHostedZone(stack, jsii.String("HostedZone"),
			&awsroute53.HostedZoneProps{
				ZoneName: jsii.String("example.com"),
			})

		agcdkparams.Store(stack, "HostedZoneIDParam", namespace, "hosted-zone-id", zone.HostedZoneId())
		agcdkparams.Store(stack, "HostedZoneArnParam", namespace, "hosted-zone-arn", zone.HostedZoneArn())
	} else {
		hostedZoneID := agcdkparams.Lookup(stack, "LookupHostedZoneID", namespace, "hosted-zone-id", "hosted-zone-id-lookup")
		_ = awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("HostedZone"),
			&awsroute53.HostedZoneAttributes{
				HostedZoneId: hostedZoneID,
				ZoneName:     jsii.String("example.com"),
			})
	}
	// Output:
}

// Example_identityConstruct demonstrates storing multiple related parameters
// under an "identity" namespace for Cognito resources.
func Example_identityConstruct() {
	defer jsii.Close()

	ctx := map[string]any{
		"myapp-qualifier":         "myapp",
		"myapp-primary-region":    "us-east-1",
		"myapp-secondary-regions": []any{"eu-west-1"},
		"myapp-deployments":       []any{"Dev"},
		"myapp-base-domain-name":  "example.com",
	}

	app := awscdk.NewApp(&awscdk.AppProps{Context: &ctx})
	cfg, _ := agcdkutil.NewConfig(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "deployers",
	})
	agcdkutil.StoreConfig(app, cfg)

	stack := awscdk.NewStack(app, jsii.String("IdentityStack"), &awscdk.StackProps{
		Env: &awscdk.Environment{Region: jsii.String("us-east-1")},
	})

	const namespace = "identity"

	if agcdkutil.IsPrimaryRegion(stack, "us-east-1") {
		userPool := awscognito.NewUserPool(stack, jsii.String("UserPool"),
			&awscognito.UserPoolProps{
				UserPoolName: jsii.String("my-user-pool"),
			})

		client := userPool.AddClient(jsii.String("WebClient"),
			&awscognito.UserPoolClientOptions{
				UserPoolClientName: jsii.String("web-client"),
			})

		agcdkparams.Store(stack, "StoreUserPoolID", namespace, "user-pool-id", userPool.UserPoolId())
		agcdkparams.Store(stack, "StoreUserPoolArn", namespace, "user-pool-arn", userPool.UserPoolArn())
		agcdkparams.Store(stack, "StoreWebClientID", namespace, "web-client-id", client.UserPoolClientId())
	} else {
		userPoolID := agcdkparams.Lookup(stack, "LookupUserPoolID", namespace, "user-pool-id", "user-pool-id-lookup")
		_ = awscognito.UserPool_FromUserPoolId(stack, jsii.String("UserPool"), userPoolID)
	}
	// Output:
}

// Example_multipleNamespaces demonstrates using separate namespaces for different
// domains of resources. This keeps parameters organized and prevents naming collisions.
func Example_multipleNamespaces() {
	defer jsii.Close()

	ctx := map[string]any{
		"myapp-qualifier":         "myapp",
		"myapp-primary-region":    "us-east-1",
		"myapp-secondary-regions": []any{"eu-west-1"},
		"myapp-deployments":       []any{"Dev"},
		"myapp-base-domain-name":  "example.com",
	}

	app := awscdk.NewApp(&awscdk.AppProps{Context: &ctx})
	cfg, _ := agcdkutil.NewConfig(app, agcdkutil.AppConfig{
		Prefix:         "myapp-",
		DeployersGroup: "deployers",
	})
	agcdkutil.StoreConfig(app, cfg)

	stack := awscdk.NewStack(app, jsii.String("MultiStack"), &awscdk.StackProps{
		Env: &awscdk.Environment{Region: jsii.String("eu-west-1")},
	})

	dnsHostedZoneID := agcdkparams.Lookup(stack, "LookupDnsHostedZoneID", "dns", "hosted-zone-id", "dns-hosted-zone-lookup")

	userPoolID := agcdkparams.Lookup(stack, "LookupUserPoolID", "identity", "user-pool-id", "identity-user-pool-lookup")

	crewPoolID := agcdkparams.Lookup(stack, "LookupCrewPoolID", "crew-identity", "user-pool-id", "crew-user-pool-lookup")

	_ = dnsHostedZoneID
	_ = userPoolID
	_ = crewPoolID
	// Output:
}
