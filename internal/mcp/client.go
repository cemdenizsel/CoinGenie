package mcpclient

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	c, err := mcpclient.NewStdioMCPClient(mcpCmd, os.Environ())
	if err != nil {
		return "", err
	}
	defer c.Close()

	_, err = c.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{Method: string(mcp.MethodInitialize)},
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "cg-mentions-bot",
				Version: "0.1.0",
			},
		},
	})
	if err != nil {
		return "", err
	}

	res, err := c.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
		Params: mcp.CallToolParams{
			Name:      tool,
			Arguments: args,
		},
	})
	if err != nil {
		return "", err
	}
	if res == nil || len(res.Content) == 0 {
		return "", errors.New("empty tool result")
	}

	var parts []string
	for _, item := range res.Content {
		switch v := item.(type) {
		case mcp.TextContent:
			if v.Text != "" {
				parts = append(parts, v.Text)
			}
		default:
			_ = fmt.Sprintf("ignored non-text content: %T", v)
		}
	}
	if len(parts) == 0 {
		return "", errors.New("no text content returned")
	}
	return strings.Join(parts, "\n"), nil
}
