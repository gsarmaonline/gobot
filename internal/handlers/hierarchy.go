package handlers

import (
	"net/http"

	"github.com/gsarmaonline/gobot/internal/models"
)

type HierarchyHandler struct {
	hierarchy *models.HierarchyStore
}

func NewHierarchyHandler(hierarchy *models.HierarchyStore) *HierarchyHandler {
	return &HierarchyHandler{hierarchy: hierarchy}
}

type createHierarchyRequest struct {
	OrgID        string `json:"orgId"`
	ParentType   string `json:"parentType"`
	ParentID     string `json:"parentId"`
	ChildAgentID string `json:"childAgentId"`
	Relationship string `json:"relationship,omitempty"`
}

func (h *HierarchyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createHierarchyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrgID == "" || req.ParentType == "" || req.ParentID == "" || req.ChildAgentID == "" {
		writeError(w, http.StatusBadRequest, "orgId, parentType, parentId, and childAgentId are required")
		return
	}

	if req.ParentType != "user" && req.ParentType != "agent" {
		writeError(w, http.StatusBadRequest, "parentType must be 'user' or 'agent'")
		return
	}

	rel := &models.HierarchyRelation{
		OrgID:        req.OrgID,
		ParentType:   req.ParentType,
		ParentID:     req.ParentID,
		ChildAgentID: req.ChildAgentID,
		Relationship: defaultStr(req.Relationship, "reports_to"),
	}

	if err := h.hierarchy.Create(rel); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create hierarchy relation")
		return
	}

	writeJSON(w, http.StatusCreated, rel)
}

func (h *HierarchyHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "orgId query parameter is required")
		return
	}

	tree, err := h.hierarchy.BuildTree(orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build hierarchy tree")
		return
	}

	if tree == nil {
		tree = []models.HierarchyNode{}
	}
	writeJSON(w, http.StatusOK, tree)
}

func (h *HierarchyHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "orgId query parameter is required")
		return
	}

	relations, err := h.hierarchy.GetByOrg(orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list hierarchy relations")
		return
	}

	if relations == nil {
		relations = []models.HierarchyRelation{}
	}
	writeJSON(w, http.StatusOK, relations)
}

func (h *HierarchyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.hierarchy.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete hierarchy relation")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}
