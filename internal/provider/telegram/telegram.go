package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gsarma/gobot/internal/provider"
	"github.com/gsarma/gobot/internal/registry"
)

// SessionClearer can remove a stored session by key (implemented by Orchestrator).
type SessionClearer interface {
	ClearSession(key string)
}

// Telegram implements provider.Provider using the Telegram Bot API.
type Telegram struct {
	bot      *tgbotapi.BotAPI
	reg      *registry.Registry
	sessions SessionClearer // optional; set after orchestrator is created
}

// New creates a new Telegram provider.
func New(token string, reg *registry.Registry) (*Telegram, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram bot init: %w", err)
	}
	log.Printf("Telegram bot authorized as @%s", bot.Self.UserName)
	return &Telegram{bot: bot, reg: reg}, nil
}

// SetSessionClearer wires in the orchestrator so /setproject can clear stale sessions.
func (t *Telegram) SetSessionClearer(sc SessionClearer) {
	t.sessions = sc
}

// Name returns the provider name.
func (t *Telegram) Name() string { return "telegram" }

// Streaming returns true — Telegram supports incremental message delivery.
func (t *Telegram) Streaming() bool { return true }

// SendTyping sends a "typing" chat action.
func (t *Telegram) SendTyping(ctx context.Context, chatID string) error {
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", chatID, err)
	}
	action := tgbotapi.NewChatAction(id, tgbotapi.ChatTyping)
	_, err = t.bot.Request(action)
	return err
}

// Send sends a text message to the given chat (and thread, if specified).
func (t *Telegram) Send(ctx context.Context, out provider.OutboundMessage) error {
	chatID, err := strconv.ParseInt(out.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID %q: %w", out.ChatID, err)
	}
	msg := tgbotapi.NewMessage(chatID, out.Text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err = t.bot.Send(msg)
	return err
}

// Messages returns a channel of incoming messages via long-polling.
func (t *Telegram) Messages(ctx context.Context) (<-chan provider.InboundMessage, error) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := t.bot.GetUpdatesChan(u)
	out := make(chan provider.InboundMessage, 16)

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				t.bot.StopReceivingUpdates()
				return
			case update, ok := <-updates:
				if !ok {
					return
				}
				t.handleUpdate(ctx, update, out)
			}
		}
	}()

	return out, nil
}

func (t *Telegram) handleUpdate(ctx context.Context, update tgbotapi.Update, out chan<- provider.InboundMessage) {
	m := update.Message
	if m == nil || m.Text == "" {
		return
	}

	chatID := m.Chat.ID
	chatIDStr := strconv.FormatInt(chatID, 10)
	text := m.Text

	// Admin commands are handled internally and never forwarded.
	if t.reg.IsAdminChat(chatID) && strings.HasPrefix(text, "/") {
		go t.handleCommand(chatID, chatIDStr, text)
		return
	}

	// Look up the project bound to this chat, falling back to the default.
	project, ok := t.reg.ProjectForChat(chatIDStr)
	if !ok {
		project = t.reg.DefaultProject()
		if project == "" {
			t.sendText(chatID, "No project bound. Use /setproject <name>.")
			return
		}
	}

	senderName := ""
	if m.From != nil {
		senderName = m.From.FirstName
		if m.From.LastName != "" {
			senderName += " " + m.From.LastName
		}
		if senderName == "" {
			senderName = m.From.UserName
		}
	}

	msg := provider.InboundMessage{
		ID:         strconv.Itoa(m.MessageID),
		ChatID:     chatIDStr,
		SenderName: senderName,
		Text:       text,
		Timestamp:  int64(m.Date),
		Meta:       map[string]string{"project": project},
	}

	select {
	case out <- msg:
	case <-ctx.Done():
	}
}

