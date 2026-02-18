package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Organization struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Domain              string          `json:"domain,omitempty"`
	WorkspaceType       string          `json:"workspaceType,omitempty"`
	WorkspaceAdminEmail string          `json:"workspaceAdminEmail,omitempty"`
	Settings            json.RawMessage `json:"settings,omitempty"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

type OrganizationStore struct {
	db *sql.DB
}

func NewOrganizationStore(db *sql.DB) *OrganizationStore {
	return &OrganizationStore{db: db}
}

func (s *OrganizationStore) Create(org *Organization) error {
	return s.db.QueryRow(
		`INSERT INTO organizations (name, domain, workspace_type, workspace_admin_email, settings)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		org.Name, nullString(org.Domain), nullString(org.WorkspaceType),
		nullString(org.WorkspaceAdminEmail), defaultJSON(org.Settings),
	).Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)
}

func (s *OrganizationStore) GetByID(id string) (*Organization, error) {
	org := &Organization{}
	err := s.db.QueryRow(
		`SELECT id, name, COALESCE(domain, ''), COALESCE(workspace_type, ''),
		        COALESCE(workspace_admin_email, ''), COALESCE(settings, '{}'),
		        created_at, updated_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&org.ID, &org.Name, &org.Domain, &org.WorkspaceType,
		&org.WorkspaceAdminEmail, &org.Settings, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func (s *OrganizationStore) List() ([]Organization, error) {
	rows, err := s.db.Query(
		`SELECT id, name, COALESCE(domain, ''), COALESCE(workspace_type, ''),
		        COALESCE(workspace_admin_email, ''), COALESCE(settings, '{}'),
		        created_at, updated_at
		 FROM organizations ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []Organization
	for rows.Next() {
		var org Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Domain, &org.WorkspaceType,
			&org.WorkspaceAdminEmail, &org.Settings, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func defaultJSON(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return json.RawMessage(`{}`)
	}
	return data
}
