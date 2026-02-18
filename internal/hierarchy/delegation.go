package hierarchy

import (
	"fmt"
	"time"
)

// DelegationTask represents a task delegated from a parent to a subordinate agent.
type DelegationTask struct {
	ID            string    `json:"id"`
	ParentType    string    `json:"parentType"` // "user" | "agent"
	ParentID      string    `json:"parentId"`
	ChildAgentID  string    `json:"childAgentId"`
	Description   string    `json:"description"`
	Channel       string    `json:"channel,omitempty"`
	Priority      string    `json:"priority"` // "low" | "normal" | "high" | "urgent"
	Status        string    `json:"status"`   // "pending" | "in_progress" | "completed" | "failed"
	Result        string    `json:"result,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

// DelegationEngine manages task delegation to subordinate agents.
type DelegationEngine struct {
	// getChildren returns the child agents of a given parent.
	getChildren func(parentType, parentID string) ([]string, error)
}

func NewDelegationEngine(getChildren func(parentType, parentID string) ([]string, error)) *DelegationEngine {
	return &DelegationEngine{getChildren: getChildren}
}

// FindSubordinate finds an available subordinate agent to delegate a task to.
// For now, returns the first child. Could be enhanced with load balancing, skill matching, etc.
func (e *DelegationEngine) FindSubordinate(parentType, parentID string) (string, error) {
	if e.getChildren == nil {
		return "", fmt.Errorf("no hierarchy lookup function configured")
	}

	children, err := e.getChildren(parentType, parentID)
	if err != nil {
		return "", fmt.Errorf("failed to find subordinates: %w", err)
	}

	if len(children) == 0 {
		return "", fmt.Errorf("no subordinate agents found for %s:%s", parentType, parentID)
	}

	return children[0], nil
}

// CanDelegate checks if a parent has any subordinates to delegate to.
func (e *DelegationEngine) CanDelegate(parentType, parentID string) (bool, error) {
	if e.getChildren == nil {
		return false, nil
	}
	children, err := e.getChildren(parentType, parentID)
	if err != nil {
		return false, err
	}
	return len(children) > 0, nil
}
