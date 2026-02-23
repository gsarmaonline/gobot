package linear

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gsarma/gobot/internal/provider"
	"github.com/gsarma/gobot/internal/registry"
)

type pendingEntry struct {
	Issue     *Issue            `json:"issue"`
	Project   string            `json:"project"`
	BlockedBy map[string]string `json:"blocked_by"` // issueID -> identifier (for logging)
	QueuedAt  time.Time         `json:"queued_at"`
}

// Linear implements provider.Provider via an HTTP webhook server.
type Linear struct {
	reg         *registry.Registry
	msgs        chan provider.InboundMessage
	server      *http.Server
	pendingFile string
	pending     map[string]*pendingEntry
	pendingMu   sync.Mutex
}

// New creates a new Linear webhook provider.
func New(reg *registry.Registry, pendingFile string) *Linear {
	l := &Linear{
		reg:         reg,
		msgs:        make(chan provider.InboundMessage, 16),
		pendingFile: pendingFile,
		pending:     make(map[string]*pendingEntry),
	}
	l.loadPending()
	return l
}

// Name returns the provider name.
func (l *Linear) Name() string { return "linear" }

// Streaming returns false — Linear comments are posted once at the end.
func (l *Linear) Streaming() bool { return false }

// SendTyping is a no-op for Linear.
func (l *Linear) SendTyping(ctx context.Context, chatID string) error { return nil }

// Send posts a comment on the Linear issue identified by out.ChatID.
func (l *Linear) Send(ctx context.Context, out provider.OutboundMessage) error {
	data := l.reg.Get().Linear
	if data == nil {
		return fmt.Errorf("linear: no config in registry")
	}
	return NewClient(data.APIKey).PostComment(out.ChatID, out.Text)
}

// Messages starts the HTTP webhook server and returns a channel of inbound events.
func (l *Linear) Messages(ctx context.Context) (<-chan provider.InboundMessage, error) {
	data := l.reg.Get().Linear
	if data == nil {
		return nil, fmt.Errorf("linear: no config in registry")
	}

	port := data.WebhookPort
	if port == 0 {
		port = 8080
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", l.handleWebhook)

	addr := fmt.Sprintf(":%d", port)
	l.server = &http.Server{Addr: addr, Handler: mux}

	go func() {
		log.Printf("Linear webhook server listening on %s", addr)
		if err := l.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("linear webhook server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := l.server.Close(); err != nil {
			log.Printf("linear webhook server close: %v", err)
		}
		close(l.msgs)
	}()

	// Periodic re-check: handles blockers that moved to Done while gobot was down.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.recheckPending()
			}
		}
	}()

	return l.msgs, nil
}

// recheckPending re-fetches each blocker's state and unblocks entries if all blockers are done.
func (l *Linear) recheckPending() {
	cfg := l.reg.Get().Linear
	if cfg == nil {
		return
	}
	doneState := cfg.DoneState
	if doneState == "" {
		doneState = "Done"
	}

	l.pendingMu.Lock()
	// Snapshot the pending map so we can release the lock during API calls.
	snapshot := make(map[string]*pendingEntry, len(l.pending))
	for id, entry := range l.pending {
		snapshot[id] = entry
	}
	l.pendingMu.Unlock()

	if len(snapshot) == 0 {
		return
	}

	client := NewClient(cfg.APIKey)
	changed := false

	for issueID, entry := range snapshot {
		for blockerID := range entry.BlockedBy {
			blocker, err := client.FetchIssue(blockerID)
			if err != nil {
				log.Printf("linear: recheck fetch blocker %s: %v", blockerID, err)
				continue
			}
			if blocker.StateName == doneState {
				l.pendingMu.Lock()
				if e, ok := l.pending[issueID]; ok {
					delete(e.BlockedBy, blockerID)
				}
				l.pendingMu.Unlock()
				changed = true
			}
		}

		// Re-read under lock to check if now unblocked.
		l.pendingMu.Lock()
		e, ok := l.pending[issueID]
		if ok && len(e.BlockedBy) == 0 {
			log.Printf("linear: pending issue %s unblocked by recheck, emitting execute", e.Issue.Identifier)
			msg := provider.InboundMessage{
				ID:     e.Issue.ID,
				ChatID: e.Issue.ID,
				Text:   buildPrompt(e.Issue),
				Meta: map[string]string{
					"action":  "execute",
					"project": e.Project,
				},
			}
			select {
			case l.msgs <- msg:
			default:
				log.Printf("linear: message channel full, dropping recheck unblock for %s", e.Issue.ID)
			}
			delete(l.pending, issueID)
			changed = true
		}
		l.pendingMu.Unlock()
	}

	if changed {
		l.savePending()
	}
}

