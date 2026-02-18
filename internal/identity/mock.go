package identity

import "fmt"

// MockEmailProvider is a test/development implementation of EmailProvider.
// It generates fake email addresses without calling any external API.
type MockEmailProvider struct {
	domain   string
	accounts map[string]bool // track created accounts
}

func NewMockEmailProvider(domain string) *MockEmailProvider {
	return &MockEmailProvider{
		domain:   domain,
		accounts: make(map[string]bool),
	}
}

func (p *MockEmailProvider) Name() string    { return "mock" }
func (p *MockEmailProvider) Available() bool  { return true }

func (p *MockEmailProvider) CreateAccount(req CreateEmailRequest) (*EmailIdentity, error) {
	username := req.Username
	if username == "" {
		username = generateUsername(req.AgentName)
	}
	domain := req.Domain
	if domain == "" {
		domain = p.domain
	}
	email := username + "@" + domain

	if p.accounts[email] {
		return nil, fmt.Errorf("account %s already exists", email)
	}
	p.accounts[email] = true

	return &EmailIdentity{
		Email:    email,
		Provider: "mock",
	}, nil
}

func (p *MockEmailProvider) DeleteAccount(email string) error {
	if !p.accounts[email] {
		return fmt.Errorf("account %s not found", email)
	}
	delete(p.accounts, email)
	return nil
}

func (p *MockEmailProvider) SuspendAccount(email string) error {
	if !p.accounts[email] {
		return fmt.Errorf("account %s not found", email)
	}
	return nil
}

func (p *MockEmailProvider) ReactivateAccount(email string) error {
	return nil
}

// MockPhoneProvider is a test/development implementation of PhoneProvider.
type MockPhoneProvider struct {
	nextNumber int
	numbers    map[string]bool
}

func NewMockPhoneProvider() *MockPhoneProvider {
	return &MockPhoneProvider{
		nextNumber: 1000,
		numbers:    make(map[string]bool),
	}
}

func (p *MockPhoneProvider) Name() string    { return "mock" }
func (p *MockPhoneProvider) Available() bool  { return true }

func (p *MockPhoneProvider) ProvisionNumber(req ProvisionPhoneRequest) (*PhoneIdentity, error) {
	p.nextNumber++
	number := fmt.Sprintf("+1555%04d", p.nextNumber)
	p.numbers[number] = true

	return &PhoneIdentity{
		PhoneNumber:     number,
		WhatsAppEnabled: req.WhatsApp,
		Provider:        "mock",
	}, nil
}

func (p *MockPhoneProvider) ReleaseNumber(phoneNumber string) error {
	if !p.numbers[phoneNumber] {
		return fmt.Errorf("number %s not found", phoneNumber)
	}
	delete(p.numbers, phoneNumber)
	return nil
}

func (p *MockPhoneProvider) EnableWhatsApp(phoneNumber string) error {
	return nil
}
