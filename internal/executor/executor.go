package executor

import "context"

// StreamChunk is a piece of output emitted during execution.
type StreamChunk struct {
	Type    string // "text", "tool_use", "error"
	Content string
}

// Result is the final outcome of an execution.
type Result struct {
	Output    string
	SessionID string // used with --resume for follow-up messages
	Err       error
}

// Executor runs prompts and streams results back.
type Executor interface {
	Name() string
	// Stream starts a new session. systemPrompt is the agent's effective system prompt; pass "" for none.
	Stream(ctx context.Context, prompt, workDir, systemPrompt string) (<-chan StreamChunk, <-chan *Result, error)
	// Resume continues an existing session (system prompt is preserved from the original session).
	Resume(ctx context.Context, sessionID, prompt, workDir string) (<-chan StreamChunk, <-chan *Result, error)
}
