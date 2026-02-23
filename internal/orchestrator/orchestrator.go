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
	SessionID     string
	LastActive    time.Time
	PRUrl         string
	WorkDir       string // stored for CI watcher resumption
	CIStatus      string // "pending" | "passing" | "failing"
	CIRetries     int    // times we've resumed Claude to fix CI
	StuckNotified bool   // prevents duplicate stuck notifications
}

// Options configures the Orchestrator.
type Options struct {
	SessionsFile    string
	CICheckInterval time.Duration // default: 60s
	CIStuckTimeout  time.Duration // default: 30m
	MaxCIRetries    int           // default: 3
	Broadcaster     func(string)  // optional: sends to Telegram admins
}

// Orchestrator ties multiple Providers and an Executor together.
type Orchestrator struct {
	providers   map[string]provider.Provider
	executor    executor.Executor
	workDirFn   func(provider.InboundMessage) string
	opts        Options

	mu       sync.Mutex
	sessions map[string]*session
}

// New creates a new Orchestrator.
// providers is a slice of Provider instances; each must have a unique Name().
// workDirFn returns the working directory to use for a given inbound message.
func New(providers []provider.Provider, e executor.Executor, workDirFn func(provider.InboundMessage) string, opts Options) *Orchestrator {
	if opts.CICheckInterval == 0 {
		opts.CICheckInterval = 60 * time.Second
	}
	if opts.CIStuckTimeout == 0 {
		opts.CIStuckTimeout = 30 * time.Minute
	}
	if opts.MaxCIRetries == 0 {
		opts.MaxCIRetries = 3
	}

	pm := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		pm[p.Name()] = p
	}
	o := &Orchestrator{
		providers: pm,
		executor:  e,
		workDirFn: workDirFn,
		opts:      opts,
		sessions:  make(map[string]*session),
	}
	o.loadSessions()
	return o
}

// loadSessions restores persisted sessions from disk, dropping any older than 24h.
func (o *Orchestrator) loadSessions() {
	if o.opts.SessionsFile == "" {
		return
	}
	data, err := os.ReadFile(o.opts.SessionsFile)
	if err != nil {
		return // file doesn't exist yet — normal on first run
	}
	var sessions map[string]*session
	if err := json.Unmarshal(data, &sessions); err != nil {
		log.Printf("sessions: failed to parse %s: %v", o.opts.SessionsFile, err)
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
	log.Printf("sessions: restored %d session(s) from %s", len(sessions), o.opts.SessionsFile)
}

// saveSessions atomically writes the current session map to disk.
func (o *Orchestrator) saveSessions() {
	if o.opts.SessionsFile == "" {
		return
	}
	o.mu.Lock()
	data, err := json.Marshal(o.sessions)
	o.mu.Unlock()
	if err != nil {
		log.Printf("sessions: marshal error: %v", err)
		return
	}
	tmp := o.opts.SessionsFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		log.Printf("sessions: write error: %v", err)
		return
	}
	if err := os.Rename(tmp, o.opts.SessionsFile); err != nil {
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
		o.handleBatch(ctx, msg, key, p, chunks, result, workDir)
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
		o.storeSession(key, res, "", "")
	}
}

