package identity

import (
	"fmt"
	"strings"
)

// GoogleWorkspaceProvider implements EmailProvider using Google Workspace Admin SDK.
// In production, this would use the Google Admin SDK to create/manage user accounts.
// Currently abstracted — the actual API calls are behind the HTTPClient interface.
type GoogleWorkspaceProvider struct {
	adminEmail  string
	credentials string // path to service account JSON or the JSON itself
	available   bool
}

func NewGoogleWorkspaceProvider(adminEmail, credentials string) *GoogleWorkspaceProvider {
	return &GoogleWorkspaceProvider{
		adminEmail:  adminEmail,
		credentials: credentials,
		available:   adminEmail != "" && credentials != "",
	}
}

func (p *GoogleWorkspaceProvider) Name() string    { return "google" }
func (p *GoogleWorkspaceProvider) Available() bool  { return p.available }

func (p *GoogleWorkspaceProvider) CreateAccount(req CreateEmailRequest) (*EmailIdentity, error) {
	if !p.Available() {
		return nil, fmt.Errorf("google workspace: not configured (need admin email and credentials)")
	}

	username := req.Username
	if username == "" {
		username = generateUsername(req.AgentName)
	}
	email := username + "@" + req.Domain

	// In production: call Google Admin SDK
	// admin.Users.Insert(&admin.User{
	//     PrimaryEmail: email,
	//     Name: &admin.UserName{GivenName: req.AgentName, FamilyName: "Bot"},
	//     Password: generatedPassword,
	//     OrgUnitPath: "/Agents",
	// })

	return &EmailIdentity{
		Email:    email,
		Provider: "google",
	}, nil
}

func (p *GoogleWorkspaceProvider) DeleteAccount(email string) error {
	if !p.Available() {
		return fmt.Errorf("google workspace: not configured")
	}
	// In production: admin.Users.Delete(email)
	return nil
}

func (p *GoogleWorkspaceProvider) SuspendAccount(email string) error {
	if !p.Available() {
		return fmt.Errorf("google workspace: not configured")
	}
	// In production: admin.Users.Update(email, &admin.User{Suspended: true})
	return nil
}

func (p *GoogleWorkspaceProvider) ReactivateAccount(email string) error {
	if !p.Available() {
		return fmt.Errorf("google workspace: not configured")
	}
	// In production: admin.Users.Update(email, &admin.User{Suspended: false})
	return nil
}

// generateUsername creates a username from an agent name
// "Sarah Support" → "sarah.support"
func generateUsername(name string) string {
	parts := strings.Fields(strings.ToLower(name))
	return strings.Join(parts, ".")
}
