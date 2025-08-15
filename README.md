# CoinGecko Mentions Bot

> ⚡️ What this is (at a glance)
>
> - 🧭 Purpose: When someone mentions `@NexArb_` on X (Twitter) with a crypto question, we automatically answer using CoinGecko data and reply under the same tweet.
> - 🧩 Pieces:
>   - 🤖 Bot (`/mentions`): receives JSON mentions from n8n
>   - 🧠 Agent: uses LangChainGo to choose the right CoinGecko MCP tool(s)
>   - 🐍 CoinGecko MCP (via SSE proxy): provides data tools (e.g., prices)
>   - 🐦 X-post MCP: posts the reply under the original tweet
> - 🔗 Flow:
>   1) n8n finds mentions of `@NexArb_`
>   2) n8n POSTs them to the bot (`/mentions`)
>   3) Bot normalizes text and delegates each mention to the agent
>   4) Agent calls CoinGecko MCP tools via HTTP proxy and composes an answer
>   5) Agent calls X-post MCP to reply under the same `tweet_id`
>
A minimal Go 1.23 service that:
- Receives mentions from n8n at `POST /mentions`.
- Assumes each mention is a CoinGecko question.
- Recommended: delegates each mention to a LangChainGo agent that auto-discovers CoinGecko MCP tools (HTTP) and posts the reply via an X-post MCP.

References:
- CoinGecko MCP Server (Beta): https://docs.coingecko.com/reference/mcp-server
- MCP-Go Getting Started: https://mcp-go.dev/getting-started

## Overview
- HTTP server exposes:
  - `GET /healthz` → `{ "ok": true }`
  - `POST /mentions` → accepts either a single JSON payload or an array of payloads
- For each mention, the service normalizes the tweet text (strip `@handles` and URLs) and either:
  - Calls the agent (recommended) which chooses appropriate CoinGecko tool(s) and posts a reply via X MCP; or
  - Uses the legacy MCP stdio + Twitter HTTP flow.

## Repo Layout
- `cmd/bot` → service entrypoint
- `cmd/agent` → LangChainGo agent (auto-discovers CG tools and can call `x_post_reply`)
- `cmd/xmcp` → MCP server exposing `twitter.post_reply` over HTTP (port 8081)
- `cmd/cgproxy` → MCP HTTP proxy for CoinGecko via `npx mcp-remote https://mcp.api.coingecko.com/sse` (port 8082)
- `cmd/askcg` → small CLI to list tools and call tools directly for testing
- `internal/httpserver` → chi router/server
- `internal/handlers` → `POST /mentions` handler
- `internal/types` → request payload types
- `internal/agent` → small runner to spawn the agent from the bot
- (legacy) `internal/mcp`, `internal/cg`, `internal/twitter` → kept for compatibility

## Requirements
- Go 1.23+
- Node + npx (for `mcp-remote` bridge)

## Environment Variables
- Agent mode (recommended):
  - `AGENT_CMD` (path to built agent binary; when set, bot delegates per mention)
  - `AGENT_CG_MCP_HTTP` (e.g., `http://localhost:8082/mcp`)
  - `AGENT_X_MCP_HTTP` (e.g., `http://localhost:8081/mcp`)
  - `OPENAI_API_KEY`, `OPENAI_MODEL` (e.g., `gpt-4.1-mini`)
- Legacy mode (without agent):
  - `MCP_CMD` (stdio command; not recommended)
  - `MCP_TOOL` (tool name, e.g., `get_simple_price`)
  - `X_BEARER_TOKEN` (user-context OAuth2 bearer with `tweet.write`)
- Common:
  - `PORT` (default `8080`)
  - `X_BASE` (default `https://api.twitter.com/2`)

## n8n integration (mentions for @NexArb_)
- n8n periodically searches for mentions of the `@NexArb_` account (e.g., via Twitter API or an n8n Twitter node/HTTP node).
- The workflow currently runs every 6 hours (configurable in n8n).
- When a new batch of mentions is found, n8n POSTs the mentions to this service at:
  - `POST http://<ec2-ip-or-host>:8080/mentions`
  - Headers: `Content-Type: application/json` (no auth header required unless you set `WEBHOOK_SECRET`).
  - Body: either a single object or an array matching the examples in this README (each mention includes `tweet_id` and `text`).
