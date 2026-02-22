package orchestrator

import (
	"context"
	"fmt"
	"log"
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

type session struct {
	SessionID  string
	LastActive time.Time
}

// Orchestrator ties a Provider and an Executor together.
type Orchestrator struct {
	provider provider.Provider
	executor executor.Executor
	workDir  string

	mu       sync.Mutex
	sessions map[string]*session
}

// New creates a new Orchestrator.
func New(p provider.Provider, e executor.Executor, workDir string) *Orchestrator {
	return &Orchestrator{
		provider: p,
		executor: e,
		workDir:  workDir,
		sessions: make(map[string]*session),
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
	log.Printf("[%s] received from %s: %q", key, msg.SenderName, msg.Text)

	// Show typing indicator.
	if err := o.provider.SendTyping(ctx, msg.ChatID); err != nil {
		log.Printf("[%s] SendTyping error: %v", key, err)
	}

	// Determine whether to resume an existing session.
	var chunks <-chan executor.StreamChunk
	var result <-chan *executor.Result
	var err error

	if s := o.getSession(key); s != nil {
		log.Printf("[%s] resuming session %s", key, s.SessionID)
		chunks, result, err = o.executor.Resume(ctx, s.SessionID, msg.Text, o.workDir)
	} else {
		log.Printf("[%s] starting new session", key)
		chunks, result, err = o.executor.Stream(ctx, msg.Text, o.workDir)
	}

	if err != nil {
		o.sendError(ctx, msg, err)
		return
	}

	// Fan out chunks to Telegram.
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
				// Channel closed — wait for result.
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
				// Flush buffered text first, then send tool status.
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

	// Final flush.
	flush()

	// Store the session ID for follow-up messages.
	if res, ok := <-result; ok && res != nil {
		if res.SessionID != "" {
			o.setSession(key, &session{
				SessionID:  res.SessionID,
				LastActive: time.Now(),
			})
			log.Printf("[%s] stored session %s", key, res.SessionID)
		}
		if res.Err != nil {
			log.Printf("[%s] executor error: %v", key, res.Err)
		}
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
