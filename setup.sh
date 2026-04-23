#!/usr/bin/env bash
# Optional: install addlicense for a local binary (Make targets use go run, so this is not required for CI or make license).
set -euo pipefail
go install github.com/google/addlicense@v1.2.0
