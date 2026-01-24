/*usr/bin/env go run "$0" "$@" ; exit #*/

// MISE description="Lint Go code using golangci-lint"

package main

import (
	"os"

	"github.com/bitfield/script"
)

func main() {
	_, err := script.Exec("golangci-lint run ./...").Stdout()
	if err != nil {
		os.Exit(1)
	}
}
