package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gsarmaonline/gobot/internal/models"
)

type OrganizationHandler struct {
	orgs *models.OrganizationStore
}

func NewOrganizationHandler(orgs *models.OrganizationStore) *OrganizationHandler {
	return &OrganizationHandler{orgs: orgs}
}

type createOrgRequest struct {
	Name                string `json:"name"`
	Domain              string `json:"domain,omitempty"`
	WorkspaceType       string `json:"workspaceType,omitempty"`
	WorkspaceAdminEmail string `json:"workspaceAdminEmail,omitempty"`
}

func (h *OrganizationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createOrgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	org := &models.Organization{
		Name:                req.Name,
		Domain:              req.Domain,
		WorkspaceType:       req.WorkspaceType,
		WorkspaceAdminEmail: req.WorkspaceAdminEmail,
	}

	if err := h.orgs.Create(org); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create organization")
		return
	}

	writeJSON(w, http.StatusCreated, org)
}

func (h *OrganizationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	org, err := h.orgs.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "organization not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get organization")
		return
	}
	writeJSON(w, http.StatusOK, org)
}

func (h *OrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgs, err := h.orgs.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}

	if orgs == nil {
		orgs = []models.Organization{}
	}
	writeJSON(w, http.StatusOK, orgs)
}
