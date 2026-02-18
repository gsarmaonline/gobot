package hierarchy

import (
	"encoding/json"
	"strings"
)

// Permissions defines what an agent is allowed to do.
type Permissions struct {
	AllowedChannels  []string `json:"allowedChannels,omitempty"`  // e.g., ["email", "whatsapp"]
	AllowedActions   []string `json:"allowedActions,omitempty"`   // e.g., ["read_email", "send_email", "read_calendar"]
	RestrictedTopics []string `json:"restrictedTopics,omitempty"` // topics the agent should NOT handle
	MaxSpendCents    int      `json:"maxSpendCents,omitempty"`    // max cost per action in cents
}

// ParsePermissions parses a JSONB permissions field into a Permissions struct.
func ParsePermissions(raw json.RawMessage) (*Permissions, error) {
	if len(raw) == 0 || string(raw) == "{}" {
		return &Permissions{}, nil
	}
	var p Permissions
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ScopeCheck represents the result of checking if an action is within an agent's scope.
type ScopeCheck struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// CheckChannelAllowed verifies if the agent is allowed to use a specific channel.
func CheckChannelAllowed(perms *Permissions, channel string) ScopeCheck {
	if len(perms.AllowedChannels) == 0 {
		// No channel restrictions = all channels allowed
		return ScopeCheck{Allowed: true}
	}
	for _, c := range perms.AllowedChannels {
		if strings.EqualFold(c, channel) {
			return ScopeCheck{Allowed: true}
		}
	}
	return ScopeCheck{
		Allowed: false,
		Reason:  "channel " + channel + " is not in agent's allowed channels",
	}
}

// CheckActionAllowed verifies if the agent is allowed to perform a specific action.
func CheckActionAllowed(perms *Permissions, action string) ScopeCheck {
	if len(perms.AllowedActions) == 0 {
		return ScopeCheck{Allowed: true}
	}
	for _, a := range perms.AllowedActions {
		if strings.EqualFold(a, action) {
			return ScopeCheck{Allowed: true}
		}
	}
	return ScopeCheck{
		Allowed: false,
		Reason:  "action " + action + " is not in agent's allowed actions",
	}
}

// CheckTopicAllowed verifies the message content doesn't touch restricted topics.
func CheckTopicAllowed(perms *Permissions, content string) ScopeCheck {
	if len(perms.RestrictedTopics) == 0 {
		return ScopeCheck{Allowed: true}
	}
	lower := strings.ToLower(content)
	for _, topic := range perms.RestrictedTopics {
		if strings.Contains(lower, strings.ToLower(topic)) {
			return ScopeCheck{
				Allowed: false,
				Reason:  "message contains restricted topic: " + topic,
			}
		}
	}
	return ScopeCheck{Allowed: true}
}

// CheckSpendAllowed verifies the action cost is within budget.
func CheckSpendAllowed(perms *Permissions, costCents int) ScopeCheck {
	if perms.MaxSpendCents == 0 {
		return ScopeCheck{Allowed: true}
	}
	if costCents > perms.MaxSpendCents {
		return ScopeCheck{
			Allowed: false,
			Reason:  "cost exceeds agent's spending limit",
		}
	}
	return ScopeCheck{Allowed: true}
}
