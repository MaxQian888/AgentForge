#!/usr/bin/env bash
# Thin compatibility wrapper so CI and existing docs can keep calling the
# historical shell entrypoint while the real implementation stays cross-platform.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
node "$SCRIPT_DIR/build-backend.js" "$@"
