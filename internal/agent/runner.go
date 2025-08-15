package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Runner invokes the agent executable with a question and optional reply-to tweet id.
type Runner func(ctx context.Context, question string, replyTo string) (string, error)

// NewRunner constructs a Runner for the given agent command. It inherits the current
// process environment and optionally overrides common agent envs if set:
//   - AGENT_CG_MCP_HTTP, AGENT_X_MCP_HTTP, OPENAI_API_KEY, OPENAI_MODEL
func NewRunner(agentCmd string) Runner {
	return func(ctx context.Context, question string, replyTo string) (string, error) {
		args := []string{"-q", question}
		if replyTo != "" {
			args = append(args, "-reply-to", replyTo)
		}
		cmd := exec.CommandContext(ctx, agentCmd, args...)
		// Inherit env, apply optional overrides
		env := os.Environ()
		if v := os.Getenv("AGENT_CG_MCP_HTTP"); v != "" {
			env = append(env, fmt.Sprintf("CG_MCP_HTTP=%s", v))
		}
		if v := os.Getenv("AGENT_X_MCP_HTTP"); v != "" {
			env = append(env, fmt.Sprintf("X_MCP_HTTP=%s", v))
		}
		if v := os.Getenv("OPENAI_API_KEY"); v != "" {
			env = append(env, fmt.Sprintf("OPENAI_API_KEY=%s", v))
		}
		if v := os.Getenv("OPENAI_MODEL"); v != "" {
			env = append(env, fmt.Sprintf("OPENAI_MODEL=%s", v))
		}
		cmd.Env = env
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		stdout := outBuf.String()
		if err != nil {
			// Return stdout if present, but include error and stderr for visibility
			if stdout != "" {
				return stdout, fmt.Errorf("agent error: %v; stderr: %s", err, errBuf.String())
			}
			return "", fmt.Errorf("agent error: %v; stderr: %s", err, errBuf.String())
		}
		return stdout, nil
	}
}
