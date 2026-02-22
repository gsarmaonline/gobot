package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gsarma/gobot/internal/provider"
)

// Telegram implements provider.Provider using the Telegram Bot API.
type Telegram struct {
	bot            *tgbotapi.BotAPI
	allowedChatIDs map[int64]bool
}

// New creates a new Telegram provider.
func New(token string, allowedChatIDs []int64) (*Telegram, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("telegram bot init: %w", err)
	}

	allowed := make(map[int64]bool, len(allowedChatIDs))
	for _, id := range allowedChatIDs {
		allowed[id] = true
	}

	log.Printf("Telegram bot authorized as @%s", bot.Self.UserName)
	return &Telegram{bot: bot, allowedChatIDs: allowed}, nil
}

// Name returns the provider name.
func (t *Telegram) Name() string { return "telegram" }

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
				msg := t.toInbound(update)
				if msg == nil {
					continue
				}
				select {
				case out <- *msg:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

func (t *Telegram) toInbound(update tgbotapi.Update) *provider.InboundMessage {
	m := update.Message
	if m == nil || m.Text == "" {
		return nil
	}

	chatID := strconv.FormatInt(m.Chat.ID, 10)

	// Enforce allow-list if configured.
	if len(t.allowedChatIDs) > 0 && !t.allowedChatIDs[m.Chat.ID] {
		log.Printf("ignoring message from unauthorized chat %s", chatID)
		return nil
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

	return &provider.InboundMessage{
		ID:         strconv.Itoa(m.MessageID),
		ChatID:     chatID,
		SenderName: senderName,
		Text:       m.Text,
		Timestamp:  int64(m.Date),
	}
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
