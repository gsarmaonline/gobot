// gobot-mcp is an MCP server that exposes Gmail and Twilio tools to Claude.
// It is started by the Claude CLI as a subprocess via --mcp-config.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gsarma/gobot/internal/identity/gmail"
	"github.com/gsarma/gobot/internal/identity/twilio"
	"github.com/gsarma/gobot/internal/registry"
)

func main() {
	projectsFile := os.Getenv("PROJECTS_FILE")
	if projectsFile == "" {
		projectsFile = "projects.json"
	}

	reg, err := registry.Load(projectsFile)
	if err != nil {
		log.Fatalf("gobot-mcp: load registry: %v", err)
	}

	ctx := context.Background()
	go reg.Watch(ctx)

	gmailClient := gmail.New(reg)
	twilioClient := twilio.New(reg)

	s := server.NewMCPServer("gobot-tools", "1.0.0")

	registerGmailTools(s, gmailClient)
	registerTwilioTools(s, twilioClient)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("gobot-mcp: serve: %v", err)
	}
}

func registerGmailTools(s *server.MCPServer, g *gmail.Client) {
	// gmail_list_messages
	s.AddTool(
		mcp.NewTool("gmail_list_messages",
			mcp.WithDescription("List recent Gmail messages matching a query."),
			mcp.WithString("query",
				mcp.Description("Gmail search query (e.g. 'from:noreply@example.com is:unread')"),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of messages to return (default 10)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			query := mcp.ParseString(req, "query", "")
			maxResults := int64(mcp.ParseInt64(req, "max_results", 10))

			msgs, err := g.ListMessages(ctx, query, maxResults)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			out := ""
			for _, m := range msgs {
				out += fmt.Sprintf("ID: %s\nFrom: %s\nSubject: %s\nDate: %s\nSnippet: %s\n\n",
					m.ID, m.From, m.Subject, m.Date.Format(time.RFC3339), m.Snippet)
			}
			if out == "" {
				out = "No messages found."
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// gmail_get_message
	s.AddTool(
		mcp.NewTool("gmail_get_message",
			mcp.WithDescription("Get the full body of a Gmail message by ID."),
			mcp.WithString("message_id",
				mcp.Description("The Gmail message ID"),
				mcp.Required(),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id := mcp.ParseString(req, "message_id", "")
			if id == "" {
				return mcp.NewToolResultError("message_id is required"), nil
			}

			msg, err := g.GetMessage(ctx, id)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			out := fmt.Sprintf("ID: %s\nFrom: %s\nSubject: %s\nDate: %s\n\n%s",
				msg.ID, msg.From, msg.Subject, msg.Date.Format(time.RFC3339), msg.Body)
			return mcp.NewToolResultText(out), nil
		},
	)

	// gmail_send_email
	s.AddTool(
		mcp.NewTool("gmail_send_email",
			mcp.WithDescription("Send an email from the configured Gmail account."),
			mcp.WithString("to",
				mcp.Description("Recipient email address"),
				mcp.Required(),
			),
			mcp.WithString("subject",
				mcp.Description("Email subject"),
				mcp.Required(),
			),
			mcp.WithString("body",
				mcp.Description("Plain-text email body"),
				mcp.Required(),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			to := mcp.ParseString(req, "to", "")
			subject := mcp.ParseString(req, "subject", "")
			body := mcp.ParseString(req, "body", "")

			if to == "" || subject == "" {
				return mcp.NewToolResultError("to and subject are required"), nil
			}

			if err := g.SendEmail(ctx, to, subject, body); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("Email sent successfully."), nil
		},
	)

	// gmail_find_otp
	s.AddTool(
		mcp.NewTool("gmail_find_otp",
			mcp.WithDescription("Search recent emails for a numeric OTP code (4-8 digits)."),
			mcp.WithString("sender_pattern",
				mcp.Description("Substring to match against the sender address (e.g. 'doordash.com')"),
			),
			mcp.WithString("subject_pattern",
				mcp.Description("Substring to match against the email subject"),
			),
			mcp.WithNumber("lookback_seconds",
				mcp.Description("How many seconds back to search for emails (default 300)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			senderPattern := mcp.ParseString(req, "sender_pattern", "")
			subjectPattern := mcp.ParseString(req, "subject_pattern", "")
			lookback := mcp.ParseInt64(req, "lookback_seconds", 300)

			since := time.Now().Add(-time.Duration(lookback) * time.Second)
			code, err := g.FindOTP(ctx, senderPattern, subjectPattern, since)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("OTP: " + code), nil
		},
	)
}

func registerTwilioTools(s *server.MCPServer, t *twilio.Client) {
	// twilio_list_messages
	s.AddTool(
		mcp.NewTool("twilio_list_messages",
			mcp.WithDescription("List recent inbound SMS messages received by the configured Twilio number."),
			mcp.WithNumber("page_size",
				mcp.Description("Number of messages to return (default 20)"),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pageSize := int(mcp.ParseInt64(req, "page_size", 20))
			msgs, err := t.ListMessages(ctx, pageSize)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			out := ""
			for _, m := range msgs {
				out += fmt.Sprintf("SID: %s\nFrom: %s\nTo: %s\nDirection: %s\nSent: %s\nBody: %s\n\n",
					m.SID, m.From, m.To, m.Direction, m.SentAt.Format(time.RFC3339), m.Body)
			}
			if out == "" {
				out = "No messages found."
			}
			return mcp.NewToolResultText(out), nil
		},
	)

	// twilio_send_sms
	s.AddTool(
		mcp.NewTool("twilio_send_sms",
			mcp.WithDescription("Send an SMS from the configured Twilio phone number."),
			mcp.WithString("to",
				mcp.Description("Destination phone number in E.164 format (e.g. +15555551234)"),
				mcp.Required(),
			),
			mcp.WithString("body",
				mcp.Description("SMS message text"),
				mcp.Required(),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			to := mcp.ParseString(req, "to", "")
			body := mcp.ParseString(req, "body", "")

			if to == "" || body == "" {
				return mcp.NewToolResultError("to and body are required"), nil
			}

			if err := t.SendSMS(ctx, to, body); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText("SMS sent to " + to + "."), nil
		},
	)
}

