package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gsarma/gobot/internal/executor"
	"github.com/gsarma/gobot/internal/provider"
)

const (
	flushInterval = 2 * time.Second
	flushMaxChars = 500
)

var prURLRegex = regexp.MustCompile(`https://github\.com/\S+/pull/\d+`)

type session struct {
	SessionID  string
	LastActive time.Time
	PRUrl      string
}

// Orchestrator ties a Provider and an Executor together.
type Orchestrator struct {
	provider  provider.Provider
	executor  executor.Executor
	workDirFn func(provider.InboundMessage) string

	mu       sync.Mutex
	sessions map[string]*session
}

// New creates a new Orchestrator.
// workDirFn returns the working directory to use for a given inbound message.
func New(p provider.Provider, e executor.Executor, workDirFn func(provider.InboundMessage) string) *Orchestrator {
	return &Orchestrator{
		provider:  p,
		executor:  e,
		workDirFn: workDirFn,
		sessions:  make(map[string]*session),
	}
}

// Run starts the orchestrator and blocks until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	msgs, err := o.provider.Messages(ctx)
	if err != nil {
		return fmt.Errorf("start messages: %w", err)
	}

	log.Printf("Orchestrator running (provider=%s executor=%s)", o.provider.Name(), o.executor.Name())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return nil
			}
			go o.handle(ctx, msg)
		}
	}
}

func sessionKey(msg provider.InboundMessage) string {
	if msg.ThreadID != "" {
		return msg.ChatID + ":" + msg.ThreadID
	}
	return msg.ChatID
}

func (o *Orchestrator) getSession(key string) *session {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.sessions[key]
}

func (o *Orchestrator) setSession(key string, s *session) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sessions[key] = s
}

func (o *Orchestrator) handle(ctx context.Context, msg provider.InboundMessage) {
	key := sessionKey(msg)

	// Route merge actions to dedicated handler.
	if msg.Meta != nil && msg.Meta["action"] == "merge" {
		o.handleMerge(ctx, msg, key)
		return
	}

	log.Printf("[%s] received from %s: %q", key, msg.SenderName, msg.Text)

	// Show typing indicator.
	if err := o.provider.SendTyping(ctx, msg.ChatID); err != nil {
		log.Printf("[%s] SendTyping error: %v", key, err)
	}

	workDir := o.workDirFn(msg)

	// Determine whether to resume an existing session.
	var chunks <-chan executor.StreamChunk
	var result <-chan *executor.Result
	var err error

	if s := o.getSession(key); s != nil {
		log.Printf("[%s] resuming session %s", key, s.SessionID)
		chunks, result, err = o.executor.Resume(ctx, s.SessionID, msg.Text, workDir)
	} else {
		log.Printf("[%s] starting new session", key)
		chunks, result, err = o.executor.Stream(ctx, msg.Text, workDir)
	}

	if err != nil {
		o.sendError(ctx, msg, err)
		return
	}

	streaming := o.provider.Streaming()

	if streaming {
		o.handleStreaming(ctx, msg, key, chunks, result)
	} else {
		o.handleBatch(ctx, msg, key, chunks, result, workDir)
	}
}

// handleStreaming fans chunks out to the provider as they arrive (e.g. Telegram).
func (o *Orchestrator) handleStreaming(ctx context.Context, msg provider.InboundMessage, key string, chunks <-chan executor.StreamChunk, result <-chan *executor.Result) {
	var textBuf strings.Builder
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		text := strings.TrimSpace(textBuf.String())
		if text == "" {
			return
		}
		textBuf.Reset()
		o.send(ctx, msg, text)
	}

	done := false
	for !done {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				done = true
				continue
			}
			switch chunk.Type {
			case "text":
				textBuf.WriteString(chunk.Content)
				if textBuf.Len() >= flushMaxChars {
					flush()
				}
			case "tool_use":
				flush()
				o.send(ctx, msg, fmt.Sprintf("_Using tool: %s…_", chunk.Content))
			case "error":
				flush()
				o.send(ctx, msg, fmt.Sprintf("⚠️ Error: %s", chunk.Content))
			}

		case <-ticker.C:
			flush()

		case <-ctx.Done():
			return
		}
	}

	flush()

	if res, ok := <-result; ok && res != nil {
		o.storeSession(key, res, "")
	}
}

// handleBatch drains all chunks silently, then sends a single final response.
func (o *Orchestrator) handleBatch(ctx context.Context, msg provider.InboundMessage, key string, chunks <-chan executor.StreamChunk, result <-chan *executor.Result, workDir string) {
	// Drain chunks — log tool_use events for observability.
	for chunk := range chunks {
		if chunk.Type == "tool_use" {
			log.Printf("[%s] tool_use: %s", key, chunk.Content)
		}
	}

	res, ok := <-result
	if !ok || res == nil {
		return
	}

	if res.Err != nil {
		log.Printf("[%s] executor error: %v", key, res.Err)
		o.sendError(ctx, msg, res.Err)
		return
	}

	// Extract PR URL from output.
	prURL := ""
	if match := prURLRegex.FindString(res.Output); match != "" {
		prURL = match
		log.Printf("[%s] extracted PR URL: %s", key, prURL)
	}

	o.storeSession(key, res, prURL)

	if res.Output != "" {
		o.send(ctx, msg, res.Output)
	}
}

// handleMerge squash-merges the PR associated with the session.
func (o *Orchestrator) handleMerge(ctx context.Context, msg provider.InboundMessage, key string) {
	log.Printf("[%s] handling merge action", key)

	s := o.getSession(key)
	if s == nil || s.PRUrl == "" {
		log.Printf("[%s] no PR URL found for merge", key)
		o.send(ctx, msg, "⚠️ No PR found to merge. Was the issue implemented first?")
		return
	}

	workDir := o.workDirFn(msg)
	cmd := exec.CommandContext(ctx, "gh", "pr", "merge", "--squash", s.PRUrl)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[%s] gh pr merge error: %v\n%s", key, err, out)
		o.send(ctx, msg, fmt.Sprintf("⚠️ PR merge failed: %s\n```\n%s\n```", err, strings.TrimSpace(string(out))))
		return
	}

	log.Printf("[%s] merged PR %s", key, s.PRUrl)
	o.send(ctx, msg, fmt.Sprintf("✅ PR merged: %s\n```\n%s\n```", s.PRUrl, strings.TrimSpace(string(out))))
}

func (o *Orchestrator) storeSession(key string, res *executor.Result, prURL string) {
	if res.SessionID != "" || prURL != "" {
		existing := o.getSession(key)
		s := &session{
			SessionID:  res.SessionID,
			LastActive: time.Now(),
		}
		if prURL != "" {
			s.PRUrl = prURL
		} else if existing != nil {
			s.PRUrl = existing.PRUrl
		}
		o.setSession(key, s)
		log.Printf("[%s] stored session %s prURL=%s", key, res.SessionID, s.PRUrl)
	}
	if res.Err != nil {
		log.Printf("[%s] executor error: %v", key, res.Err)
	}
}

func (o *Orchestrator) send(ctx context.Context, inbound provider.InboundMessage, text string) {
	if err := o.provider.Send(ctx, provider.OutboundMessage{
		ChatID:   inbound.ChatID,
		ThreadID: inbound.ThreadID,
		Text:     text,
	}); err != nil {
		log.Printf("send error: %v", err)
	}
}

func (o *Orchestrator) sendError(ctx context.Context, inbound provider.InboundMessage, err error) {
	o.send(ctx, inbound, fmt.Sprintf("⚠️ %s", err.Error()))
}
