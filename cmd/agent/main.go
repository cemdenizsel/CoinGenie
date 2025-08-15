package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/tools"
)

type mcpHTTP struct {
	base string
	hc   *http.Client
}

func newMCP(base string) *mcpHTTP {
	c := &http.Client{Timeout: 60 * time.Second}
	_ = post(c, base, map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "lc", "version": "0.1"},
		},
	})
	return &mcpHTTP{base: base, hc: c}
}

func (m *mcpHTTP) call(name string, args map[string]any) (string, error) {
	var out struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if _, err := postJSON(m.hc, m.base, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	}, &out); err != nil {
		return "", err
	}
	if len(out.Result.Content) == 0 {
		return "", nil
	}
	return out.Result.Content[0].Text, nil
}

func (m *mcpHTTP) listTools() ([]map[string]any, error) {
	var out struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if _, err := postJSON(m.hc, m.base, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/list",
	}, &out); err != nil {
		return nil, err
	}
	return out.Result.Tools, nil
}

func post(c *http.Client, url string, body any) error {
	_, err := postJSON(c, url, body, nil)
	return err
}

func postJSON(c *http.Client, url string, body any, out any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if out == nil {
			resp.Body.Close()
		}
	}()
	if out != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

type genericMCPTool struct {
	client *mcpHTTP
	name   string
	desc   string
}

func (t genericMCPTool) Name() string        { return t.name }
func (t genericMCPTool) Description() string { return t.desc }
func (t genericMCPTool) Call(ctx context.Context, input string) (string, error) {
	var a map[string]any
	_ = json.Unmarshal([]byte(input), &a)
	return t.client.call(t.name, a)
}

type xTool struct{ client *mcpHTTP }

func (t xTool) Name() string { return "x_post_reply" }
func (t xTool) Description() string {
	return "Reply under a tweet via X MCP. Input JSON: {\"in_reply_to_tweet_id\":\"...\",\"text\":\"...\"}"
}
func (t xTool) Call(ctx context.Context, input string) (string, error) {
	var a map[string]any
	_ = json.Unmarshal([]byte(input), &a)
	return t.client.call("twitter.post_reply", a)
}

func cgDiscoveredTools(cg *mcpHTTP) ([]tools.Tool, error) {
	raw, err := cg.listTools()
	if err != nil {
		return nil, err
	}
	out := make([]tools.Tool, 0, len(raw))
	for _, t := range raw {
		name, _ := t["name"].(string)
		if name == "" {
			continue
		}
		description, _ := t["description"].(string)
		// include inputSchema (if any) as a compact JSON to guide the LLM
		if schemaVal, ok := t["inputSchema"]; ok && schemaVal != nil {
			if b, err := json.Marshal(schemaVal); err == nil {
				description = fmt.Sprintf("%s\nInput JSON must match schema: %s", description, string(b))
			}
		}
		out = append(out, genericMCPTool{client: cg, name: name, desc: description})
	}
	return out, nil
}

func main() {
	cgURL := os.Getenv("CG_MCP_HTTP")
	xURL := os.Getenv("X_MCP_HTTP")
	if cgURL == "" || xURL == "" {
		fmt.Fprintln(os.Stderr, "Set CG_MCP_HTTP (e.g., http://localhost:8082/mcp) and X_MCP_HTTP (e.g., http://localhost:8081/mcp)")
		os.Exit(1)
	}

	question := flag.String("q", "", "question to ask the agent (fallback: AGENT_INPUT or stdin)")
	replyTo := flag.String("reply-to", "", "tweet id to reply under using x_post_reply (optional)")
	flag.Parse()

	q := strings.TrimSpace(*question)
	if q == "" {
		if v := strings.TrimSpace(os.Getenv("AGENT_INPUT")); v != "" {
			q = v
		} else if fi, _ := os.Stdin.Stat(); fi != nil && (fi.Mode()&os.ModeCharDevice) == 0 {
			b, _ := io.ReadAll(os.Stdin)
			q = strings.TrimSpace(string(b))
		}
	}
	if q == "" {
		fmt.Fprintln(os.Stderr, "Provide a question with -q, AGENT_INPUT, or piped stdin.")
		os.Exit(1)
	}

	cg := newMCP(cgURL)
	x := newMCP(xURL)

	cgTools, err := cgDiscoveredTools(cg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to discover CG tools:", err)
		os.Exit(1)
	}
	toolsList := append(cgTools, xTool{client: x})

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4.1-mini"
	}
	llm, err := openai.New(openai.WithModel(model))
	if err != nil {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is required or configure a provider supported by LangChainGo")
		os.Exit(1)
	}

	peh := agents.NewParserErrorHandler(nil)
	exec, err := agents.Initialize(
		llm,
		toolsList,
		agents.ZeroShotReactDescription,
		agents.WithMaxIterations(8),
		agents.WithReturnIntermediateSteps(),
		agents.WithParserErrorHandler(peh),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	prompt := q
	if strings.TrimSpace(*replyTo) != "" {
		prompt = fmt.Sprintf("%s Answer this question using CoinGecko MCP. Then reply to tweet %s using x_post_reply.", prompt, strings.TrimSpace(*replyTo))
	}

	ctx := context.Background()
	out, err := exec.Call(ctx, map[string]any{"input": prompt})
	if err != nil {
		// Non-fatal: print best effort output or last observation, exit 0
		if steps, ok := out["intermediateSteps"].([]schema.AgentStep); ok && len(steps) > 0 {
			fmt.Println(steps[len(steps)-1].Observation)
			return
		}
		if v, ok := out["output"].(string); ok && v != "" {
			fmt.Println(v)
			return
		}
		fmt.Println(err.Error())
		return
	}
	fmt.Println(out["output"])
}
