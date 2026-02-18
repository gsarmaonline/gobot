package channels

import "sync"

// MockChannel is a test implementation of Channel that captures sent messages.
type MockChannel struct {
	name     string
	sent     []OutboundMessage
	handler  func(InboundMessage)
	mu       sync.Mutex
	stopCh   chan struct{}
}

func NewMockChannel(name string) *MockChannel {
	return &MockChannel{
		name:   name,
		stopCh: make(chan struct{}),
	}
}

func (c *MockChannel) Name() string    { return c.name }
func (c *MockChannel) Available() bool { return true }

func (c *MockChannel) Send(msg OutboundMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, msg)
	return nil
}

func (c *MockChannel) Listen(handler func(InboundMessage)) error {
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()
	<-c.stopCh
	return nil
}

func (c *MockChannel) Stop() error {
	select {
	case c.stopCh <- struct{}{}:
	default:
	}
	return nil
}

// SentMessages returns all messages sent through this channel
func (c *MockChannel) SentMessages() []OutboundMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]OutboundMessage, len(c.sent))
	copy(result, c.sent)
	return result
}

// SimulateInbound simulates receiving a message on this channel
func (c *MockChannel) SimulateInbound(msg InboundMessage) {
	c.mu.Lock()
	handler := c.handler
	c.mu.Unlock()
	if handler != nil {
		handler(msg)
	}
}
