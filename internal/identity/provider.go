package identity

import "fmt"

// EmailIdentity represents a provisioned email account for an agent
type EmailIdentity struct {
	Email    string `json:"email"`
	Password string `json:"-"` // never serialized
	Provider string `json:"provider"` // "google" | "microsoft"
}

// PhoneIdentity represents a provisioned phone number for an agent
type PhoneIdentity struct {
	PhoneNumber    string `json:"phoneNumber"`
	WhatsAppEnabled bool   `json:"whatsAppEnabled"`
	Provider       string `json:"provider"` // "twilio"
}

// AgentIdentity holds all provisioned identities for an agent
type AgentIdentity struct {
	AgentID string         `json:"agentId"`
	Email   *EmailIdentity `json:"email,omitempty"`
	Phone   *PhoneIdentity `json:"phone,omitempty"`
}

// EmailProvider is the abstraction for creating/managing email accounts.
// Implementations: Google Workspace Admin SDK, Microsoft 365, mock.
type EmailProvider interface {
	// Name returns the provider name (e.g., "google", "microsoft")
	Name() string

	// Available returns true if the provider is configured
	Available() bool

	// CreateAccount provisions a new email account for an agent
	CreateAccount(req CreateEmailRequest) (*EmailIdentity, error)

	// DeleteAccount removes an email account
	DeleteAccount(email string) error

	// SuspendAccount disables an email account without deleting it
	SuspendAccount(email string) error

	// ReactivateAccount re-enables a suspended account
	ReactivateAccount(email string) error
}

// PhoneProvider is the abstraction for provisioning phone numbers.
// Implementations: Twilio, mock.
type PhoneProvider interface {
	// Name returns the provider name (e.g., "twilio")
	Name() string

	// Available returns true if the provider is configured
	Available() bool

	// ProvisionNumber acquires a new phone number
	ProvisionNumber(req ProvisionPhoneRequest) (*PhoneIdentity, error)

	// ReleaseNumber releases a provisioned phone number
	ReleaseNumber(phoneNumber string) error

	// EnableWhatsApp registers the number for WhatsApp Business
	EnableWhatsApp(phoneNumber string) error
}

// CreateEmailRequest contains the details for creating an agent's email account
type CreateEmailRequest struct {
	AgentName  string // e.g., "Sarah"
	AgentTitle string // e.g., "Support Rep"
	Domain     string // e.g., "company.com"
	Username   string // e.g., "sarah" → sarah@company.com (optional, auto-generated if empty)
}

// ProvisionPhoneRequest contains the details for provisioning a phone number
type ProvisionPhoneRequest struct {
	CountryCode string // e.g., "US", "IN"
	AreaCode    string // optional, e.g., "415"
	WhatsApp    bool   // whether to enable WhatsApp on this number
}

// Manager orchestrates identity provisioning across providers
type Manager struct {
	email EmailProvider
	phone PhoneProvider
}

func NewManager(email EmailProvider, phone PhoneProvider) *Manager {
	return &Manager{
		email: email,
		phone: phone,
	}
}

// ProvisionAgent creates all requested identities for an agent
func (m *Manager) ProvisionAgent(agentID, agentName, agentTitle, domain string, wantEmail, wantPhone, wantWhatsApp bool) (*AgentIdentity, error) {
	identity := &AgentIdentity{AgentID: agentID}

	if wantEmail {
		if m.email == nil || !m.email.Available() {
			return nil, fmt.Errorf("email provider not available")
		}
		email, err := m.email.CreateAccount(CreateEmailRequest{
			AgentName:  agentName,
			AgentTitle: agentTitle,
			Domain:     domain,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create email: %w", err)
		}
		identity.Email = email
	}

	if wantPhone {
		if m.phone == nil || !m.phone.Available() {
			return nil, fmt.Errorf("phone provider not available")
		}
		phone, err := m.phone.ProvisionNumber(ProvisionPhoneRequest{
			CountryCode: "US",
			WhatsApp:    wantWhatsApp,
		})
		if err != nil {
			// Rollback email if phone fails
			if identity.Email != nil && m.email != nil {
				m.email.DeleteAccount(identity.Email.Email)
			}
			return nil, fmt.Errorf("failed to provision phone: %w", err)
		}
		identity.Phone = phone
	}

	return identity, nil
}

// DeprovisionAgent removes all identities for an agent
func (m *Manager) DeprovisionAgent(identity *AgentIdentity) error {
	var errs []error

	if identity.Email != nil && m.email != nil {
		if err := m.email.DeleteAccount(identity.Email.Email); err != nil {
			errs = append(errs, fmt.Errorf("email: %w", err))
		}
	}

	if identity.Phone != nil && m.phone != nil {
		if err := m.phone.ReleaseNumber(identity.Phone.PhoneNumber); err != nil {
			errs = append(errs, fmt.Errorf("phone: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("deprovisioning errors: %v", errs)
	}
	return nil
}
