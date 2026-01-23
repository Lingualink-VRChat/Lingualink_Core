#!/usr/bin/env bash
set -euo pipefail

export LINGUALINK_CONFIG_FILE="${LINGUALINK_CONFIG_FILE:-config/config.yaml}"
export LINGUALINK_KEYS_FILE="${LINGUALINK_KEYS_FILE:-config/api_keys.json}"

echo "Starting Lingualink Core locally with Go ${GO_VERSION:-$(go env GOVERSION)}"
go run ./cmd/server
