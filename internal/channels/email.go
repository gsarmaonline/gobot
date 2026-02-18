package channels

import "fmt"

// EmailChannel implements Channel for email communication.
// In production, uses Gmail API (for Google Workspace) or IMAP/SMTP.
type EmailChannel struct {
	agentEmail string
	credentials string
	stopCh     chan struct{}
}

func NewEmailChannel(agentEmail, credentials string) *EmailChannel {
	return &EmailChannel{
		agentEmail:  agentEmail,
		credentials: credentials,
		stopCh:      make(chan struct{}),
	}
}

func (c *EmailChannel) Name() string    { return "email" }
func (c *EmailChannel) Available() bool { return c.agentEmail != "" && c.credentials != "" }

func (c *EmailChannel) Send(msg OutboundMessage) error {
	if !c.Available() {
		return fmt.Errorf("email channel not configured")
	}
	// In production: use Gmail API
	// srv.Users.Messages.Send("me", &gmail.Message{Raw: base64EncodedEmail})
	return nil
}

func (c *EmailChannel) Listen(handler func(InboundMessage)) error {
	if !c.Available() {
		return fmt.Errorf("email channel not configured")
	}
	// In production: poll Gmail API or use push notifications (Pub/Sub)
	// for {
	//   select {
	//   case <-c.stopCh: return nil
	//   case <-time.After(30 * time.Second): checkNewEmails()
	//   }
	// }
	<-c.stopCh
	return nil
}

func (c *EmailChannel) Stop() error {
	select {
	case c.stopCh <- struct{}{}:
	default:
	}
	return nil
}
