package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gsarmaonline/gobot/internal/models"
)

type AgentHandler struct {
	agents *models.AgentStore
}

func NewAgentHandler(agents *models.AgentStore) *AgentHandler {
	return &AgentHandler{agents: agents}
}

type createAgentRequest struct {
	OrgID            string  `json:"orgId"`
	DepartmentID     string  `json:"departmentId,omitempty"`
	Name             string  `json:"name"`
	Title            string  `json:"title,omitempty"`
	LLMProvider      string  `json:"llmProvider,omitempty"`
	LLMModel         string  `json:"llmModel,omitempty"`
	SystemPrompt     string  `json:"systemPrompt,omitempty"`
	Temperature      float64 `json:"temperature,omitempty"`
	ScopeDescription string  `json:"scopeDescription,omitempty"`
	MaxAutonomyLevel string  `json:"maxAutonomyLevel,omitempty"`
	ReportsToUserID  string  `json:"reportsToUserId,omitempty"`
	ReportsToAgentID string  `json:"reportsToAgentId,omitempty"`
}

func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.OrgID == "" {
		writeError(w, http.StatusBadRequest, "name and orgId are required")
		return
	}

	agent := &models.Agent{
		OrgID:            req.OrgID,
		DepartmentID:     req.DepartmentID,
		Name:             req.Name,
		Title:            req.Title,
		Status:           "inactive",
		LLMProvider:      defaultStr(req.LLMProvider, "claude"),
		LLMModel:         defaultStr(req.LLMModel, "claude-sonnet-4-5-20250929"),
		SystemPrompt:     req.SystemPrompt,
		Temperature:      defaultFloat(req.Temperature, 0.7),
		ScopeDescription: req.ScopeDescription,
		MaxAutonomyLevel: defaultStr(req.MaxAutonomyLevel, "suggest"),
		ReportsToUserID:  req.ReportsToUserID,
		ReportsToAgentID: req.ReportsToAgentID,
	}

	if err := h.agents.Create(agent); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create agent")
		return
	}

	writeJSON(w, http.StatusCreated, agent)
}

func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	agent, err := h.agents.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "orgId query parameter is required")
		return
	}

	agents, err := h.agents.ListByOrg(orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents")
		return
	}

	if agents == nil {
		agents = []models.Agent{}
	}
	writeJSON(w, http.StatusOK, agents)
}

func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, err := h.agents.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent")
		return
	}

	var req createAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing.Name = defaultStr(req.Name, existing.Name)
	existing.Title = defaultStr(req.Title, existing.Title)
	existing.DepartmentID = req.DepartmentID
	existing.LLMProvider = defaultStr(req.LLMProvider, existing.LLMProvider)
	existing.LLMModel = defaultStr(req.LLMModel, existing.LLMModel)
	existing.SystemPrompt = defaultStr(req.SystemPrompt, existing.SystemPrompt)
	if req.Temperature != 0 {
		existing.Temperature = req.Temperature
	}
	existing.ScopeDescription = defaultStr(req.ScopeDescription, existing.ScopeDescription)
	existing.MaxAutonomyLevel = defaultStr(req.MaxAutonomyLevel, existing.MaxAutonomyLevel)
	existing.ReportsToUserID = req.ReportsToUserID
	existing.ReportsToAgentID = req.ReportsToAgentID

	if err := h.agents.Update(existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.agents.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete agent")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *AgentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	action := r.PathValue("action")

	var status string
	switch action {
	case "start":
		status = "active"
	case "pause":
		status = "paused"
	case "stop":
		status = "inactive"
	default:
		writeError(w, http.StatusBadRequest, "invalid action: use start, pause, or stop")
		return
	}

	if err := h.agents.UpdateStatus(id, status); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": status})
}

func defaultStr(val, fallback string) string {
	if val == "" {
		return fallback
	}
	return val
}

func defaultFloat(val, fallback float64) float64 {
	if val == 0 {
		return fallback
	}
	return val
}
