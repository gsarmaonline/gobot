package channels

import "sync"

// InternalChannel implements Channel for agent-to-agent communication.
// Uses an in-memory pub/sub (can be replaced with Redis pub/sub in production).
type InternalChannel struct {
	mu       sync.RWMutex
	handler  func(InboundMessage)
	stopCh   chan struct{}
	messages chan InboundMessage
}

func NewInternalChannel() *InternalChannel {
	return &InternalChannel{
		stopCh:   make(chan struct{}),
		messages: make(chan InboundMessage, 100),
	}
}

func (c *InternalChannel) Name() string    { return "internal" }
func (c *InternalChannel) Available() bool { return true }

func (c *InternalChannel) Send(msg OutboundMessage) error {
	c.mu.RLock()
	handler := c.handler
	c.mu.RUnlock()

	if handler != nil {
		handler(InboundMessage{
			Channel:   "internal",
			From:      msg.From,
			To:        msg.To,
			Content:   msg.Content,
			MessageID: msg.InReplyTo,
		})
	}
	return nil
}

func (c *InternalChannel) Listen(handler func(InboundMessage)) error {
	c.mu.Lock()
	c.handler = handler
	c.mu.Unlock()

	<-c.stopCh
	return nil
}

func (c *InternalChannel) Stop() error {
	select {
	case c.stopCh <- struct{}{}:
	default:
	}
	return nil
}
