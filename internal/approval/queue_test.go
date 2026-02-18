package approval

import "testing"

func TestQueueSubmitAndGet(t *testing.T) {
	q := NewQueue()

	req := &Request{
		OrgID:       "org-1",
		AgentID:     "agent-1",
		AgentName:   "SupportBot",
		Action:      "send_email",
		Description: "Want to send email to customer",
		Channel:     "email",
		Payload:     "Dear customer, your order is shipped.",
	}

	id := q.Submit(req)
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	got, err := q.Get(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != "pending" {
		t.Errorf("expected status 'pending', got '%s'", got.Status)
	}
	if got.AgentName != "SupportBot" {
		t.Errorf("expected agent name 'SupportBot', got '%s'", got.AgentName)
	}
}

func TestQueueDecideApprove(t *testing.T) {
	q := NewQueue()
	id := q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1", Action: "send"})

	err := q.Decide(Decision{
		RequestID:    id,
		Approved:     true,
		ReviewerType: "user",
		ReviewerID:   "user-1",
		Note:         "Looks good",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := q.Get(id)
	if got.Status != "approved" {
		t.Errorf("expected status 'approved', got '%s'", got.Status)
	}
	if got.ReviewerID != "user-1" {
		t.Errorf("expected reviewer 'user-1', got '%s'", got.ReviewerID)
	}
	if got.ReviewedAt == nil {
		t.Error("expected ReviewedAt to be set")
	}
}

func TestQueueDecideReject(t *testing.T) {
	q := NewQueue()
	id := q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1", Action: "delete"})

	err := q.Decide(Decision{
		RequestID:    id,
		Approved:     false,
		ReviewerType: "user",
		ReviewerID:   "user-1",
		Note:         "Too risky",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := q.Get(id)
	if got.Status != "rejected" {
		t.Errorf("expected status 'rejected', got '%s'", got.Status)
	}
}

func TestQueueDecideAlreadyDecided(t *testing.T) {
	q := NewQueue()
	id := q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1"})

	q.Decide(Decision{RequestID: id, Approved: true, ReviewerType: "user", ReviewerID: "u1"})

	// Try to decide again
	err := q.Decide(Decision{RequestID: id, Approved: false, ReviewerType: "user", ReviewerID: "u2"})
	if err == nil {
		t.Error("expected error deciding on already-decided request")
	}
}

func TestQueueListPending(t *testing.T) {
	q := NewQueue()
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1"})
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-2"})
	q.Submit(&Request{OrgID: "org-2", AgentID: "agent-3"})

	// Approve one
	pending := q.ListPending("org-1")
	if len(pending) != 2 {
		t.Errorf("expected 2 pending for org-1, got %d", len(pending))
	}

	allPending := q.ListPending("")
	if len(allPending) != 3 {
		t.Errorf("expected 3 total pending, got %d", len(allPending))
	}
}

func TestQueueListByAgent(t *testing.T) {
	q := NewQueue()
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1"})
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1"})
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-2"})

	results := q.ListByAgent("agent-1")
	if len(results) != 2 {
		t.Errorf("expected 2 requests for agent-1, got %d", len(results))
	}
}

func TestQueueCount(t *testing.T) {
	q := NewQueue()
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-1"})
	q.Submit(&Request{OrgID: "org-1", AgentID: "agent-2"})

	if count := q.Count("org-1"); count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
	if count := q.Count("org-2"); count != 0 {
		t.Errorf("expected count 0 for org-2, got %d", count)
	}
}

func TestQueueGetNotFound(t *testing.T) {
	q := NewQueue()
	_, err := q.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent request")
	}
}
