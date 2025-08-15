# cg-mentions-bot

A minimal Go 1.23 service that:
- Receives mentions from n8n at `POST /mentions`.
- Assumes each mention is a CoinGecko question.
- Calls a CoinGecko MCP server (over stdio via mcp-go) to get an answer.
- Posts a reply under the original tweet.

References:
- CoinGecko MCP Server (Beta): https://docs.coingecko.com/reference/mcp-server
- MCP-Go Getting Started: https://mcp-go.dev/getting-started

## Overview
- HTTP server exposes:
  - `GET /healthz` → `{ "ok": true }`
  - `POST /mentions` → accepts either a single JSON payload or an array of payloads (see examples below)
- For each mention, we normalize the tweet text (strip `@handles` and URLs), assume it is a CoinGecko question, then ask the MCP server.
- The MCP client uses stdio to spawn whatever command is in `MCP_CMD` and calls the tool named in `MCP_TOOL`.

## Repo Layout
- `cmd/bot` → service entrypoint
- `cmd/askcg` → small CLI to list tools and call tools directly for testing
- `internal/httpserver` → chi router/server
- `internal/handlers` → `POST /mentions` handler
- `internal/types` → request payload types
- `internal/mcp` → minimal mcp-go stdio client wrappers
- `internal/cg` → builds the natural-language prompt to MCP
- `internal/twitter` → posts replies to Twitter API v2
- `scripts/mcp-remote-keyless.sh` → stdio bridge to CoinGecko public keyless server
- `scripts/mcp-local-demo.sh` → launches local CoinGecko MCP server (requires API key)

## Requirements
- Go 1.23+
- Node + npx (required by CoinGecko MCP server or `mcp-remote` bridge)
- A CoinGecko API key if you run the local MCP server (Demo)

## Environment Variables
- `PORT` (default `8080`)
- `WEBHOOK_SECRET` (required; validated against `X-Webhook-Secret` header)
- `MCP_CMD` (required; command to launch an MCP server that speaks stdio)
- `MCP_TOOL` (default `coingecko.answer`; tool to call)
- `X_BEARER_TOKEN` (required; user-context OAuth2 bearer with tweet.write)
- `X_BASE` (default `https://api.twitter.com/2`)

## Pick an MCP server

Option A) Public keyless (SSE bridged via stdio):
```bash
# The script assumes npx is at /opt/homebrew/bin/npx (macOS ARM). Adjust if needed.
./scripts/mcp-remote-keyless.sh
```
Set for the app:
```bash
export MCP_CMD="./scripts/mcp-remote-keyless.sh"
export MCP_TOOL="get_simple_price"   # keyless often exposes API-shaped tools, not NL "answer"
```

Option B) Local server (Demo/Pro key):
```bash
# Demo key example
export COINGECKO_DEMO_API_KEY="YOUR_DEMO_KEY"
export COINGECKO_ENVIRONMENT=demo
export MCP_CMD="./scripts/mcp-local-demo.sh"
export MCP_TOOL="coingecko.answer"   # if the local server exposes the NL answer tool
```

Note: If the server does not expose `coingecko.answer` and only provides API-shaped tools (e.g., `get_simple_price`), set `MCP_TOOL` accordingly and pass structured args when testing via `askcg`.

## Quick testing with askcg

Build the CLI:
```bash
go build -o askcg ./cmd/askcg
```

List available tools:
```bash
MCP_CMD="./scripts/mcp-local-demo.sh" ./askcg -list
# Or keyless:
MCP_CMD="./scripts/mcp-remote-keyless.sh" ./askcg -list
```

Call a specific tool with JSON args (example: get_simple_price):
```bash
MCP_CMD="./scripts/mcp-local-demo.sh" ./askcg -tool get_simple_price -args '{"ids":"bitcoin","vs_currencies":"usd"}'
```

If you have a natural-language answer tool:
```bash
MCP_CMD="./scripts/mcp-local-demo.sh" MCP_TOOL="coingecko.answer" ./askcg -q "price of btc in usd?"
```

## Run the service locally
```bash
export WEBHOOK_SECRET=devsecret
export MCP_CMD="./scripts/mcp-local-demo.sh"      # or ./scripts/mcp-remote-keyless.sh
export MCP_TOOL="coingecko.answer"                # or get_simple_price if answer is not available
export X_BEARER_TOKEN="YOUR_USER_CONTEXT_BEARER_WITH_TWEET_WRITE"

go run ./cmd/bot
```

