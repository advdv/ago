package agcdkutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
	"github.com/iancoleman/strcase"
)

// SharedStackName returns the CloudFormation stack name for a shared stack.
// This is the canonical function for generating shared stack names.
func SharedStackName(qualifier, regionIdent string) string {
	base := strcase.ToLowerCamel(fmt.Sprintf("%s-%s", qualifier, regionIdent))
	return base + "Shared"
}

// DeploymentStackName returns the CloudFormation stack name for a deployment stack.
// This is the canonical function for generating deployment stack names.
func DeploymentStackName(qualifier, regionIdent, deploymentIdent string) string {
	base := strcase.ToLowerCamel(fmt.Sprintf("%s-%s", qualifier, regionIdent))
	return base + deploymentIdent
}

// NewStack creates a new CDK Stack, either shared or multi-deployment.
//
// Deprecated: Use NewStackFromConfig instead for upfront validation.
func NewStack(
	scope constructs.Construct, prefix, region string, deploymentIdent ...string,
) awscdk.Stack {
	qual := QualifierFromContext(scope, prefix)
	regionAcronym := RegionAcronymIdentFromContext(scope, prefix, region)
	return newStackInternal(scope, qual, regionAcronym, region, deploymentIdent...)
}

// NewStackFromConfig creates a new CDK Stack using a validated Config.
func NewStackFromConfig(
	scope constructs.Construct, cfg *Config, region string, deploymentIdent ...string,
) awscdk.Stack {
	return newStackInternal(scope, cfg.Qualifier, cfg.RegionIdent(region), region, deploymentIdent...)
}

func newStackInternal(
	scope constructs.Construct, qual, regionAcronym, region string, deploymentIdent ...string,
) awscdk.Stack {
	var stackName string
	var description string

	baseIdent := strcase.ToLowerCamel(fmt.Sprintf("%s-%s", qual, regionAcronym))

	switch {
	case len(deploymentIdent) > 0 && deploymentIdent[0] != "":
		dident := deploymentIdent[0]
		if strings.ToUpper(string(dident[0])) != string(dident[0]) {
			panic("deployment identifier must start with a upper-case letter, got: " + dident)
		}

		stackName = DeploymentStackName(qual, regionAcronym, dident)
		description = fmt.Sprintf("%s (region: %s, deployment: %s)", baseIdent, region, dident)
	case len(deploymentIdent) > 0:
		panic("invalid deploymentIdent: " + deploymentIdent[0])
	default:
		stackName = SharedStackName(qual, regionAcronym)
		description = fmt.Sprintf("%s (region: %s)", baseIdent, region)
	}

	stack := awscdk.NewStack(scope, jsii.String(stackName), &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
			Region:  jsii.String(region),
		},
		Description: jsii.String(description),
		Synthesizer: awscdk.NewDefaultStackSynthesizer(&awscdk.DefaultStackSynthesizerProps{
			Qualifier: jsii.String(qual),
		}),
	})

	awscdk.Annotations_Of(stack).AcknowledgeWarning(
		jsii.String("@aws-cdk/aws-lambda-go-alpha:goBuildFlagsSecurityWarning"),
		jsii.String("Build flags are controlled by agcdkutil.ReproducibleGoBundling and are safe"),
	)

	return stack
}
