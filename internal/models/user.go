package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"orgId"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Create(user *User) error {
	return s.db.QueryRow(
		`INSERT INTO users (org_id, email, name, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at`,
		user.OrgID, user.Email, user.Name, user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (s *UserStore) GetByID(id string) (*User, error) {
	user := &User{}
	err := s.db.QueryRow(
		`SELECT id, org_id, email, name, role, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.OrgID, &user.Email, &user.Name, &user.Role,
		&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserStore) ListByOrg(orgID string) ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, org_id, email, name, role, created_at, updated_at
		 FROM users WHERE org_id = $1 ORDER BY created_at DESC`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.OrgID, &user.Email, &user.Name, &user.Role,
			&user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}
