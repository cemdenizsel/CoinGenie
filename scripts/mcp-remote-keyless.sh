#!/usr/bin/env bash
set -euo pipefail

# Simple stdio bridge to CoinGecko public keyless MCP server via mcp-remote
export NODE_NO_WARNINGS=1
exec /opt/homebrew/bin/npx mcp-remote https://mcp.api.coingecko.com/sse


