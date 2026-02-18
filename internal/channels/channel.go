package channels

// InboundMessage represents a message received on any channel
type InboundMessage struct {
	Channel   string            `json:"channel"`   // "email", "whatsapp", "phone", "internal"
	From      string            `json:"from"`      // sender identifier
	To        string            `json:"to"`        // recipient (agent) identifier
	Subject   string            `json:"subject,omitempty"`
	Content   string            `json:"content"`
	MessageID string            `json:"messageId"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// OutboundMessage represents a message to be sent on any channel
type OutboundMessage struct {
	Channel   string            `json:"channel"`
	From      string            `json:"from"` // agent identifier
	To        string            `json:"to"`   // recipient
	Subject   string            `json:"subject,omitempty"`
	Content   string            `json:"content"`
	InReplyTo string            `json:"inReplyTo,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Channel is the abstraction for a communication channel.
// Implementations: email (Gmail), WhatsApp (Twilio), phone (Twilio), internal (Redis pub/sub).
type Channel interface {
	// Name returns the channel identifier (e.g., "email", "whatsapp")
	Name() string

	// Available returns true if the channel is configured and operational
	Available() bool

	// Send delivers an outbound message
	Send(msg OutboundMessage) error

	// Listen starts receiving inbound messages. Sends them to the handler.
	// Blocks until Stop is called.
	Listen(handler func(InboundMessage)) error

	// Stop stops the listener
	Stop() error
}

// Router manages multiple channels and routes messages to the right one
type Router struct {
	channels map[string]Channel
	handler  func(InboundMessage) // callback for all inbound messages
}

func NewRouter() *Router {
	return &Router{
		channels: make(map[string]Channel),
	}
}

// Register adds a channel to the router
func (r *Router) Register(ch Channel) {
	r.channels[ch.Name()] = ch
}

// SetHandler sets the callback for all inbound messages
func (r *Router) SetHandler(handler func(InboundMessage)) {
	r.handler = handler
}

// Send routes an outbound message to the appropriate channel
func (r *Router) Send(msg OutboundMessage) error {
	ch, ok := r.channels[msg.Channel]
	if !ok {
		return nil // silently skip unavailable channels
	}
	if !ch.Available() {
		return nil
	}
	return ch.Send(msg)
}

// StartAll starts listening on all registered channels
func (r *Router) StartAll() {
	for _, ch := range r.channels {
		if ch.Available() {
			go ch.Listen(r.handler)
		}
	}
}

// StopAll stops all channels
func (r *Router) StopAll() {
	for _, ch := range r.channels {
		ch.Stop()
	}
}

// AvailableChannels returns names of all configured channels
func (r *Router) AvailableChannels() []string {
	var names []string
	for name, ch := range r.channels {
		if ch.Available() {
			names = append(names, name)
		}
	}
	return names
}
