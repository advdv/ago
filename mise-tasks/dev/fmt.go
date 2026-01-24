/*usr/bin/env go run "$0" "$@" ; exit #*/

// MISE description="Format Go code using golangci-lint"

//go:build ignore

package main

import (
	"os"

	"github.com/bitfield/script"
)

func main() {
	_, err := script.Exec("golangci-lint fmt ./...").Stdout()
	if err != nil {
		os.Exit(1)
	}
}
