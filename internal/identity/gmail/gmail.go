package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/gsarma/gobot/internal/registry"
)

var otpRegexp = regexp.MustCompile(`\b\d{4,8}\b`)

// Client wraps the Gmail v1 API.
type Client struct {
	reg *registry.Registry
}

// New creates a new Gmail client backed by the registry for credentials.
func New(reg *registry.Registry) *Client {
	return &Client{reg: reg}
}

// Message is a simplified Gmail message.
type Message struct {
	ID      string
	From    string
	Subject string
	Date    time.Time
	Snippet string
	Body    string
}

// ListMessages queries Gmail and returns up to maxResults messages matching query.
func (c *Client) ListMessages(ctx context.Context, query string, maxResults int64) ([]Message, error) {
	svc, err := c.service(ctx)
	if err != nil {
		return nil, err
	}

	req := svc.Users.Messages.List("me").Q(query)
	if maxResults > 0 {
		req = req.MaxResults(maxResults)
	}

	resp, err := req.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("gmail list: %w", err)
	}

	msgs := make([]Message, 0, len(resp.Messages))
	for _, m := range resp.Messages {
		msg, err := c.GetMessage(ctx, m.Id)
		if err != nil {
			continue
		}
		msgs = append(msgs, *msg)
	}
	return msgs, nil
}

// GetMessage fetches a full Gmail message by ID.
func (c *Client) GetMessage(ctx context.Context, id string) (*Message, error) {
	svc, err := c.service(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := svc.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("gmail get %s: %w", id, err)
	}

	msg := &Message{
		ID:      raw.Id,
		Snippet: raw.Snippet,
	}

	for _, h := range raw.Payload.Headers {
		switch h.Name {
		case "From":
			msg.From = h.Value
		case "Subject":
			msg.Subject = h.Value
		case "Date":
			msg.Date, _ = time.Parse(time.RFC1123Z, h.Value)
		}
	}

	msg.Body = extractBody(raw.Payload)
	return msg, nil
}

// SendEmail sends an email from the configured Gmail account.
func (c *Client) SendEmail(ctx context.Context, to, subject, body string) error {
	svc, err := c.service(ctx)
	if err != nil {
		return err
	}

	data := c.reg.Get()
	from := ""
	if data.Google != nil {
		from = data.Google.Email
	}

	raw := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body)
	encoded := base64.URLEncoding.EncodeToString([]byte(raw))

	_, err = svc.Users.Messages.Send("me", &gmailv1.Message{Raw: encoded}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("gmail send: %w", err)
	}
	return nil
}

// FindOTP searches recent emails for a numeric OTP code.
// It looks at messages from senderPattern (substring match) with subjectPattern (substring match)
// received after since, and extracts the first 4-8 digit code.
func (c *Client) FindOTP(ctx context.Context, senderPattern, subjectPattern string, since time.Time) (string, error) {
	query := fmt.Sprintf("after:%d", since.Unix())
	if senderPattern != "" {
		query += " from:" + senderPattern
	}
	if subjectPattern != "" {
		query += " subject:" + subjectPattern
	}

	msgs, err := c.ListMessages(ctx, query, 10)
	if err != nil {
		return "", fmt.Errorf("find otp: %w", err)
	}

	for _, msg := range msgs {
		text := msg.Body + " " + msg.Snippet
		if m := otpRegexp.FindString(text); m != "" {
			return m, nil
		}
	}
	return "", fmt.Errorf("no OTP found in recent messages")
}

// service builds an authenticated Gmail service, refreshing the token if needed.
func (c *Client) service(ctx context.Context) (*gmailv1.Service, error) {
	ts, err := c.tokenSource(ctx)
	if err != nil {
		return nil, err
	}
	svc, err := gmailv1.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("gmail service: %w", err)
	}
	return svc, nil
}

func (c *Client) tokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	data := c.reg.Get()
	if data.Google == nil {
		return nil, fmt.Errorf("no google config in registry")
	}
	g := data.Google

	oauthCfg := &oauth2.Config{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			gmailv1.GmailSendScope,
			gmailv1.GmailReadonlyScope,
		},
	}

	expiry, _ := time.Parse(time.RFC3339, g.TokenExpiry)
	tok := &oauth2.Token{
		RefreshToken: g.RefreshToken,
		AccessToken:  g.AccessToken,
		Expiry:       expiry,
	}

	base := oauthCfg.TokenSource(ctx, tok)
	return &saveOnRefreshTokenSource{base: base, reg: c.reg}, nil
}

// saveOnRefreshTokenSource wraps an oauth2.TokenSource and persists the new
// access token + expiry to projects.json whenever a refresh occurs.
type saveOnRefreshTokenSource struct {
	base oauth2.TokenSource
	reg  *registry.Registry
	last *oauth2.Token
}

func (s *saveOnRefreshTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	// Persist only when we got a new token.
	if s.last == nil || tok.AccessToken != s.last.AccessToken {
		s.last = tok
		_ = s.reg.Save(func(d *registry.Data) {
			if d.Google == nil {
				return
			}
			d.Google.AccessToken = tok.AccessToken
			d.Google.TokenExpiry = tok.Expiry.UTC().Format(time.RFC3339)
		})
	}
	return tok, nil
}

// extractBody recursively extracts plain-text body from a MIME payload.
func extractBody(part *gmailv1.MessagePart) string {
	if part == nil {
		return ""
	}
	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		b, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	for _, p := range part.Parts {
		if text := extractBody(p); text != "" {
			return text
		}
	}
	return ""
}