Health check:
```bash
curl -s http://localhost:8080/healthz
```

Send a mock mention (single payload object):
```bash
curl -s -H "X-Webhook-Secret: devsecret" -H "Content-Type: application/json" \
  -d '{"count":1,"mentions":[{"tweet_id":"1","text":"btc price in usd?","author_id":"x","author_username":"u","conversation_id":"1","created_at":"2025-01-01T00:00:00Z"}]}' \
  http://localhost:8080/mentions
```

Send an array of payloads (handler flattens):
```bash
curl -s -H "X-Webhook-Secret: devsecret" -H "Content-Type: application/json" \
  -d '[{"count":2,"mentions":[{"tweet_id":"1","text":"btc price in usd?","author_id":"x","author_username":"u","conversation_id":"1","created_at":"2025-01-01T00:00:00Z"},{"tweet_id":"2","text":"eth market cap?","author_id":"y","author_username":"v","conversation_id":"2","created_at":"2025-01-01T00:01:00Z"}]}]' \
  http://localhost:8080/mentions
```

Response format:
```json
{"received": N, "processed": N, "results": [{"tweet_id":"...","posted":true|false,"error":"...optional"}]}
```

## Endpoint contract
- Header: `X-Webhook-Secret: <token>` must match `WEBHOOK_SECRET` or 401 is returned.
- Body (single object):
```json
{
  "count": 1,
  "mentions": [
    {"tweet_id":"1","text":"btc price in usd?","author_id":"x","author_username":"u","conversation_id":"1","created_at":"2025-01-01T00:00:00Z"}
  ],
  "meta": { }
}
```
- Body (array): `[{...}, {...}]` – the handler accepts and flattens all `mentions`.
- The handler removes `@handles` and URLs before asking the MCP tool.

## Twitter posting
- Uses `X_BEARER_TOKEN` (user-context OAuth2) and `X_BASE` (default `https://api.twitter.com/2`).
- POST `/2/tweets` with body `{ "text": "...", "reply": { "in_reply_to_tweet_id": "..." } }`.
- If posting fails, the error string is returned in the `results` array and processing continues.

## X MCP (post replies) — HTTP
```bash
# OAuth2 bearer (user-context)
PORT=8081 X_BEARER_TOKEN="YOUR_ACCESS_TOKEN" ./xmcp

# Or OAuth1 (app perms: Read and Write; OAuth 1.0a enabled)
export X_AUTH_MODE=oauth1
export X_CONSUMER_KEY=...
export X_CONSUMER_SECRET=...
export X_ACCESS_TOKEN=...
export X_ACCESS_SECRET=...
PORT=8081 ./xmcp

# Test
curl -s http://localhost:8081/mcp -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}'
curl -s http://localhost:8081/mcp -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
curl -s http://localhost:8081/mcp -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"twitter.post_reply","arguments":{"in_reply_to_tweet_id":"<tweet_id>","text":"hello from XMCP"}}}'
```

## CoinGecko MCP HTTP proxy (cgproxy)
Expose CoinGecko MCP server as an HTTP MCP endpoint so agents can call it directly.
```bash
# Public keyless (SSE remote bridged to HTTP)
export CG_MCP_CMD="npx"
export CG_MCP_ARGS="mcp-remote https://mcp.api.coingecko.com/sse"
PORT=8082 ./cgproxy

# Or local server (Demo/Pro)
export CG_MCP_CMD="npx"
export CG_MCP_ARGS="-y @coingecko/coingecko-mcp"
export COINGECKO_DEMO_API_KEY=YOUR_DEMO_KEY
export COINGECKO_ENVIRONMENT=demo
PORT=8082 ./cgproxy

# Test
curl -s http://localhost:8082/mcp -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}'
curl -s http://localhost:8082/mcp -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
curl -s http://localhost:8082/mcp -X POST -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_simple_price","arguments":{"ids":"bitcoin","vs_currencies":"usd"}}}'
```

## Troubleshooting
- `request failed` from MCP:
  - Use the local MCP server with a Demo/Pro key.
  - Verify tools via `askcg -list` and set `MCP_TOOL` to one that exists.
- `listen tcp :8080: bind: address already in use`:
  - Kill the existing process on 8080 (e.g., `lsof -ti tcp:8080 | xargs kill -9`).
- `command not found: npx`:
  - Install Node and ensure `npx` is on PATH; or edit the script to use your absolute `npx` path.

