package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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

// Orchestrator ties multiple Providers and an Executor together.
type Orchestrator struct {
	providers    map[string]provider.Provider
	executor     executor.Executor
	workDirFn    func(provider.InboundMessage) string
	sessionsFile string

	mu       sync.Mutex
	sessions map[string]*session
}

// New creates a new Orchestrator.
// providers is a slice of Provider instances; each must have a unique Name().
// workDirFn returns the working directory to use for a given inbound message.
// sessionsFile is the path to persist session state across restarts (empty = disabled).
func New(providers []provider.Provider, e executor.Executor, workDirFn func(provider.InboundMessage) string, sessionsFile string) *Orchestrator {
	pm := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		pm[p.Name()] = p
	}
	o := &Orchestrator{
		providers:    pm,
		executor:     e,
		workDirFn:    workDirFn,
		sessionsFile: sessionsFile,
		sessions:     make(map[string]*session),
	}
	o.loadSessions()
	return o
}

// loadSessions restores persisted sessions from disk, dropping any older than 24h.
func (o *Orchestrator) loadSessions() {
	if o.sessionsFile == "" {
		return
	}
	data, err := os.ReadFile(o.sessionsFile)
	if err != nil {
		return // file doesn't exist yet — normal on first run
	}
	var sessions map[string]*session
	if err := json.Unmarshal(data, &sessions); err != nil {
		log.Printf("sessions: failed to parse %s: %v", o.sessionsFile, err)
		return
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for key, s := range sessions {
		if s.LastActive.Before(cutoff) {
			delete(sessions, key)
		}
	}
	o.mu.Lock()
	o.sessions = sessions
	o.mu.Unlock()
	log.Printf("sessions: restored %d session(s) from %s", len(sessions), o.sessionsFile)
}

// saveSessions atomically writes the current session map to disk.
func (o *Orchestrator) saveSessions() {
	if o.sessionsFile == "" {
		return
	}
	o.mu.Lock()
	data, err := json.Marshal(o.sessions)
	o.mu.Unlock()
	if err != nil {
		log.Printf("sessions: marshal error: %v", err)
		return
	}
	tmp := o.sessionsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		log.Printf("sessions: write error: %v", err)
		return
	}
	if err := os.Rename(tmp, o.sessionsFile); err != nil {
		log.Printf("sessions: rename error: %v", err)
	}
}

// Run starts all providers and fans their messages into a single stream,
// blocking until ctx is cancelled.
func (o *Orchestrator) Run(ctx context.Context) error {
	merged := make(chan provider.InboundMessage, 32)
	var wg sync.WaitGroup

	names := make([]string, 0, len(o.providers))
	for name, p := range o.providers {
		names = append(names, name)
		ch, err := p.Messages(ctx)
		if err != nil {
			return fmt.Errorf("start messages for %s: %w", name, err)
		}
		wg.Add(1)
		go func(name string, ch <-chan provider.InboundMessage) {
			defer wg.Done()
			for msg := range ch {
				msg.ProviderName = name
				select {
				case merged <- msg:
				case <-ctx.Done():
					return
				}
			}
		}(name, ch)
	}

	go func() {
		wg.Wait()
		close(merged)
	}()

	log.Printf("Orchestrator running (providers=%s executor=%s)", strings.Join(names, ","), o.executor.Name())

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-merged:
			if !ok {
				return nil
			}
			go o.handle(ctx, msg)
		}
	}
}