- The bot normalizes each mention’s text (removes handles/URLs), then delegates to the agent which:
  - Auto-discovers CoinGecko MCP tools via the HTTP proxy (`cgproxy` on 8082) and selects the right tool based on the question.
  - Posts the answer under the same tweet using the X-post MCP (`xmcp` on 8081) via the `twitter.post_reply` tool with `in_reply_to_tweet_id = <tweet_id>`.
- The `/mentions` response returns a summary with `posted`/`error` per mention so you can track outcomes in n8n.

## Quick start (agent mode)
Start the two MCP HTTP services in separate terminals, then the bot.

1) X-post MCP (posts replies to Twitter)
```bash
# Terminal A
go build -o xmcp ./cmd/xmcp
export X_AUTH_MODE=oauth1
export X_CONSUMER_KEY=...
export X_CONSUMER_SECRET=...
export X_ACCESS_TOKEN=...
export X_ACCESS_SECRET=...
PORT=8081 ./xmcp
```

2) CoinGecko MCP HTTP proxy (via SSE only)
```bash
# Terminal B
go build -o cgproxy ./cmd/cgproxy
export CG_MCP_CMD="npx"
export CG_MCP_ARGS="mcp-remote https://mcp.api.coingecko.com/sse"
PORT=8082 ./cgproxy
```

3) Bot (delegates to agent per mention)
```bash
# Terminal C
go build -o agent ./cmd/agent
export AGENT_CMD="$(pwd)/agent"
export AGENT_CG_MCP_HTTP="http://localhost:8082/mcp"
export AGENT_X_MCP_HTTP="http://localhost:8081/mcp"
export OPENAI_API_KEY=...
export OPENAI_MODEL=gpt-4.1-mini
PORT=8080 go run ./cmd/bot
```

Send a mock mention (single object):
```bash
curl -s -H "Content-Type: application/json" \
  -d '{"count":1,"mentions":[{"tweet_id":"1957000000000000001","text":"btc price in usd?","author_id":"x","author_username":"u","conversation_id":"1957000000000000001","created_at":"2025-01-01T00:00:00Z"}]}' \
  http://localhost:8080/mentions | jq
```

Send an array of payloads:
```bash
curl -s -H "Content-Type: application/json" \
  -d '[{"count":2,"mentions":[{"tweet_id":"1957000000000000002","text":"eth market cap?","author_id":"y","author_username":"v","conversation_id":"1957000000000000002","created_at":"2025-01-01T00:01:00Z"},{"tweet_id":"1957000000000000003","text":"Compare solana and cardano market cap","author_id":"z","author_username":"w","conversation_id":"1957000000000000003","created_at":"2025-01-01:00:02:00Z"}]}]' \
  http://localhost:8080/mentions | jq
```

Response format:
```json
{"received": N, "processed": N, "results": [{"tweet_id":"...","posted":true|false,"error":"...optional"}]}
```

## Quick testing with askcg (optional)
Build the CLI:
```bash
go build -o askcg ./cmd/askcg
```
List available tools (via SSE bridge):
```bash
MCP_CMD="./scripts/mcp-remote-keyless.sh" ./askcg -list
```
Call a specific tool with JSON args:
```bash
MCP_CMD="./scripts/mcp-remote-keyless.sh" ./askcg -tool get_simple_price -args '{"ids":"bitcoin","vs_currencies":"usd"}'
```

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
export CG_MCP_CMD="npx"
export CG_MCP_ARGS="mcp-remote https://mcp.api.coingecko.com/sse"
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
- `request failed` from MCP: verify cgproxy is running and reachable at `AGENT_CG_MCP_HTTP`.
- `listen tcp :8080: bind: address already in use`: kill the existing process on 8080.
- `command not found: npx`: install Node and ensure `npx` is on PATH.

