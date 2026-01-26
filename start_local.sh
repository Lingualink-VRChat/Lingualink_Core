#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

export LINGUALINK_CONFIG_FILE="${LINGUALINK_CONFIG_FILE:-$ROOT_DIR/config/config.yaml}"
export LINGUALINK_CONFIG_DIR="${LINGUALINK_CONFIG_DIR:-$ROOT_DIR/config}"
export LINGUALINK_KEYS_FILE="${LINGUALINK_KEYS_FILE:-$ROOT_DIR/config/api_keys.json}"

go run ./cmd/server

