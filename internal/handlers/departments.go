package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gsarmaonline/gobot/internal/models"
)

type DepartmentHandler struct {
	departments *models.DepartmentStore
}

func NewDepartmentHandler(departments *models.DepartmentStore) *DepartmentHandler {
	return &DepartmentHandler{departments: departments}
}

type createDepartmentRequest struct {
	OrgID       string `json:"orgId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (h *DepartmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDepartmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.OrgID == "" {
		writeError(w, http.StatusBadRequest, "name and orgId are required")
		return
	}

	dept := &models.Department{
		OrgID:       req.OrgID,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := h.departments.Create(dept); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create department")
		return
	}

	writeJSON(w, http.StatusCreated, dept)
}

func (h *DepartmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dept, err := h.departments.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "department not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get department")
		return
	}
	writeJSON(w, http.StatusOK, dept)
}

func (h *DepartmentHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		writeError(w, http.StatusBadRequest, "orgId query parameter is required")
		return
	}

	depts, err := h.departments.ListByOrg(orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list departments")
		return
	}

	if depts == nil {
		depts = []models.Department{}
	}
	writeJSON(w, http.StatusOK, depts)
}

func (h *DepartmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, err := h.departments.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "department not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get department")
		return
	}

	var req createDepartmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing.Name = defaultStr(req.Name, existing.Name)
	existing.Description = defaultStr(req.Description, existing.Description)

	if err := h.departments.Update(existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update department")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *DepartmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.departments.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete department")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}
