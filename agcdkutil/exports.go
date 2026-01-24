package agcdkutil

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// PreserveExport creates an explicit CfnOutput with a specific export name to maintain
// backward compatibility when cross-stack references are removed. This prevents
// CloudFormation from failing with "Cannot delete export X as it is in use" errors.
//
// Use this when you need to remove a cross-stack dependency but the importing stack
// hasn't been updated yet. Once all importing stacks are updated, these exports can
// be removed.
func PreserveExport(scope constructs.Construct, id string, exportName string, value *string) {
	awscdk.NewCfnOutput(scope, jsii.String(id), &awscdk.CfnOutputProps{
		Value:      value,
		ExportName: jsii.String(exportName),
	})
}
