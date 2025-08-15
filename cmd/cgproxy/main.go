package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	cmd := os.Getenv("CG_MCP_CMD")
	if cmd == "" {
		log.Fatal("CG_MCP_CMD is required (e.g., 'npx' or path to coingecko mcp binary)")
	}
	argsEnv := os.Getenv("CG_MCP_ARGS")
	args := []string{}
	if strings.TrimSpace(argsEnv) != "" {
		args = splitArgs(argsEnv)
	}

	toolMap, err := fetchTools(cmd, args)
	if err != nil {
		log.Fatalf("failed to fetch tools from upstream: %v", err)
	}

	s := server.NewMCPServer("cg-proxy", "0.1.0", server.WithToolCapabilities(true), server.WithLogging())

	// Register each upstream tool name and forward calls
	for _, tool := range toolMap {
		t := tool // capture
		s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			res, fwdErr := forwardCall(cmd, args, req.Params.Name, req.GetArguments())
			if fwdErr != nil {
				return mcp.NewToolResultError(fwdErr.Error()), nil
			}
			return res, nil
		})
	}

	port := getEnv("PORT", "8082")
	http := server.NewStreamableHTTPServer(s, server.WithEndpointPath("/mcp"), server.WithStateLess(true))
	log.Printf("cg-proxy MCP server listening on :%s/mcp (forwarding to: %s %s)", port, cmd, strings.Join(args, " "))
	if err := http.Start(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func fetchTools(cmd string, args []string) (map[string]mcp.Tool, error) {
	c, err := mcpclient.NewStdioMCPClient(cmd, os.Environ(), args...)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{Request: mcp.Request{Method: string(mcp.MethodInitialize)}, Params: mcp.InitializeParams{ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION, Capabilities: mcp.ClientCapabilities{}, ClientInfo: mcp.Implementation{Name: "cg-proxy", Version: "0.1.0"}}}); err != nil {
		return nil, err
	}
	lt, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	tools := make(map[string]mcp.Tool, len(lt.Tools))
	for _, t := range lt.Tools {
		tools[t.Name] = t
	}
	return tools, nil
}

func forwardCall(cmd string, args []string, name string, arguments map[string]any) (*mcp.CallToolResult, error) {
	c, err := mcpclient.NewStdioMCPClient(cmd, os.Environ(), args...)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{Request: mcp.Request{Method: string(mcp.MethodInitialize)}, Params: mcp.InitializeParams{ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION, Capabilities: mcp.ClientCapabilities{}, ClientInfo: mcp.Implementation{Name: "cg-proxy", Version: "0.1.0"}}}); err != nil {
		return nil, err
	}
	return c.CallTool(ctx, mcp.CallToolRequest{Request: mcp.Request{Method: string(mcp.MethodToolsCall)}, Params: mcp.CallToolParams{Name: name, Arguments: arguments}})
}

func splitArgs(s string) []string {
	// naive split on spaces; trim extra spaces
	parts := strings.Fields(s)
	return parts
}

func getEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}