// loadPending reads the pending file into l.pending, dropping entries older than 7 days.
func (l *Linear) loadPending() {
	if l.pendingFile == "" {
		return
	}
	data, err := os.ReadFile(l.pendingFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("linear: load pending %s: %v", l.pendingFile, err)
		}
		return
	}
	var loaded map[string]*pendingEntry
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("linear: parse pending %s: %v", l.pendingFile, err)
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	l.pendingMu.Lock()
	defer l.pendingMu.Unlock()
	for id, entry := range loaded {
		if entry.QueuedAt.After(cutoff) {
			l.pending[id] = entry
		}
	}
	log.Printf("linear: loaded %d pending entries from %s", len(l.pending), l.pendingFile)
}

// savePending atomically writes l.pending to the pending file.
func (l *Linear) savePending() {
	if l.pendingFile == "" {
		return
	}
	l.pendingMu.Lock()
	data, err := json.MarshalIndent(l.pending, "", "  ")
	l.pendingMu.Unlock()
	if err != nil {
		log.Printf("linear: marshal pending: %v", err)
		return
	}
	tmp := l.pendingFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		log.Printf("linear: write pending tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, l.pendingFile); err != nil {
		log.Printf("linear: rename pending: %v", err)
	}
}

// tryUnblockPending removes completedIssueID from all pending entries' BlockedBy maps
// and emits execute messages for any entries that become fully unblocked.
func (l *Linear) tryUnblockPending(completedIssueID string, doneState string) {
	var toEmit []provider.InboundMessage

	l.pendingMu.Lock()
	changed := false
	for issueID, entry := range l.pending {
		if _, blocked := entry.BlockedBy[completedIssueID]; !blocked {
			continue
		}
		delete(entry.BlockedBy, completedIssueID)
		changed = true

		if len(entry.BlockedBy) == 0 {
			log.Printf("linear: pending issue %s unblocked, emitting execute", entry.Issue.Identifier)
			toEmit = append(toEmit, provider.InboundMessage{
				ID:     entry.Issue.ID,
				ChatID: entry.Issue.ID,
				Text:   buildPrompt(entry.Issue),
				Meta: map[string]string{
					"action":  "execute",
					"project": entry.Project,
				},
			})
			delete(l.pending, issueID)
		}
	}
	l.pendingMu.Unlock()

	for _, msg := range toEmit {
		select {
		case l.msgs <- msg:
		default:
			log.Printf("linear: message channel full, dropping unblock for %s", msg.ID)
		}
	}

	if changed {
		l.savePending()
	}
}

// --- webhook payload types ---

type webhookPayload struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
	Type   string          `json:"type"`
}

