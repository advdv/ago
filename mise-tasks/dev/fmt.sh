#!/usr/bin/env bash
#MISE description="Format Go code using golangci-lint"
exec go run ./cmd/ago dev fmt "$@"
