#!/usr/bin/env bash
#MISE description="Check generated code is checked-in"
#MISE depends=["dev:fmt", "dev:gen"]
exec go run ./cmd/ago check uncommitted-changes "$@"
