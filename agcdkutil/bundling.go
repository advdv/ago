package agcdkutil

import (
	"github.com/aws/aws-cdk-go/awscdklambdagoalpha/v2"
	"github.com/aws/jsii-runtime-go"
)

// ReproducibleGoBundling returns BundlingOptions configured for 100% reproducible builds.
// Same source code will always produce identical binaries, preventing unnecessary redeploys.
func ReproducibleGoBundling() *awscdklambdagoalpha.BundlingOptions {
	return &awscdklambdagoalpha.BundlingOptions{
		GoBuildFlags: jsii.Strings(
			"-trimpath",          // removes filesystem paths from binary
			"-ldflags=-buildid=", // clears timestamp-based build ID
			"-buildvcs=false",    // excludes git commit hash, allowing identical builds across commits
		),
		Environment: &map[string]*string{
			"CGO_ENABLED": jsii.String("0"), // pure Go, no C toolchain variance
		},
	}
}
