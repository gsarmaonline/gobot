package hierarchy

import (
	"fmt"
	"time"
)

// EscalationReason describes why an agent is escalating.
type EscalationReason string

const (
	EscalationOutOfScope     EscalationReason = "out_of_scope"
	EscalationNeedsApproval  EscalationReason = "needs_approval"
	EscalationRestrictedTopic EscalationReason = "restricted_topic"
	EscalationBudgetExceeded EscalationReason = "budget_exceeded"
	EscalationAgentUncertain EscalationReason = "agent_uncertain"
	EscalationUserRequested  EscalationReason = "user_requested"
)

// EscalationRequest represents a request from an agent to escalate to its manager.
type EscalationRequest struct {
	ID             string           `json:"id"`
	AgentID        string           `json:"agentId"`
	AgentName      string           `json:"agentName"`
	Reason         EscalationReason `json:"reason"`
	Description    string           `json:"description"`
	OriginalMessage string          `json:"originalMessage"`
	Channel        string           `json:"channel"`
	From           string           `json:"from"`
	TargetType     string           `json:"targetType"` // "user" | "agent"
	TargetID       string           `json:"targetId"`
	Status         string           `json:"status"` // "pending" | "accepted" | "rejected" | "redirected"
	CreatedAt      time.Time        `json:"createdAt"`
}

// EscalationTarget identifies where to escalate.
type EscalationTarget struct {
	Type string `json:"type"` // "user" | "agent"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// EscalationEngine manages escalation routing based on the org hierarchy.
type EscalationEngine struct {
	// getParent returns the parent of a given agent in the hierarchy.
	// Returns parentType ("user"|"agent"), parentID, and agent/user name.
	getParent func(agentID string) (*EscalationTarget, error)
}

func NewEscalationEngine(getParent func(agentID string) (*EscalationTarget, error)) *EscalationEngine {
	return &EscalationEngine{getParent: getParent}
}

// DetermineTarget finds the right escalation target for an agent.
func (e *EscalationEngine) DetermineTarget(agentID string) (*EscalationTarget, error) {
	if e.getParent == nil {
		return nil, fmt.Errorf("no hierarchy lookup function configured")
	}
	target, err := e.getParent(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find escalation target for agent %s: %w", agentID, err)
	}
	return target, nil
}

// ShouldEscalate evaluates whether a message/action should be escalated
// based on the agent's permissions and autonomy level.
func ShouldEscalate(perms *Permissions, autonomyLevel, channel, action, content string, costCents int) *EscalationRequest {
	// Check channel
	if check := CheckChannelAllowed(perms, channel); !check.Allowed {
		return &EscalationRequest{
			Reason:      EscalationOutOfScope,
			Description: check.Reason,
		}
	}

	// Check action
	if check := CheckActionAllowed(perms, action); !check.Allowed {
		return &EscalationRequest{
			Reason:      EscalationOutOfScope,
			Description: check.Reason,
		}
	}

	// Check topic
	if check := CheckTopicAllowed(perms, content); !check.Allowed {
		return &EscalationRequest{
			Reason:      EscalationRestrictedTopic,
			Description: check.Reason,
		}
	}

	// Check budget
	if check := CheckSpendAllowed(perms, costCents); !check.Allowed {
		return &EscalationRequest{
			Reason:      EscalationBudgetExceeded,
			Description: check.Reason,
		}
	}

	// Check autonomy level
	if autonomyLevel == "suggest" {
		return &EscalationRequest{
			Reason:      EscalationNeedsApproval,
			Description: "agent autonomy level is 'suggest' — all actions require approval",
		}
	}

	return nil // No escalation needed
}
