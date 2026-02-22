package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/gsarma/gobot/internal/config"
	"github.com/gsarma/gobot/internal/executor"
)

// Claude implements executor.Executor using the Claude Code CLI.
type Claude struct {
	cfg *config.Config
}

// New creates a new Claude executor.
func New(cfg *config.Config) *Claude {
	return &Claude{cfg: cfg}
}

// Name returns the executor name.
func (c *Claude) Name() string { return "claude" }

// Stream starts a new Claude Code session for the given prompt.
func (c *Claude) Stream(ctx context.Context, prompt, workDir string) (<-chan executor.StreamChunk, <-chan *executor.Result, error) {
	args := c.baseArgs()
	args = append(args, "-p", prompt)
	return c.stream(ctx, args, workDir)
}

// Resume continues an existing Claude Code session.
func (c *Claude) Resume(ctx context.Context, sessionID, prompt, workDir string) (<-chan executor.StreamChunk, <-chan *executor.Result, error) {
	args := c.baseArgs()
	args = append(args, "-p", prompt, "--resume", sessionID)
	return c.stream(ctx, args, workDir)
}

func (c *Claude) baseArgs() []string {
	budgetStr := strconv.FormatFloat(c.cfg.ClaudeMaxBudgetUSD, 'f', 2, 64)
	return []string{
		"--output-format", "stream-json",
		"--verbose",
		"--model", c.cfg.ClaudeModel,
		"--allowedTools", c.cfg.ClaudeAllowedTools,
		"--permission-mode", "bypassPermissions",
		"--max-budget-usd", budgetStr,
	}
}

func (c *Claude) stream(ctx context.Context, args []string, workDir string) (<-chan executor.StreamChunk, <-chan *executor.Result, error) {
	cmd := exec.CommandContext(ctx, c.cfg.ClaudePath, args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ() // CRITICAL: inherit ANTHROPIC_API_KEY, HOME, etc.

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start claude: %w", err)
	}

	chunks := make(chan executor.StreamChunk, 32)
	result := make(chan *executor.Result, 1)

	go func() {
		defer close(chunks)
		defer close(result)

		var finalResult executor.Result

		// Drain stderr in background so it doesn't block.
		go io.Copy(io.Discard, stderr)

		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1 MB line buffer

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			parseStreamLine(line, chunks, &finalResult)
		}

		if err := cmd.Wait(); err != nil {
			if finalResult.Err == nil {
				finalResult.Err = err
			}
		}

		result <- &finalResult
	}()

	return chunks, result, nil
}

// --- stream-json parsing ---

// Top-level event shapes emitted by `claude --output-format stream-json`.
type streamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	Message json.RawMessage `json:"message"`
	Result  string          `json:"result"`
	IsError bool            `json:"is_error"`
	// session_id lives at the top level on "result" events
	SessionID string `json:"session_id"`
}

type assistantMessage struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Name  string `json:"name"` // for tool_use blocks
	Input any    `json:"input"`
}

func parseStreamLine(line string, chunks chan<- executor.StreamChunk, result *executor.Result) {
	var ev streamEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		// Not valid JSON — skip silently
		return
	}

	switch ev.Type {
	case "assistant":
		var msg assistantMessage
		if err := json.Unmarshal(ev.Message, &msg); err != nil {
			return
		}
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					chunks <- executor.StreamChunk{Type: "text", Content: block.Text}
					result.Output += block.Text
				}
			case "tool_use":
				chunks <- executor.StreamChunk{Type: "tool_use", Content: block.Name}
			}
		}

	case "result":
		result.SessionID = ev.SessionID
		if ev.IsError || ev.Subtype == "error_during_execution" {
			result.Err = fmt.Errorf("claude error: %s", ev.Result)
			chunks <- executor.StreamChunk{Type: "error", Content: ev.Result}
		} else {
			// "result" on success may contain a final text summary.
			if ev.Result != "" && result.Output == "" {
				result.Output = ev.Result
			}
		}
	}
}
