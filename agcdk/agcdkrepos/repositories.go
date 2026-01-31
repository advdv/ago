// Package agcdkrepos provides reusable ECR repository constructs for multi-region CDK deployments.
//
// The Repositories construct creates ECR repositories in each region with immutable image tags,
// lifecycle policies, and cross-region replication. Unlike DNS which only exists in
// the primary region, repositories are created in every region independently.
//
// In the primary region, a replication configuration is also created to automatically
// sync images to all secondary regions.
package agcdkrepos

import (
	"fmt"

	"github.com/advdv/ago/agcdkutil"
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsecr"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// RepositoryURIOutputKey is the CloudFormation output key for the repository URI.
// Use this to configure ko for image builds:
//
//	export KO_DOCKER_REPO=$(aws cloudformation describe-stacks --stack-name MyStack \
//	  --query 'Stacks[0].Outputs[?OutputKey==`RepositoryURI`].OutputValue' --output text)
const RepositoryURIOutputKey = "RepositoryURI"

const defaultLifecycleMaxImages = 100

// Repositories provides access to ECR repositories.
type Repositories interface {
	// MainRepository returns the main ECR repository for this region.
	MainRepository() awsecr.IRepository
}

// Props configures the Repositories construct.
type Props struct {
	// RepositoryName overrides the default repository name.
	// If nil, uses "{qualifier}-main".
	RepositoryName *string

	// LifecycleMaxImages is the maximum number of images to retain.
	// Defaults to 100 if not specified.
	LifecycleMaxImages *float64
}

type repositories struct {
	repository awsecr.IRepository
}

// New creates a Repositories construct that manages ECR repositories across regions.
//
// In all regions: Creates a repository with immutable tags, lifecycle policy,
// and removal policy that allows deletion.
//
// In the primary region only: Also creates a replication configuration to
// automatically sync images to all secondary regions.
func New(scope constructs.Construct, props Props) Repositories {
	scope = constructs.NewConstruct(scope, jsii.String("Repositories"))
	con := &repositories{}

	stack := awscdk.Stack_Of(scope)
	region := *stack.Region()
	account := *stack.Account()
	qualifier := agcdkutil.Qualifier(scope)

	repoName := props.RepositoryName
	if repoName == nil {
		repoName = jsii.String(fmt.Sprintf("%s-main", qualifier))
	}

	maxImages := props.LifecycleMaxImages
	if maxImages == nil {
		maxImages = jsii.Number(defaultLifecycleMaxImages)
	}

	con.repository = awsecr.NewRepository(scope, jsii.String("MainRepository"), &awsecr.RepositoryProps{
		RepositoryName:     repoName,
		ImageTagMutability: awsecr.TagMutability_IMMUTABLE,
		RemovalPolicy:      awscdk.RemovalPolicy_DESTROY,
		EmptyOnDelete:      jsii.Bool(true),
		LifecycleRules: &[]*awsecr.LifecycleRule{{
			MaxImageCount: maxImages,
			Description:   jsii.String(fmt.Sprintf("Keep last %.0f images", *maxImages)),
		}},
	})

	if agcdkutil.IsPrimaryRegion(scope, region) {
		awscdk.NewCfnOutput(stack, jsii.String(RepositoryURIOutputKey), &awscdk.CfnOutputProps{
			Value:       con.repository.RepositoryUri(),
			Description: jsii.String("ECR repository URI for ko (export as KO_DOCKER_REPO)"),
		})
		cfg := agcdkutil.ConfigFromScope(scope)
		destinations := make(
			[]*awsecr.CfnReplicationConfiguration_ReplicationDestinationProperty,
			0, len(cfg.SecondaryRegions))
		for _, secondaryRegion := range cfg.SecondaryRegions {
			destinations = append(destinations, &awsecr.CfnReplicationConfiguration_ReplicationDestinationProperty{
				Region:     jsii.String(secondaryRegion),
				RegistryId: jsii.String(account),
			})
		}

		if len(destinations) > 0 {
			awsecr.NewCfnReplicationConfiguration(scope, jsii.String("ReplicationConfig"),
				&awsecr.CfnReplicationConfigurationProps{
					ReplicationConfiguration: &awsecr.CfnReplicationConfiguration_ReplicationConfigurationProperty{
						Rules: &[]*awsecr.CfnReplicationConfiguration_ReplicationRuleProperty{{
							Destinations: &destinations,
							RepositoryFilters: &[]*awsecr.CfnReplicationConfiguration_RepositoryFilterProperty{{
								FilterType: jsii.String("PREFIX_MATCH"),
								Filter:     con.repository.RepositoryName(),
							}},
						}},
					},
				})
		}
	}

	return con
}

func (r *repositories) MainRepository() awsecr.IRepository {
	return r.repository
}
