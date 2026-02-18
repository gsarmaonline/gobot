package models

import (
	"database/sql"
	"time"
)

type Department struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"orgId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DepartmentStore struct {
	db *sql.DB
}

func NewDepartmentStore(db *sql.DB) *DepartmentStore {
	return &DepartmentStore{db: db}
}

func (s *DepartmentStore) Create(dept *Department) error {
	return s.db.QueryRow(
		`INSERT INTO departments (org_id, name, description)
		 VALUES ($1, $2, $3)
		 RETURNING id, created_at, updated_at`,
		dept.OrgID, dept.Name, nullString(dept.Description),
	).Scan(&dept.ID, &dept.CreatedAt, &dept.UpdatedAt)
}

func (s *DepartmentStore) GetByID(id string) (*Department, error) {
	dept := &Department{}
	err := s.db.QueryRow(
		`SELECT id, org_id, name, COALESCE(description, ''), created_at, updated_at
		 FROM departments WHERE id = $1`, id,
	).Scan(&dept.ID, &dept.OrgID, &dept.Name, &dept.Description,
		&dept.CreatedAt, &dept.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *DepartmentStore) ListByOrg(orgID string) ([]Department, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, name, COALESCE(description, ''), created_at, updated_at
		 FROM departments WHERE org_id = $1 ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var depts []Department
	for rows.Next() {
		var dept Department
		if err := rows.Scan(&dept.ID, &dept.OrgID, &dept.Name, &dept.Description,
			&dept.CreatedAt, &dept.UpdatedAt); err != nil {
			return nil, err
		}
		depts = append(depts, dept)
	}
	return depts, rows.Err()
}

func (s *DepartmentStore) Update(dept *Department) error {
	_, err := s.db.Exec(
		`UPDATE departments SET name = $2, description = $3, updated_at = NOW()
		 WHERE id = $1`,
		dept.ID, dept.Name, nullString(dept.Description),
	)
	return err
}

func (s *DepartmentStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM departments WHERE id = $1`, id)
	return err
}
