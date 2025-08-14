#!/usr/bin/env bash
set -euo pipefail

: "${COINGECKO_DEMO_API_KEY:?COINGECKO_DEMO_API_KEY required}"
export COINGECKO_ENVIRONMENT=demo

exec npx -y @coingecko/coingecko-mcp


