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
	qualifier := strcase.ToLowerCamel(fmt.Sprintf("%s-%s", qual, regionAcronym))
	stackName := jsii.Sprintf("%sShared", qualifier)

	description := jsii.String(fmt.Sprintf("%s (region: %s)",
		qualifier, region))
	if len(deploymentIdent) > 0 && deploymentIdent[0] != "" {
		tident := deploymentIdent[0]
		if strings.ToUpper(string(tident[0])) != string(tident[0]) {
			panic("deployment identifier must start with a upper-case letter, got: " + tident)
		}

		description = jsii.String(fmt.Sprintf("%s (region: %s, deployment: %s)",
			qualifier, region, tident))

		stackName = jsii.Sprintf("%s%s", qualifier, tident)
	} else if len(deploymentIdent) > 0 {
		panic("invalid deploymentIdent: " + deploymentIdent[0])
	}

	stack := awscdk.NewStack(scope, stackName, &awscdk.StackProps{
		Env: &awscdk.Environment{
			Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
			Region:  jsii.String(region),
		},
		Description: description,
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
