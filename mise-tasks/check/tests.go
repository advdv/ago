/*usr/bin/env go run "$0" "$@" ; exit #*/

// MISE description="Run Go tests"

//go:build ignore

package main

import (
	"os"

	"github.com/bitfield/script"
)

func main() {
	_, err := script.Exec("go test ./...").Stdout()
	if err != nil {
		os.Exit(1)
	}
}
