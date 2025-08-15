package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cg-mentions-bot/internal/handlers"
	"cg-mentions-bot/internal/twitter"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	baseURL := getEnv("X_BASE", "https://api.twitter.com/2")
	bearer := os.Getenv("X_BEARER_TOKEN")
	if bearer == "" {
		fmt.Fprintln(os.Stderr, "X_BEARER_TOKEN is required for xmcp")
		os.Exit(1)
	}

	post := twitter.NewPoster(baseURL, bearer)

	s := server.NewMCPServer(
		"x-poster",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	tool := mcp.Tool{
		Name:        "twitter.post_reply",
		Description: "Post a reply under a tweet using Twitter API v2",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"in_reply_to_tweet_id": map[string]any{"type": "string", "description": "The tweet ID to reply to"},
				"text":                 map[string]any{"type": "string", "description": "The text to post as reply"},
			},
			Required: []string{"in_reply_to_tweet_id", "text"},
		},
	}

	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		inReply, err := request.RequireString("in_reply_to_tweet_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		text, err := request.RequireString("text")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := post(ctx, handlers.ReplyIn{InReplyTo: inReply, Text: text}); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("ok"), nil
	})

	port := getEnv("PORT", "8081")
	httpServer := server.NewStreamableHTTPServer(
		s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)
	log.Printf("x-poster MCP server listening on :%s/mcp", port)
	if err := httpServer.Start(":" + port); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func getEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}
