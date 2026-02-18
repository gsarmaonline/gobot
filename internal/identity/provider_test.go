package identity

import (
	"testing"
)

func TestMockEmailProvider(t *testing.T) {
	p := NewMockEmailProvider("test.com")

	if p.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", p.Name())
	}
	if !p.Available() {
		t.Error("expected mock provider to be available")
	}

	// Create account
	email, err := p.CreateAccount(CreateEmailRequest{
		AgentName: "Sarah Support",
		Domain:    "company.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email.Email != "sarah.support@company.com" {
		t.Errorf("expected 'sarah.support@company.com', got '%s'", email.Email)
	}

	// Duplicate should fail
	_, err = p.CreateAccount(CreateEmailRequest{
		AgentName: "Sarah Support",
		Domain:    "company.com",
	})
	if err == nil {
		t.Error("expected error for duplicate account")
	}

	// Suspend
	if err := p.SuspendAccount(email.Email); err != nil {
		t.Errorf("unexpected error suspending: %v", err)
	}

	// Delete
	if err := p.DeleteAccount(email.Email); err != nil {
		t.Errorf("unexpected error deleting: %v", err)
	}

	// Delete again should fail
	if err := p.DeleteAccount(email.Email); err == nil {
		t.Error("expected error deleting non-existent account")
	}
}

func TestMockEmailProviderCustomUsername(t *testing.T) {
	p := NewMockEmailProvider("test.com")

	email, err := p.CreateAccount(CreateEmailRequest{
		AgentName: "Sarah",
		Username:  "support",
		Domain:    "company.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email.Email != "support@company.com" {
		t.Errorf("expected 'support@company.com', got '%s'", email.Email)
	}
}

func TestMockPhoneProvider(t *testing.T) {
	p := NewMockPhoneProvider()

	if p.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", p.Name())
	}
	if !p.Available() {
		t.Error("expected mock provider to be available")
	}

	// Provision number
	phone, err := p.ProvisionNumber(ProvisionPhoneRequest{
		CountryCode: "US",
		WhatsApp:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if phone.PhoneNumber == "" {
		t.Error("expected non-empty phone number")
	}
	if !phone.WhatsAppEnabled {
		t.Error("expected WhatsApp enabled")
	}

	// Provision another number — should be different
	phone2, err := p.ProvisionNumber(ProvisionPhoneRequest{CountryCode: "US"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if phone2.PhoneNumber == phone.PhoneNumber {
		t.Error("expected different phone numbers")
	}

	// Release number
	if err := p.ReleaseNumber(phone.PhoneNumber); err != nil {
		t.Errorf("unexpected error releasing: %v", err)
	}

	// Release again should fail
	if err := p.ReleaseNumber(phone.PhoneNumber); err == nil {
		t.Error("expected error releasing non-existent number")
	}
}

func TestManagerProvisionAgent(t *testing.T) {
	emailProvider := NewMockEmailProvider("test.com")
	phoneProvider := NewMockPhoneProvider()
	mgr := NewManager(emailProvider, phoneProvider)

	// Provision with email + phone + WhatsApp
	identity, err := mgr.ProvisionAgent("agent-1", "Sarah", "Support Rep", "company.com", true, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if identity.AgentID != "agent-1" {
		t.Errorf("expected agent ID 'agent-1', got '%s'", identity.AgentID)
	}
	if identity.Email == nil {
		t.Fatal("expected email identity")
	}
	if identity.Email.Email != "sarah@company.com" {
		t.Errorf("expected 'sarah@company.com', got '%s'", identity.Email.Email)
	}
	if identity.Phone == nil {
		t.Fatal("expected phone identity")
	}
	if !identity.Phone.WhatsAppEnabled {
		t.Error("expected WhatsApp enabled")
	}
}

func TestManagerProvisionEmailOnly(t *testing.T) {
	emailProvider := NewMockEmailProvider("test.com")
	mgr := NewManager(emailProvider, nil)

	identity, err := mgr.ProvisionAgent("agent-2", "Alex", "Researcher", "company.com", true, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.Email == nil {
		t.Fatal("expected email identity")
	}
	if identity.Phone != nil {
		t.Error("expected no phone identity")
	}
}

func TestManagerProvisionNoProviders(t *testing.T) {
	mgr := NewManager(nil, nil)

	_, err := mgr.ProvisionAgent("agent-3", "Bot", "", "company.com", true, false, false)
	if err == nil {
		t.Error("expected error when email provider is nil")
	}
}

func TestManagerDeprovisionAgent(t *testing.T) {
	emailProvider := NewMockEmailProvider("test.com")
	phoneProvider := NewMockPhoneProvider()
	mgr := NewManager(emailProvider, phoneProvider)

	identity, _ := mgr.ProvisionAgent("agent-4", "Tom", "Clerk", "company.com", true, true, false)

	err := mgr.DeprovisionAgent(identity)
	if err != nil {
		t.Fatalf("unexpected error deprovisioning: %v", err)
	}

	// Verify email was deleted
	if err := emailProvider.DeleteAccount(identity.Email.Email); err == nil {
		t.Error("expected error — account should already be deleted")
	}
}

func TestGoogleWorkspaceNotConfigured(t *testing.T) {
	p := NewGoogleWorkspaceProvider("", "")
	if p.Available() {
		t.Error("expected not available without credentials")
	}
	_, err := p.CreateAccount(CreateEmailRequest{AgentName: "Test", Domain: "test.com"})
	if err == nil {
		t.Error("expected error when not configured")
	}
}

func TestGoogleWorkspaceConfigured(t *testing.T) {
	p := NewGoogleWorkspaceProvider("admin@company.com", "fake-credentials")
	if !p.Available() {
		t.Error("expected available with credentials")
	}
	if p.Name() != "google" {
		t.Errorf("expected 'google', got '%s'", p.Name())
	}

	email, err := p.CreateAccount(CreateEmailRequest{
		AgentName: "Sarah",
		Domain:    "company.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if email.Email != "sarah@company.com" {
		t.Errorf("expected 'sarah@company.com', got '%s'", email.Email)
	}
}

func TestTwilioNotConfigured(t *testing.T) {
	p := NewTwilioProvider("", "")
	if p.Available() {
		t.Error("expected not available without credentials")
	}
	_, err := p.ProvisionNumber(ProvisionPhoneRequest{CountryCode: "US"})
	if err == nil {
		t.Error("expected error when not configured")
	}
}

func TestTwilioConfigured(t *testing.T) {
	p := NewTwilioProvider("ACXXX", "token")
	if !p.Available() {
		t.Error("expected available with credentials")
	}
	if p.Name() != "twilio" {
		t.Errorf("expected 'twilio', got '%s'", p.Name())
	}
}

func TestGenerateUsername(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"Sarah", "sarah"},
		{"Sarah Support", "sarah.support"},
		{"John Paul Jones", "john.paul.jones"},
		{"UPPERCASE", "uppercase"},
	}
	for _, tt := range tests {
		result := generateUsername(tt.name)
		if result != tt.expected {
			t.Errorf("generateUsername(%q) = %q, want %q", tt.name, result, tt.expected)
		}
	}
}
