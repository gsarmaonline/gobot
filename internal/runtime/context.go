package runtime

import (
	"github.com/gsarmaonline/gobot/internal/llm"
	"github.com/gsarmaonline/gobot/internal/memory"
	"github.com/gsarmaonline/gobot/internal/models"
)

// AgentContext holds everything an agent worker needs to operate.
type AgentContext struct {
	Agent      *models.Agent
	LLMRouter  *llm.Router
	Memory     memory.Store
	TokenUsage *llm.TokenUsage
}

// NewAgentContext creates a new context for an agent worker.
func NewAgentContext(agent *models.Agent, router *llm.Router, mem memory.Store) *AgentContext {
	return &AgentContext{
		Agent:      agent,
		LLMRouter:  router,
		Memory:     mem,
		TokenUsage: &llm.TokenUsage{},
	}
}

// BuildSystemPrompt constructs the full system prompt for an agent
// including its role, scope, and personality.
func (ctx *AgentContext) BuildSystemPrompt() string {
	prompt := ""

	if ctx.Agent.SystemPrompt != "" {
		prompt = ctx.Agent.SystemPrompt
	} else {
		prompt = "You are " + ctx.Agent.Name
		if ctx.Agent.Title != "" {
			prompt += ", a " + ctx.Agent.Title
		}
		prompt += "."
	}

	if ctx.Agent.ScopeDescription != "" {
		prompt += "\n\nYour scope and responsibilities: " + ctx.Agent.ScopeDescription
	}

	switch ctx.Agent.MaxAutonomyLevel {
	case "suggest":
		prompt += "\n\nIMPORTANT: You should only suggest actions, never execute them directly. Always present your recommendations and wait for approval."
	case "act_with_approval":
		prompt += "\n\nIMPORTANT: For routine tasks within your scope, you may act directly. For anything significant or outside your defined scope, present your plan and wait for approval."
	case "fully_autonomous":
		prompt += "\n\nYou are authorized to act autonomously within your defined scope. Execute tasks directly without waiting for approval."
	}

	return prompt
}
