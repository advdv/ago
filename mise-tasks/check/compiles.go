//usr/local/go/bin/go run "$0" "$@"; exit

//MISE description="Check that all packages compile"

//go:build ignore

package main

import (
	"os"

	"github.com/bitfield/script"
)

func main() {
	_, err := script.Exec("go build ./...").Stdout()
	if err != nil {
		os.Exit(1)
	}
}
