package runtime

import (
	"fmt"
	"log"

	"github.com/gsarmaonline/gobot/internal/llm"
)

// IncomingMessage represents a message received by an agent from any channel.
type IncomingMessage struct {
	Channel   string // "email", "whatsapp", "phone", "internal", "api"
	From      string // sender identifier
	Content   string // message content
	MessageID string // unique message ID for reply threading
}

// OutgoingMessage represents a response from an agent.
type OutgoingMessage struct {
	Channel   string // where to send the response
	To        string // recipient
	Content   string // response content
	InReplyTo string // original message ID
}

// AgentWorker handles message processing for a single agent.
type AgentWorker struct {
	ctx        *AgentContext
	inbox      chan IncomingMessage
	outbox     chan OutgoingMessage
	stopCh     chan struct{}
	maxHistory int
}

func NewAgentWorker(ctx *AgentContext) *AgentWorker {
	return &AgentWorker{
		ctx:        ctx,
		inbox:      make(chan IncomingMessage, 100),
		outbox:     make(chan OutgoingMessage, 100),
		stopCh:     make(chan struct{}),
		maxHistory: 50,
	}
}

// Inbox returns the channel for sending messages to this agent.
func (w *AgentWorker) Inbox() chan<- IncomingMessage {
	return w.inbox
}

// Outbox returns the channel for receiving responses from this agent.
func (w *AgentWorker) Outbox() <-chan OutgoingMessage {
	return w.outbox
}

// Run starts the agent's main processing loop. Blocks until Stop is called.
func (w *AgentWorker) Run() {
	log.Printf("[agent:%s] Worker started (provider: %s, model: %s)",
		w.ctx.Agent.Name, w.ctx.Agent.LLMProvider, w.ctx.Agent.LLMModel)

	for {
		select {
		case msg := <-w.inbox:
			w.handleMessage(msg)
		case <-w.stopCh:
			log.Printf("[agent:%s] Worker stopped", w.ctx.Agent.Name)
			return
		}
	}
}

// Stop signals the worker to shut down.
func (w *AgentWorker) Stop() {
	close(w.stopCh)
}

// ProcessMessage handles a single message synchronously (useful for testing).
func (w *AgentWorker) ProcessMessage(msg IncomingMessage) (*OutgoingMessage, error) {
	return w.process(msg)
}

func (w *AgentWorker) handleMessage(msg IncomingMessage) {
	response, err := w.process(msg)
	if err != nil {
		log.Printf("[agent:%s] Error processing message: %v", w.ctx.Agent.Name, err)
		return
	}

	select {
	case w.outbox <- *response:
	default:
		log.Printf("[agent:%s] Outbox full, dropping response", w.ctx.Agent.Name)
	}
}

func (w *AgentWorker) process(msg IncomingMessage) (*OutgoingMessage, error) {
	// 1. Store incoming message in memory
	if err := w.ctx.Memory.AddMessage(w.ctx.Agent.ID, llm.Message{
		Role:    "user",
		Content: fmt.Sprintf("[%s from %s] %s", msg.Channel, msg.From, msg.Content),
	}); err != nil {
		return nil, fmt.Errorf("failed to store message: %w", err)
	}

	// 2. Get conversation history
	history, err := w.ctx.Memory.GetHistory(w.ctx.Agent.ID, w.maxHistory)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// 3. Build LLM request
	systemPrompt := w.ctx.BuildSystemPrompt()
	llmReq := &llm.Request{
		Messages:    history,
		Model:       w.ctx.Agent.LLMModel,
		Temperature: w.ctx.Agent.Temperature,
		SystemMsg:   systemPrompt,
	}

	// 4. Call LLM
	resp, err := w.ctx.LLMRouter.Chat(w.ctx.Agent.LLMProvider, llmReq)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 5. Track token usage
	w.ctx.TokenUsage.Add(resp)

	// 6. Store response in memory
	if err := w.ctx.Memory.AddMessage(w.ctx.Agent.ID, llm.Message{
		Role:    "assistant",
		Content: resp.Content,
	}); err != nil {
		return nil, fmt.Errorf("failed to store response: %w", err)
	}

	// 7. Return outgoing message
	return &OutgoingMessage{
		Channel:   msg.Channel,
		To:        msg.From,
		Content:   resp.Content,
		InReplyTo: msg.MessageID,
	}, nil
}