func sessionKey(msg provider.InboundMessage) string {
	key := msg.ProviderName + ":" + msg.ChatID
	if msg.ThreadID != "" {
		key += ":" + msg.ThreadID
	}
	return key
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

	p, ok := o.providers[msg.ProviderName]
	if !ok {
		log.Printf("[%s] unknown provider %q; dropping message", key, msg.ProviderName)
		return
	}

	// Route merge actions to dedicated handler.
	if msg.Meta != nil && msg.Meta["action"] == "merge" {
		o.handleMerge(ctx, msg, key, p)
		return
	}

	log.Printf("[%s] received from %s: %q", key, msg.SenderName, msg.Text)

	// Show typing indicator.
	if err := p.SendTyping(ctx, msg.ChatID); err != nil {
		log.Printf("[%s] SendTyping error: %v", key, err)
	}

	workDir := o.workDirFn(msg)

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
		o.sendError(ctx, msg, p, err)
		return
	}

	if p.Streaming() {
		o.handleStreaming(ctx, msg, key, p, chunks, result)
	} else {
		o.handleBatch(ctx, msg, key, p, chunks, result)
	}
}

// handleStreaming fans chunks out to the provider as they arrive (e.g. Telegram).
func (o *Orchestrator) handleStreaming(ctx context.Context, msg provider.InboundMessage, key string, p provider.Provider, chunks <-chan executor.StreamChunk, result <-chan *executor.Result) {
	var textBuf strings.Builder
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		text := strings.TrimSpace(textBuf.String())
		if text == "" {
			return
		}
		textBuf.Reset()
		o.send(ctx, msg, p, text)
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
				o.send(ctx, msg, p, fmt.Sprintf("_Using tool: %s…_", chunk.Content))
			case "error":
				flush()
				o.send(ctx, msg, p, fmt.Sprintf("⚠️ Error: %s", chunk.Content))
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
func (o *Orchestrator) handleBatch(ctx context.Context, msg provider.InboundMessage, key string, p provider.Provider, chunks <-chan executor.StreamChunk, result <-chan *executor.Result) {
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
		o.sendError(ctx, msg, p, res.Err)
		return
	}

	prURL := ""
	if match := prURLRegex.FindString(res.Output); match != "" {
		prURL = match
		log.Printf("[%s] extracted PR URL: %s", key, prURL)
	}

	o.storeSession(key, res, prURL)

	if res.Output != "" {
		o.send(ctx, msg, p, res.Output)
	}
}

// handleMerge squash-merges the PR associated with the session.
func (o *Orchestrator) handleMerge(ctx context.Context, msg provider.InboundMessage, key string, p provider.Provider) {
	log.Printf("[%s] handling merge action", key)

	s := o.getSession(key)
	if s == nil || s.PRUrl == "" {
		log.Printf("[%s] no PR URL found for merge", key)
		o.send(ctx, msg, p, "⚠️ No PR found to merge. Was the issue implemented first?")
		return
	}

	workDir := o.workDirFn(msg)
	cmd := exec.CommandContext(ctx, "gh", "pr", "merge", "--squash", s.PRUrl)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[%s] gh pr merge error: %v\n%s", key, err, out)
		o.send(ctx, msg, p, fmt.Sprintf("⚠️ PR merge failed: %s\n```\n%s\n```", err, strings.TrimSpace(string(out))))
		return
	}

	log.Printf("[%s] merged PR %s", key, s.PRUrl)
	o.send(ctx, msg, p, fmt.Sprintf("✅ PR merged: %s\n```\n%s\n```", s.PRUrl, strings.TrimSpace(string(out))))
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
		o.saveSessions()
	}
	if res.Err != nil {
		log.Printf("executor error: %v", res.Err)
	}
}

func (o *Orchestrator) send(ctx context.Context, inbound provider.InboundMessage, p provider.Provider, text string) {
	if err := p.Send(ctx, provider.OutboundMessage{
		ChatID:   inbound.ChatID,
		ThreadID: inbound.ThreadID,
		Text:     text,
	}); err != nil {
		log.Printf("send error: %v", err)
	}
}

func (o *Orchestrator) sendError(ctx context.Context, inbound provider.InboundMessage, p provider.Provider, err error) {
	o.send(ctx, inbound, p, fmt.Sprintf("⚠️ %s", err.Error()))
}
