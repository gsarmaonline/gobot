// gobot-setup performs one-time OAuth2 setup for Gmail using the device flow.
// Run once to obtain and save tokens to projects.json.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailv1 "google.golang.org/api/gmail/v1"

	"github.com/gsarma/gobot/internal/registry"
)

func main() {
	var projectsFile string
	flag.StringVar(&projectsFile, "projects-file", "", "Path to projects.json (default: PROJECTS_FILE env or projects.json)")
	flag.Parse()

	if projectsFile == "" {
		projectsFile = os.Getenv("PROJECTS_FILE")
	}
	if projectsFile == "" {
		projectsFile = "projects.json"
	}

	reg, err := registry.Load(projectsFile)
	if err != nil {
		log.Fatalf("load registry: %v", err)
	}

	data := reg.Get()
	if data.Google == nil || data.Google.ClientID == "" || data.Google.ClientSecret == "" {
		log.Fatalf("projects.json must have google.clientID and google.clientSecret set before running setup")
	}

	g := data.Google
	oauthCfg := &oauth2.Config{
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{
			gmailv1.GmailSendScope,
			gmailv1.GmailReadonlyScope,
		},
	}

	ctx := context.Background()

	deviceResp, err := oauthCfg.DeviceAuth(ctx, oauth2.AccessTypeOffline)
	if err != nil {
		log.Fatalf("device auth: %v", err)
	}

	fmt.Printf("\nVisit: %s\nEnter code: %s\n\n", deviceResp.VerificationURI, deviceResp.UserCode)
	fmt.Println("Waiting for authorization...")

	tok, err := oauthCfg.DeviceAccessToken(ctx, deviceResp)
	if err != nil {
		log.Fatalf("device access token: %v", err)
	}

	if err := reg.Save(func(d *registry.Data) {
		if d.Google == nil {
			d.Google = &registry.GoogleConfig{}
		}
		d.Google.RefreshToken = tok.RefreshToken
		d.Google.AccessToken = tok.AccessToken
		d.Google.TokenExpiry = tok.Expiry.UTC().Format("2006-01-02T15:04:05Z07:00")
	}); err != nil {
		log.Fatalf("save tokens: %v", err)
	}

	fmt.Println("Done. Tokens saved to", projectsFile)
}
