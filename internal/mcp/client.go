package mcpclient

import (
	"context"
	"errors"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Ask uses the official MCP stdio client to initialize and call a tool, then
// extracts plain text from the tool response content.
func Ask(ctx context.Context, mcpCmd string, tool string, prompt string) (string, error) {
	return Call(ctx, mcpCmd, tool, map[string]interface{}{"question": prompt})
}

// Call initializes the MCP client and calls a tool with arbitrary arguments,
// returning concatenated text content.
func Call(ctx context.Context, mcpCmd string, tool string, args map[string]interface{}) (string, error) {
	c, err := mcpclient.NewStdioMCPClient(mcpCmd)
	if err != nil {
		return "", err
	}
	defer c.Close()

	_, err = c.Initialize(ctx, mcp.ClientCapabilities{}, mcp.Implementation{
		Name:    "cg-mentions-bot",
		Version: "0.1.0",
	}, "2024-11-05")
	if err != nil {
		return "", err
	}

	res, err := c.CallTool(ctx, tool, args)
	if err != nil {
		return "", err
	}
	if res == nil || len(res.Content) == 0 {
		return "", errors.New("empty tool result")
	}

	var parts []string
	for _, item := range res.Content {
		if tc, ok := item.(mcp.TextContent); ok {
			if tc.Text != "" {
				parts = append(parts, tc.Text)
			}
			continue
		}
		if m, ok := item.(map[string]interface{}); ok {
			if m["type"] == "text" {
				if s, _ := m["text"].(string); s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	if len(parts) == 0 {
		return "", errors.New("no text content returned")
	}
	return strings.Join(parts, "\n"), nil
}
