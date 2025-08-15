package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"cg-mentions-bot/internal/cg"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	var q string
	var list bool
	var tool string
	var argsJSON string
	flag.StringVar(&q, "q", "price of btc in usd?", "question to ask CoinGecko MCP")
	flag.BoolVar(&list, "list", false, "list available tools instead of asking a question")
	flag.StringVar(&tool, "tool", "", "call a specific tool (overrides -q)")
	flag.StringVar(&argsJSON, "args", "", "JSON object string for tool arguments (used with -tool)")
	flag.Parse()

	mcpCmd := os.Getenv("MCP_CMD")
	if mcpCmd == "" {
		fmt.Fprintln(os.Stderr, "MCP_CMD env required (path to MCP stdio executable)")
		os.Exit(1)
	}
	mcpTool := getEnv("MCP_TOOL", "coingecko.answer")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if list {
		if err := listTools(ctx, mcpCmd); err != nil {
			fmt.Fprintln(os.Stderr, "ERR:", err)
			os.Exit(1)
		}
		return
	}

	if tool != "" {
		if err := callTool(ctx, mcpCmd, tool, argsJSON); err != nil {
			fmt.Fprintln(os.Stderr, "ERR:", err)
			os.Exit(1)
		}
		return
	}

	ask := cg.NewAsker(mcpCmd, mcpTool)
	ans, err := ask(ctx, q)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERR:", err)
		os.Exit(1)
	}
	fmt.Println(ans)
}

func getEnv(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

func listTools(ctx context.Context, mcpCmd string) error {
	c, err := mcpclient.NewStdioMCPClient(mcpCmd, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{Method: string(mcp.MethodInitialize)},
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo:      mcp.Implementation{Name: "cg-mentions-bot", Version: "0.1.0"},
		},
	}); err != nil {
		return err
	}
	lt, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return err
	}
	for _, t := range lt.Tools {
		fmt.Printf("tool: %s\n", t.Name)
		if len(t.InputSchema.Properties) > 0 {
			fmt.Printf("  props: %v\n", t.InputSchema.Properties)
		}
	}
	return nil
}

func callTool(ctx context.Context, mcpCmd string, tool string, argsJSON string) error {
	c, err := mcpclient.NewStdioMCPClient(mcpCmd, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{
		Request: mcp.Request{Method: string(mcp.MethodInitialize)},
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo:      mcp.Implementation{Name: "cg-mentions-bot", Version: "0.1.0"},
		},
	}); err != nil {
		return err
	}
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fmt.Errorf("invalid -args JSON: %w", err)
		}
	}
	res, err := c.CallTool(ctx, mcp.CallToolRequest{
		Request: mcp.Request{Method: string(mcp.MethodToolsCall)},
		Params:  mcp.CallToolParams{Name: tool, Arguments: args},
	})
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(b))
	return nil
}
