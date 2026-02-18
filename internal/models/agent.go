package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Agent struct {
	ID               string          `json:"id"`
	OrgID            string          `json:"orgId"`
	DepartmentID     string          `json:"departmentId,omitempty"`
	Name             string          `json:"name"`
	Title            string          `json:"title,omitempty"`
	Email            string          `json:"email,omitempty"`
	PhoneNumber      string          `json:"phoneNumber,omitempty"`
	WhatsappNumber   string          `json:"whatsappNumber,omitempty"`
	Status           string          `json:"status"`
	LLMProvider      string          `json:"llmProvider"`
	LLMModel         string          `json:"llmModel"`
	SystemPrompt     string          `json:"systemPrompt,omitempty"`
	Temperature      float64         `json:"temperature"`
	Permissions      json.RawMessage `json:"permissions,omitempty"`
	ScopeDescription string          `json:"scopeDescription,omitempty"`
	MaxAutonomyLevel string          `json:"maxAutonomyLevel"`
	ReportsToUserID  string          `json:"reportsToUserId,omitempty"`
	ReportsToAgentID string          `json:"reportsToAgentId,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type AgentStore struct {
	db *sql.DB
}

func NewAgentStore(db *sql.DB) *AgentStore {
	return &AgentStore{db: db}
}

func (s *AgentStore) Create(agent *Agent) error {
	return s.db.QueryRow(
		`INSERT INTO agents (org_id, department_id, name, title, email, phone_number,
		 whatsapp_number, status, llm_provider, llm_model, system_prompt, temperature,
		 permissions, scope_description, max_autonomy_level, reports_to_user_id, reports_to_agent_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		 RETURNING id, created_at, updated_at`,
		agent.OrgID, nullString(agent.DepartmentID), agent.Name, nullString(agent.Title),
		nullString(agent.Email), nullString(agent.PhoneNumber), nullString(agent.WhatsappNumber),
		agent.Status, agent.LLMProvider, agent.LLMModel, nullString(agent.SystemPrompt),
		agent.Temperature, defaultJSON(agent.Permissions), nullString(agent.ScopeDescription),
		agent.MaxAutonomyLevel, nullString(agent.ReportsToUserID), nullString(agent.ReportsToAgentID),
	).Scan(&agent.ID, &agent.CreatedAt, &agent.UpdatedAt)
}

func (s *AgentStore) GetByID(id string) (*Agent, error) {
	agent := &Agent{}
	err := s.db.QueryRow(
		`SELECT id, org_id, COALESCE(department_id::text, ''), name, COALESCE(title, ''),
		 COALESCE(email, ''), COALESCE(phone_number, ''), COALESCE(whatsapp_number, ''),
		 status, llm_provider, llm_model, COALESCE(system_prompt, ''), temperature,
		 COALESCE(permissions, '{}'), COALESCE(scope_description, ''),
		 max_autonomy_level, COALESCE(reports_to_user_id::text, ''),
		 COALESCE(reports_to_agent_id::text, ''), created_at, updated_at
		 FROM agents WHERE id = $1`, id,
	).Scan(&agent.ID, &agent.OrgID, &agent.DepartmentID, &agent.Name, &agent.Title,
		&agent.Email, &agent.PhoneNumber, &agent.WhatsappNumber,
		&agent.Status, &agent.LLMProvider, &agent.LLMModel, &agent.SystemPrompt,
		&agent.Temperature, &agent.Permissions, &agent.ScopeDescription,
		&agent.MaxAutonomyLevel, &agent.ReportsToUserID, &agent.ReportsToAgentID,
		&agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentStore) ListByOrg(orgID string) ([]Agent, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, COALESCE(department_id::text, ''), name, COALESCE(title, ''),
		 COALESCE(email, ''), COALESCE(phone_number, ''), COALESCE(whatsapp_number, ''),
		 status, llm_provider, llm_model, COALESCE(system_prompt, ''), temperature,
		 COALESCE(permissions, '{}'), COALESCE(scope_description, ''),
		 max_autonomy_level, COALESCE(reports_to_user_id::text, ''),
		 COALESCE(reports_to_agent_id::text, ''), created_at, updated_at
		 FROM agents WHERE org_id = $1 ORDER BY created_at DESC`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var agent Agent
		if err := rows.Scan(&agent.ID, &agent.OrgID, &agent.DepartmentID, &agent.Name, &agent.Title,
			&agent.Email, &agent.PhoneNumber, &agent.WhatsappNumber,
			&agent.Status, &agent.LLMProvider, &agent.LLMModel, &agent.SystemPrompt,
			&agent.Temperature, &agent.Permissions, &agent.ScopeDescription,
			&agent.MaxAutonomyLevel, &agent.ReportsToUserID, &agent.ReportsToAgentID,
			&agent.CreatedAt, &agent.UpdatedAt); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *AgentStore) Update(agent *Agent) error {
	_, err := s.db.Exec(
		`UPDATE agents SET department_id = $2, name = $3, title = $4, email = $5,
		 phone_number = $6, whatsapp_number = $7, llm_provider = $8, llm_model = $9,
		 system_prompt = $10, temperature = $11, permissions = $12, scope_description = $13,
		 max_autonomy_level = $14, reports_to_user_id = $15, reports_to_agent_id = $16,
		 updated_at = NOW()
		 WHERE id = $1`,
		agent.ID, nullString(agent.DepartmentID), agent.Name, nullString(agent.Title),
		nullString(agent.Email), nullString(agent.PhoneNumber), nullString(agent.WhatsappNumber),
		agent.LLMProvider, agent.LLMModel, nullString(agent.SystemPrompt), agent.Temperature,
		defaultJSON(agent.Permissions), nullString(agent.ScopeDescription),
		agent.MaxAutonomyLevel, nullString(agent.ReportsToUserID), nullString(agent.ReportsToAgentID),
	)
	return err
}

func (s *AgentStore) UpdateStatus(id string, status string) error {
	_, err := s.db.Exec(
		`UPDATE agents SET status = $2, updated_at = NOW() WHERE id = $1`,
		id, status,
	)
	return err
}

func (s *AgentStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM agents WHERE id = $1`, id)
	return err
}
