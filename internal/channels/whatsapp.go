package channels

import "fmt"

// WhatsAppChannel implements Channel for WhatsApp messaging via Twilio.
type WhatsAppChannel struct {
	phoneNumber string
	accountSID  string
	authToken   string
	stopCh      chan struct{}
}

func NewWhatsAppChannel(phoneNumber, accountSID, authToken string) *WhatsAppChannel {
	return &WhatsAppChannel{
		phoneNumber: phoneNumber,
		accountSID:  accountSID,
		authToken:   authToken,
		stopCh:      make(chan struct{}),
	}
}

func (c *WhatsAppChannel) Name() string    { return "whatsapp" }
func (c *WhatsAppChannel) Available() bool {
	return c.phoneNumber != "" && c.accountSID != "" && c.authToken != ""
}

func (c *WhatsAppChannel) Send(msg OutboundMessage) error {
	if !c.Available() {
		return fmt.Errorf("whatsapp channel not configured")
	}
	// In production: use Twilio API
	// params := &api.CreateMessageParams{}
	// params.SetFrom("whatsapp:" + c.phoneNumber)
	// params.SetTo("whatsapp:" + msg.To)
	// params.SetBody(msg.Content)
	return nil
}

func (c *WhatsAppChannel) Listen(handler func(InboundMessage)) error {
	if !c.Available() {
		return fmt.Errorf("whatsapp channel not configured")
	}
	// In production: Twilio sends webhooks for incoming WhatsApp messages
	// The webhook handler would parse the request and call handler(msg)
	<-c.stopCh
	return nil
}

func (c *WhatsAppChannel) Stop() error {
	select {
	case c.stopCh <- struct{}{}:
	default:
	}
	return nil
}
