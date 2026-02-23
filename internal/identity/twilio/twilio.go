package twilio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gsarma/gobot/internal/registry"
)

const baseURL = "https://api.twilio.com/2010-04-01"

// Client wraps the Twilio REST API using HTTP Basic Auth.
type Client struct {
	reg        *registry.Registry
	httpClient *http.Client
}

// New creates a new Twilio client backed by the registry for credentials.
func New(reg *registry.Registry) *Client {
	return &Client{
		reg:        reg,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// SMS represents a Twilio SMS message.
type SMS struct {
	SID       string
	From      string
	To        string
	Body      string
	Direction string
	SentAt    time.Time
}

type twilioMessageList struct {
	Messages []twilioMessage `json:"messages"`
}

type twilioMessage struct {
	SID         string `json:"sid"`
	From        string `json:"from"`
	To          string `json:"to"`
	Body        string `json:"body"`
	Direction   string `json:"direction"`
	DateCreated string `json:"date_created"`
}

// ListMessages returns the most recent inbound SMS messages.
func (c *Client) ListMessages(ctx context.Context, pageSize int) ([]SMS, error) {
	data := c.reg.Get()
	if data.Twilio == nil {
		return nil, fmt.Errorf("no twilio config in registry")
	}
	t := data.Twilio

	endpoint := fmt.Sprintf("%s/Accounts/%s/Messages.json?PageSize=%d&To=%s",
		baseURL, t.AccountSID, pageSize, url.QueryEscape(t.PhoneNumber))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("twilio list request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("twilio list: HTTP %d", resp.StatusCode)
	}

	var result twilioMessageList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("twilio decode: %w", err)
	}

	messages := make([]SMS, 0, len(result.Messages))
	for _, m := range result.Messages {
		sentAt, _ := time.Parse(time.RFC1123Z, m.DateCreated)
		messages = append(messages, SMS{
			SID:       m.SID,
			From:      m.From,
			To:        m.To,
			Body:      m.Body,
			Direction: m.Direction,
			SentAt:    sentAt,
		})
	}
	return messages, nil
}

// SendSMS sends an SMS from the configured Twilio phone number.
func (c *Client) SendSMS(ctx context.Context, to, body string) error {
	data := c.reg.Get()
	if data.Twilio == nil {
		return fmt.Errorf("no twilio config in registry")
	}
	t := data.Twilio

	endpoint := fmt.Sprintf("%s/Accounts/%s/Messages.json", baseURL, t.AccountSID)

	form := url.Values{}
	form.Set("From", t.PhoneNumber)
	form.Set("To", to)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("twilio send request: %w", err)
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twilio send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("twilio send: HTTP %d", resp.StatusCode)
	}
	return nil
}
