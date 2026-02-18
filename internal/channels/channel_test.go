package channels

import (
	"testing"
	"time"
)

func TestMockChannelSendAndCapture(t *testing.T) {
	ch := NewMockChannel("test")

	msg := OutboundMessage{
		Channel: "test",
		From:    "agent-1",
		To:      "user@example.com",
		Content: "Hello from agent!",
	}

	if err := ch.Send(msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := ch.SentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].Content != "Hello from agent!" {
		t.Errorf("expected 'Hello from agent!', got '%s'", sent[0].Content)
	}
}

func TestMockChannelSimulateInbound(t *testing.T) {
	ch := NewMockChannel("test")

	received := make(chan InboundMessage, 1)
	go ch.Listen(func(msg InboundMessage) {
		received <- msg
	})

	// Wait for listener to register
	time.Sleep(10 * time.Millisecond)

	ch.SimulateInbound(InboundMessage{
		Channel: "test",
		From:    "user@example.com",
		Content: "Hello agent!",
	})

	select {
	case msg := <-received:
		if msg.Content != "Hello agent!" {
			t.Errorf("expected 'Hello agent!', got '%s'", msg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for inbound message")
	}

	ch.Stop()
}

func TestInternalChannelRouting(t *testing.T) {
	ch := NewInternalChannel()

	received := make(chan InboundMessage, 1)
	go ch.Listen(func(msg InboundMessage) {
		received <- msg
	})

	time.Sleep(10 * time.Millisecond)

	// Agent-to-agent message
	ch.Send(OutboundMessage{
		Channel: "internal",
		From:    "agent-1",
		To:      "agent-2",
		Content: "Please handle this customer",
	})

	select {
	case msg := <-received:
		if msg.From != "agent-1" {
			t.Errorf("expected from 'agent-1', got '%s'", msg.From)
		}
		if msg.Content != "Please handle this customer" {
			t.Errorf("unexpected content: %s", msg.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for internal message")
	}

	ch.Stop()
}

func TestRouterSend(t *testing.T) {
	router := NewRouter()
	emailCh := NewMockChannel("email")
	whatsappCh := NewMockChannel("whatsapp")
	router.Register(emailCh)
	router.Register(whatsappCh)

	// Send via email
	router.Send(OutboundMessage{
		Channel: "email",
		Content: "Email message",
	})

	// Send via whatsapp
	router.Send(OutboundMessage{
		Channel: "whatsapp",
		Content: "WhatsApp message",
	})

	// Send via unknown channel (should not error)
	err := router.Send(OutboundMessage{
		Channel: "sms",
		Content: "Should be silently ignored",
	})
	if err != nil {
		t.Errorf("expected no error for unknown channel, got: %v", err)
	}

	if len(emailCh.SentMessages()) != 1 {
		t.Errorf("expected 1 email message, got %d", len(emailCh.SentMessages()))
	}
	if len(whatsappCh.SentMessages()) != 1 {
		t.Errorf("expected 1 whatsapp message, got %d", len(whatsappCh.SentMessages()))
	}
}

func TestRouterAvailableChannels(t *testing.T) {
	router := NewRouter()
	router.Register(NewMockChannel("email"))
	router.Register(NewMockChannel("whatsapp"))
	router.Register(NewInternalChannel())

	available := router.AvailableChannels()
	if len(available) != 3 {
		t.Errorf("expected 3 available channels, got %d", len(available))
	}
}

func TestEmailChannelNotConfigured(t *testing.T) {
	ch := NewEmailChannel("", "")
	if ch.Available() {
		t.Error("expected not available without config")
	}
	if err := ch.Send(OutboundMessage{}); err == nil {
		t.Error("expected error sending without config")
	}
}

func TestWhatsAppChannelNotConfigured(t *testing.T) {
	ch := NewWhatsAppChannel("", "", "")
	if ch.Available() {
		t.Error("expected not available without config")
	}
	if err := ch.Send(OutboundMessage{}); err == nil {
		t.Error("expected error sending without config")
	}
}

func TestEmailChannelConfigured(t *testing.T) {
	ch := NewEmailChannel("agent@company.com", "credentials")
	if !ch.Available() {
		t.Error("expected available with config")
	}
	if ch.Name() != "email" {
		t.Errorf("expected 'email', got '%s'", ch.Name())
	}
}

func TestWhatsAppChannelConfigured(t *testing.T) {
	ch := NewWhatsAppChannel("+1555001234", "ACXXX", "token")
	if !ch.Available() {
		t.Error("expected available with config")
	}
	if ch.Name() != "whatsapp" {
		t.Errorf("expected 'whatsapp', got '%s'", ch.Name())
	}
}