// handleBatch drains all chunks silently, then sends a single final response.
func (o *Orchestrator) handleBatch(ctx context.Context, msg provider.InboundMessage, key string, p provider.Provider, chunks <-chan executor.StreamChunk, result <-chan *executor.Result, workDir string) {
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

	o.storeSession(key, res, prURL, workDir)

	if res.Output != "" {
		o.send(ctx, msg, p, res.Output)
	}

	// Start CI watcher if we have a PR URL.
	if prURL != "" {
		go o.startCIWatcher(ctx, key, prURL, workDir)
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

	// Warn if CI is currently failing — bot may still be working on it.
	if s.CIStatus == "failing" {
		o.send(ctx, msg, p, fmt.Sprintf("⚠️ CI is failing for %s — bot is working on fixes. Retry merge once CI passes.", s.PRUrl))
		return
	}

	workDir := o.workDirFn(msg)
	if err := o.runMerge(ctx, s.PRUrl, workDir); err != nil {
		log.Printf("[%s] gh pr merge error: %v", key, err)
		o.send(ctx, msg, p, fmt.Sprintf("⚠️ PR merge failed: %v", err))
		return
	}

	log.Printf("[%s] merged PR %s", key, s.PRUrl)
	o.send(ctx, msg, p, fmt.Sprintf("✅ PR merged: %s", s.PRUrl))
}

// runMerge executes gh pr merge --squash for the given PR URL.
func (o *Orchestrator) runMerge(ctx context.Context, prURL, workDir string) error {
	cmd := exec.CommandContext(ctx, "gh", "pr", "merge", "--squash", prURL)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// checkCIStatus queries GitHub for the PR's check statuses.
// Returns "passing", "failing", or "pending" with a details string.
func (o *Orchestrator) checkCIStatus(prURL, workDir string) (status, details string, err error) {
	type check struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	}

	cmd := exec.Command("gh", "pr", "checks", prURL, "--json", "name,status,conclusion")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("gh pr checks: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	var checks []check
	if err := json.Unmarshal(out, &checks); err != nil {
		return "", "", fmt.Errorf("parse gh pr checks output: %w", err)
	}

	// No checks configured — treat as passing.
	if len(checks) == 0 {
		return "passing", "", nil
	}

	var failing, pending []string
	for _, c := range checks {
		switch c.Status {
		case "completed":
			switch c.Conclusion {
			case "success", "neutral", "skipped":
				// passing
			default:
				failing = append(failing, c.Name)
			}
		default:
			// in_progress, queued, waiting, etc.
			pending = append(pending, c.Name)
		}
	}

	if len(failing) > 0 {
		return "failing", strings.Join(failing, ", "), nil
	}
	if len(pending) > 0 {
		return "pending", strings.Join(pending, ", "), nil
	}
	return "passing", "", nil
}

// startCIWatcher polls GitHub CI status for prURL and auto-merges on success,
// or resumes Claude to fix failures (up to maxCIRetries times).
func (o *Orchestrator) startCIWatcher(ctx context.Context, key, prURL, workDir string) {
	log.Printf("[%s] CI watcher started for %s", key, prURL)

	checkInterval := o.opts.CICheckInterval
	stuckTimer := time.NewTimer(o.opts.CIStuckTimeout)
	defer stuckTimer.Stop()

	// Initial wait — CI needs time to trigger after PR creation.
	select {
	case <-time.After(checkInterval):
	case <-ctx.Done():
		return
	}

	broadcast := func(msg string) {
		if o.opts.Broadcaster != nil {
			o.opts.Broadcaster(msg)
		}
	}

	markStuck := func(reason string) {
		s := o.getSession(key)
		if s == nil || s.StuckNotified {
			return
		}
		msg := fmt.Sprintf("⚠️ [%s] stuck after %d retries / %s: %s", key, o.opts.MaxCIRetries, reason, prURL)
		log.Printf(msg)
		broadcast(msg)
		o.mu.Lock()
		if s2 := o.sessions[key]; s2 != nil {
			s2.StuckNotified = true
		}
		o.mu.Unlock()
		o.saveSessions()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-stuckTimer.C:
			markStuck("timeout")
			return
		default:
		}

		status, details, err := o.checkCIStatus(prURL, workDir)
		if err != nil {
			log.Printf("[%s] CI status check error: %v", key, err)
			// Treat transient errors as pending and retry.
			status = "pending"
		}

		log.Printf("[%s] CI status: %s", key, status)

		// Update session CI status.
		o.mu.Lock()
		if s := o.sessions[key]; s != nil {
			s.CIStatus = status
		}
		o.mu.Unlock()
		o.saveSessions()

		switch status {
		case "passing":
			if err := o.autoMerge(ctx, key, prURL, workDir); err != nil {
				broadcast(fmt.Sprintf("⚠️ [%s] auto-merge failed: %v — %s", key, err, prURL))
			} else {
				broadcast(fmt.Sprintf("✅ PR auto-merged: %s", prURL))
			}
			return

		case "failing":
			s := o.getSession(key)
			retries := 0
			if s != nil {
				retries = s.CIRetries
			}
			if retries >= o.opts.MaxCIRetries {
				markStuck(fmt.Sprintf("%d retries", retries))
				return
			}
			log.Printf("[%s] CI failing (retry %d/%d): %s", key, retries+1, o.opts.MaxCIRetries, details)
			o.resumeForCI(ctx, key, prURL, workDir, details)
			// Double the check interval to give CI time to re-trigger.
			checkInterval *= 2
		}

		// Wait before next poll.
		select {
		case <-time.After(checkInterval):
		case <-ctx.Done():
			return
		case <-stuckTimer.C:
			markStuck("timeout")
			return
		}
	}
}

// resumeForCI resumes the Claude session with a prompt to fix the failing CI checks.
func (o *Orchestrator) resumeForCI(ctx context.Context, key, prURL, workDir, failedChecks string) {
	s := o.getSession(key)
	if s == nil || s.SessionID == "" {
		log.Printf("[%s] resumeForCI: no session to resume", key)
		return
	}

	// Increment retry counter before resuming.
	o.mu.Lock()
	if s2 := o.sessions[key]; s2 != nil {
		s2.CIRetries++
	}
	o.mu.Unlock()
	o.saveSessions()

	prompt := fmt.Sprintf(
		"The GitHub Actions CI is failing for PR %s.\nFailing checks: %s\n\n"+
			"Please investigate the failures (use `gh run view --log-failed` to read the logs), fix the code, and push the fixes.",
		prURL, failedChecks,
	)

	log.Printf("[%s] resuming Claude to fix CI failures: %s", key, failedChecks)
	chunks, result, err := o.executor.Resume(ctx, s.SessionID, prompt, workDir)
	if err != nil {
		log.Printf("[%s] resumeForCI executor error: %v", key, err)
		return
	}

	// Drain chunks (batch mode — no provider to stream to).
	for chunk := range chunks {
		if chunk.Type == "tool_use" {
			log.Printf("[%s] CI fix tool_use: %s", key, chunk.Content)
		}
	}

	res, ok := <-result
	if !ok || res == nil {
		return
	}
	if res.Err != nil {
		log.Printf("[%s] resumeForCI result error: %v", key, res.Err)
		return
	}

	// Update session ID after resume.
	o.storeSession(key, res, "", workDir)
	log.Printf("[%s] CI fix attempt complete", key)
}

// autoMerge squash-merges the PR and updates session state.
func (o *Orchestrator) autoMerge(ctx context.Context, key, prURL, workDir string) error {
	log.Printf("[%s] auto-merging PR %s", key, prURL)
	return o.runMerge(ctx, prURL, workDir)
}

func (o *Orchestrator) storeSession(key string, res *executor.Result, prURL, workDir string) {
	if res.SessionID != "" || prURL != "" || workDir != "" {
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
		if workDir != "" {
			s.WorkDir = workDir
		} else if existing != nil {
			s.WorkDir = existing.WorkDir
		}
		if existing != nil {
			s.CIStatus = existing.CIStatus
			s.CIRetries = existing.CIRetries
			s.StuckNotified = existing.StuckNotified
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
