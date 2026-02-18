package approval

import (
	"fmt"
	"sync"
	"time"
)

// Queue manages pending approval requests.
// This is an in-memory implementation. A production version would use PostgreSQL.
type Queue struct {
	mu       sync.RWMutex
	requests map[string]*Request // id -> request
	nextID   int
}

func NewQueue() *Queue {
	return &Queue{
		requests: make(map[string]*Request),
	}
}

// Submit adds a new approval request to the queue.
func (q *Queue) Submit(req *Request) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.nextID++
	req.ID = fmt.Sprintf("approval-%d", q.nextID)
	req.Status = "pending"
	req.CreatedAt = time.Now()
	q.requests[req.ID] = req
	return req.ID
}

// Get retrieves an approval request by ID.
func (q *Queue) Get(id string) (*Request, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	req, ok := q.requests[id]
	if !ok {
		return nil, fmt.Errorf("approval request %s not found", id)
	}
	return req, nil
}

// Decide approves or rejects a pending request.
func (q *Queue) Decide(decision Decision) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	req, ok := q.requests[decision.RequestID]
	if !ok {
		return fmt.Errorf("approval request %s not found", decision.RequestID)
	}

	if req.Status != "pending" {
		return fmt.Errorf("approval request %s is already %s", decision.RequestID, req.Status)
	}

	now := time.Now()
	req.ReviewedAt = &now
	req.ReviewerType = decision.ReviewerType
	req.ReviewerID = decision.ReviewerID
	req.ReviewNote = decision.Note

	if decision.Approved {
		req.Status = "approved"
	} else {
		req.Status = "rejected"
	}

	return nil
}

// ListPending returns all pending requests, optionally filtered by org.
func (q *Queue) ListPending(orgID string) []*Request {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var pending []*Request
	for _, req := range q.requests {
		if req.Status == "pending" {
			if orgID == "" || req.OrgID == orgID {
				pending = append(pending, req)
			}
		}
	}
	return pending
}

// ListByAgent returns all requests for a specific agent.
func (q *Queue) ListByAgent(agentID string) []*Request {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var results []*Request
	for _, req := range q.requests {
		if req.AgentID == agentID {
			results = append(results, req)
		}
	}
	return results
}

// Count returns the number of pending requests.
func (q *Queue) Count(orgID string) int {
	return len(q.ListPending(orgID))
}