func (t *Telegram) handleCommand(chatID int64, chatIDStr, text string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}
	cmd := parts[0]

	switch cmd {
	case "/addproject":
		if len(parts) < 3 {
			t.sendText(chatID, "Usage: /addproject <name> <path>")
			return
		}
		name, path := parts[1], parts[2]
		err := t.reg.Save(func(d *registry.Data) {
			if d.Projects == nil {
				d.Projects = make(map[string]registry.Project)
			}
			d.Projects[name] = registry.Project{WorkDir: path}
		})
		if err != nil {
			t.sendText(chatID, fmt.Sprintf("Error saving project: %v", err))
			return
		}
		t.sendText(chatID, fmt.Sprintf("Project %q added with workDir %q.", name, path))

	case "/setproject":
		if len(parts) < 2 {
			t.sendText(chatID, "Usage: /setproject <name>")
			return
		}
		name := parts[1]
		err := t.reg.Save(func(d *registry.Data) {
			if d.Telegram == nil {
				d.Telegram = &registry.TelegramConfig{}
			}
			if d.Telegram.ChatBindings == nil {
				d.Telegram.ChatBindings = make(map[string]string)
			}
			d.Telegram.ChatBindings[chatIDStr] = name
		})
		if err != nil {
			t.sendText(chatID, fmt.Sprintf("Error saving binding: %v", err))
			return
		}
		// Clear any existing session so the next message starts fresh in the new workDir.
		if t.sessions != nil {
			t.sessions.ClearSession("telegram:" + chatIDStr)
		}
		t.sendText(chatID, fmt.Sprintf("Chat bound to project %q. Starting fresh session.", name))

	case "/addlinear":
		if len(parts) < 3 {
			t.sendText(chatID, "Usage: /addlinear <teamKey> <project>")
			return
		}
		teamKey, project := parts[1], parts[2]
		if t.reg.Get().Linear == nil {
			t.sendText(chatID, "Linear section not configured in projects.json.")
			return
		}
		err := t.reg.Save(func(d *registry.Data) {
			if d.Linear.TeamBindings == nil {
				d.Linear.TeamBindings = make(map[string]string)
			}
			d.Linear.TeamBindings[teamKey] = project
		})
		if err != nil {
			t.sendText(chatID, fmt.Sprintf("Error saving Linear binding: %v", err))
			return
		}
		t.sendText(chatID, fmt.Sprintf("Linear team %q bound to project %q.", teamKey, project))

	case "/listprojects":
		data := t.reg.Get()
		var sb strings.Builder
		sb.WriteString("*Projects:*\n")
		for name, p := range data.Projects {
			sb.WriteString(fmt.Sprintf("  • %s → %s\n", name, p.WorkDir))
		}
		if data.Telegram != nil && len(data.Telegram.ChatBindings) > 0 {
			sb.WriteString("\n*Telegram chat bindings:*\n")
			for chat, proj := range data.Telegram.ChatBindings {
				sb.WriteString(fmt.Sprintf("  • chat %s → %s\n", chat, proj))
			}
		}
		if data.Linear != nil && len(data.Linear.TeamBindings) > 0 {
			sb.WriteString("\n*Linear team bindings:*\n")
			for team, proj := range data.Linear.TeamBindings {
				sb.WriteString(fmt.Sprintf("  • team %s → %s\n", team, proj))
			}
		}
		t.sendText(chatID, sb.String())

	default:
		t.sendText(chatID, "Commands:\n"+
			"  /addproject <name> <path> — add or update a project\n"+
			"  /setproject <name>        — bind this chat to a project\n"+
			"  /addlinear <teamKey> <project> — bind a Linear team to a project\n"+
			"  /listprojects             — show all projects and bindings",
		)
	}
	log.Printf("telegram: admin command %q from chat %s", cmd, chatIDStr)
}

// Broadcast sends text to all configured admin chat IDs.
func (t *Telegram) Broadcast(text string) {
	data := t.reg.Get()
	if data.Telegram == nil {
		return
	}
	for _, chatID := range data.Telegram.AdminChatIDs {
		t.sendText(chatID, text)
	}
}

func (t *Telegram) sendText(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := t.bot.Send(msg); err != nil {
		log.Printf("telegram: sendText to %d: %v", chatID, err)
	}
}
