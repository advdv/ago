#!/usr/bin/env bash
#MISE description="Initialize secrets from 1Password"
set -euo pipefail

op inject -i .env.tpl -o .env
