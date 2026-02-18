package approval

import "time"

// Request represents an action that needs human/manager approval.
type Request struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"orgId"`
	AgentID     string    `json:"agentId"`
	AgentName   string    `json:"agentName"`
	Action      string    `json:"action"`      // what the agent wants to do
	Description string    `json:"description"` // human-readable explanation
	Channel     string    `json:"channel"`     // which channel this is for
	Payload     string    `json:"payload"`     // the actual content (email draft, message, etc.)
	Status      string    `json:"status"`      // "pending" | "approved" | "rejected"
	ReviewerType string   `json:"reviewerType,omitempty"` // "user" | "agent"
	ReviewerID   string   `json:"reviewerID,omitempty"`
	ReviewNote   string   `json:"reviewNote,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	ReviewedAt  *time.Time `json:"reviewedAt,omitempty"`
}

// Decision represents an approval or rejection.
type Decision struct {
	RequestID   string `json:"requestId"`
	Approved    bool   `json:"approved"`
	ReviewerType string `json:"reviewerType"`
	ReviewerID  string `json:"reviewerId"`
	Note        string `json:"note,omitempty"`
}
