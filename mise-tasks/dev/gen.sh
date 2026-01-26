#!/usr/bin/env bash
#MISE description="Run go generate"
exec go run ./cmd/ago dev gen "$@"
