#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
GO_TOOL="${TUNEHARBOR_GO:-go}"
"$GO_TOOL" run ./cmd/tuneharbor "$@"
