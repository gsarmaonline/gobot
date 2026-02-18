package identity

import "fmt"

// TwilioProvider implements PhoneProvider using Twilio's API.
// In production, this would use the Twilio Go SDK to provision numbers.
type TwilioProvider struct {
	accountSID string
	authToken  string
	available  bool
}

func NewTwilioProvider(accountSID, authToken string) *TwilioProvider {
	return &TwilioProvider{
		accountSID: accountSID,
		authToken:  authToken,
		available:  accountSID != "" && authToken != "",
	}
}

func (p *TwilioProvider) Name() string    { return "twilio" }
func (p *TwilioProvider) Available() bool  { return p.available }

func (p *TwilioProvider) ProvisionNumber(req ProvisionPhoneRequest) (*PhoneIdentity, error) {
	if !p.Available() {
		return nil, fmt.Errorf("twilio: not configured (need account SID and auth token)")
	}

	// In production: use Twilio API to search and purchase a number
	// client := twilio.NewRestClientWithParams(twilio.ClientParams{
	//     Username: p.accountSID,
	//     Password: p.authToken,
	// })
	// params := &api.CreateIncomingPhoneNumberParams{}
	// params.SetAreaCode(req.AreaCode)
	// number, err := client.Api.CreateIncomingPhoneNumber(params)

	return &PhoneIdentity{
		Provider:        "twilio",
		WhatsAppEnabled: req.WhatsApp,
	}, nil
}

func (p *TwilioProvider) ReleaseNumber(phoneNumber string) error {
	if !p.Available() {
		return fmt.Errorf("twilio: not configured")
	}
	// In production: client.Api.DeleteIncomingPhoneNumber(sid)
	return nil
}

func (p *TwilioProvider) EnableWhatsApp(phoneNumber string) error {
	if !p.Available() {
		return fmt.Errorf("twilio: not configured")
	}
	// In production: register number with WhatsApp Business API
	// This involves Twilio's WhatsApp sender registration
	return nil
}
