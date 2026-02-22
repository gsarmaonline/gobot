package provider

import "context"

// InboundMessage is a message received from a provider.
type InboundMessage struct {
	ID         string
	ChatID     string
	ThreadID   string // non-empty for Telegram Forum topic messages
	SenderName string
	Text       string
	Timestamp  int64
	Meta       map[string]string // provider-specific metadata (e.g. "action", "teamKey")
}

// OutboundMessage is a message to be sent via a provider.
type OutboundMessage struct {
	ChatID   string
	ThreadID string
	Text     string
	ReplyTo  string
}

// Provider is the interface for messaging backends.
type Provider interface {
	Name() string
	Messages(ctx context.Context) (<-chan InboundMessage, error)
	Send(ctx context.Context, msg OutboundMessage) error
	SendTyping(ctx context.Context, chatID string) error
	// Streaming returns true if the provider supports streaming responses
	// (send chunks as they arrive), false if it prefers a single final response.
	Streaming() bool
}
