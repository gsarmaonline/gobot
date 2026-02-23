// gobot-mcp is an MCP server that exposes Twilio SMS tools to Claude.
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

	twilioClient := twilio.New(reg)

	s := server.NewMCPServer("gobot-tools", "1.0.0")
	registerTwilioTools(s, twilioClient)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("gobot-mcp: serve: %v", err)
	}
}

func registerTwilioTools(s *server.MCPServer, t *twilio.Client) {
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
