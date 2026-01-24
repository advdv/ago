//usr/local/go/bin/go run "$0" "$@"; exit

//MISE description="Check generated code is checked-in"
//MISE depends=["dev:fmt", "dev:gen"]

//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bitfield/script"
)

func main() {
	if os.Getenv("CI") != "true" {
		return
	}

	status, err := script.Exec("git status --porcelain").String()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: Failed to check git status:", err)
		os.Exit(1)
	}

	if strings.TrimSpace(status) != "" {
		fmt.Fprintln(os.Stderr, "ERROR: Code is not up to date.")
		fmt.Fprintln(os.Stderr, "Run generating tasks locally and commit the changes.")
		fmt.Fprintln(os.Stderr)
		script.Exec("git status --short").Stdout()
		os.Exit(1)
	}
}
