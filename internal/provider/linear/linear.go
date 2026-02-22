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
	"strings"

	"github.com/gsarma/gobot/internal/config"
	"github.com/gsarma/gobot/internal/provider"
)

// Linear implements provider.Provider via an HTTP webhook server.
type Linear struct {
	cfg    *config.Config
	client *Client
	msgs   chan provider.InboundMessage
	server *http.Server
}

// New creates a new Linear webhook provider.
func New(cfg *config.Config) *Linear {
	return &Linear{
		cfg:    cfg,
		client: NewClient(cfg.LinearAPIKey),
		msgs:   make(chan provider.InboundMessage, 16),
	}
}

// Name returns the provider name.
func (l *Linear) Name() string { return "linear" }

// Streaming returns false — Linear comments are posted once at the end.
func (l *Linear) Streaming() bool { return false }

// SendTyping is a no-op for Linear.
func (l *Linear) SendTyping(ctx context.Context, chatID string) error { return nil }

// Send posts a comment on the Linear issue identified by out.ChatID.
func (l *Linear) Send(ctx context.Context, out provider.OutboundMessage) error {
	return l.client.PostComment(out.ChatID, out.Text)
}

// Messages starts the HTTP webhook server and returns a channel of inbound events.
func (l *Linear) Messages(ctx context.Context) (<-chan provider.InboundMessage, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", l.handleWebhook)

	addr := fmt.Sprintf(":%d", l.cfg.LinearWebhookPort)
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

	return l.msgs, nil
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

	// Verify HMAC-SHA256 signature.
	if !l.verifySignature(rawBody, r.Header.Get("Linear-Signature")) {
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

	var action string
	switch stateName {
	case l.cfg.LinearTriggerState:
		action = "execute"
	case l.cfg.LinearDoneState:
		action = "merge"
	default:
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fetch full issue details from Linear API.
	issue, err := l.client.FetchIssue(data.ID)
	if err != nil {
		log.Printf("linear: fetch issue %s: %v", data.ID, err)
		http.Error(w, "fetch issue failed", http.StatusInternalServerError)
		return
	}

	msg := provider.InboundMessage{
		ID:     issue.ID,
		ChatID: issue.ID,
		Text:   "",
		Meta: map[string]string{
			"action":  action,
			"teamKey": issue.TeamKey,
		},
	}

	if action == "execute" {
		msg.Text = buildPrompt(issue)
	}

	log.Printf("linear: issue %s action=%s team=%s", issue.Identifier, action, issue.TeamKey)

	select {
	case l.msgs <- msg:
	default:
		log.Printf("linear: message channel full, dropping event for %s", issue.ID)
	}

	w.WriteHeader(http.StatusOK)
}

func (l *Linear) verifySignature(body []byte, sig string) bool {
	mac := hmac.New(sha256.New, []byte(l.cfg.LinearWebhookSecret))
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