type issueData struct {
	ID    string `json:"id"`
	State struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"state"`
	UpdatedFrom struct {
		StateID string `json:"stateId"`
	} `json:"updatedFrom"`
}

func (l *Linear) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	// Read current config snapshot.
	cfg := l.reg.Get().Linear
	if cfg == nil {
		http.Error(w, "linear not configured", http.StatusInternalServerError)
		return
	}

	// Verify HMAC-SHA256 signature.
	if !verifySignature(rawBody, r.Header.Get("Linear-Signature"), cfg.WebhookSecret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// Only handle issue update events.
	if payload.Action != "update" || payload.Type != "Issue" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var data issueData
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		http.Error(w, "invalid issue data", http.StatusBadRequest)
		return
	}

	// Determine which state transition occurred.
	stateName := data.State.Name
	stateChanged := data.UpdatedFrom.StateID != "" && data.UpdatedFrom.StateID != data.State.ID

	if !stateChanged {
		w.WriteHeader(http.StatusOK)
		return
	}

	triggerState := cfg.TriggerState
	if triggerState == "" {
		triggerState = "In Progress"
	}
	doneState := cfg.DoneState
	if doneState == "" {
		doneState = "Done"
	}

	var action string
	switch stateName {
	case triggerState:
		action = "execute"
	case doneState:
		action = "merge"
	default:
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fetch full issue details from Linear API.
	issue, err := NewClient(cfg.APIKey).FetchIssue(data.ID)
	if err != nil {
		log.Printf("linear: fetch issue %s: %v", data.ID, err)
		http.Error(w, "fetch issue failed", http.StatusInternalServerError)
		return
	}

	// Resolve project via team binding.
	project, ok := l.reg.ProjectForTeam(issue.TeamKey)
	if !ok {
		log.Printf("linear: no project binding for team %q; dropping event for %s", issue.TeamKey, issue.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	if action == "execute" {
		// Check for active blockers.
		var activeBlockers []BlockingIssue
		for _, b := range issue.BlockedBy {
			if b.StateName != doneState {
				activeBlockers = append(activeBlockers, b)
			}
		}

		if len(activeBlockers) > 0 {
			// Build blocked_by map for persistence.
			blockedByMap := make(map[string]string, len(activeBlockers))
			identifiers := make([]string, 0, len(activeBlockers))
			for _, b := range activeBlockers {
				blockedByMap[b.ID] = b.Identifier
				identifiers = append(identifiers, b.Identifier)
			}

			l.pendingMu.Lock()
			l.pending[issue.ID] = &pendingEntry{
				Issue:     issue,
				Project:   project,
				BlockedBy: blockedByMap,
				QueuedAt:  time.Now(),
			}
			l.pendingMu.Unlock()
			l.savePending()

			comment := fmt.Sprintf("Waiting on: %s before starting", strings.Join(identifiers, ", "))
			log.Printf("linear: issue %s deferred — %s", issue.Identifier, comment)
			if err := NewClient(cfg.APIKey).PostComment(issue.ID, comment); err != nil {
				log.Printf("linear: post pending comment for %s: %v", issue.Identifier, err)
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		// No active blockers — emit execute immediately.
		msg := provider.InboundMessage{
			ID:     issue.ID,
			ChatID: issue.ID,
			Text:   buildPrompt(issue),
			Meta: map[string]string{
				"action":  "execute",
				"project": project,
			},
		}
		log.Printf("linear: issue %s action=execute team=%s project=%s", issue.Identifier, issue.TeamKey, project)
		select {
		case l.msgs <- msg:
		default:
			log.Printf("linear: message channel full, dropping event for %s", issue.ID)
		}
	} else {
		// action == "merge"
		msg := provider.InboundMessage{
			ID:     issue.ID,
			ChatID: issue.ID,
			Text:   "",
			Meta: map[string]string{
				"action":  "merge",
				"project": project,
			},
		}
		log.Printf("linear: issue %s action=merge team=%s project=%s", issue.Identifier, issue.TeamKey, project)
		select {
		case l.msgs <- msg:
		default:
			log.Printf("linear: message channel full, dropping event for %s", issue.ID)
		}

		// Unblock any pending issues that were waiting on this one.
		l.tryUnblockPending(data.ID, doneState)
	}

	w.WriteHeader(http.StatusOK)
}

func verifySignature(body []byte, sig, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

func buildPrompt(issue *Issue) string {
	identifierLower := strings.ToLower(issue.Identifier)

	desc := issue.Description
	if desc == "" {
		desc = "(no description provided)"
	}

	return fmt.Sprintf(`You are implementing a software task from Linear.

Issue: %s: %s
URL: %s

Description:
%s

Instructions:
1. You are working in the git repository at your current working directory
2. Create a new branch: git checkout -b linear/%s
3. Implement the changes described
4. Write or update tests as needed
5. Commit: git commit -m "%s: %s"
6. Push: git push -u origin linear/%s
7. Create PR: gh pr create --title "%s: %s" --body "Closes %s"
8. Print the full PR URL in your final response

Make reasonable decisions and proceed without asking for clarification.`,
		issue.Identifier, issue.Title,
		issue.URL,
		desc,
		identifierLower,
		issue.Identifier, issue.Title,
		identifierLower,
		issue.Identifier, issue.Title, issue.URL,
	)
}
