package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListProjects returns all configured projects.
func (db *DB) ListProjects() ([]Project, error) {
	rows, err := db.Query(`
		SELECT id, name, slug, public_key, COALESCE(secret_key, ''), created_at
		FROM projects
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed listing projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var createdAt string
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.PublicKey, &p.SecretKey, &createdAt); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetProjectByID fetches a project by primary key ID.
func (db *DB) GetProjectByID(id int64) (*Project, error) {
	var p Project
	var createdAt string
	err := db.QueryRow(`
		SELECT id, name, slug, public_key, COALESCE(secret_key, ''), created_at
		FROM projects
		WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.Slug, &p.PublicKey, &p.SecretKey, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching project %d: %w", id, err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &p, nil
}

// GetProjectByKey fetches a project matching the supplied public DSN key.
func (db *DB) GetProjectByKey(publicKey string) (*Project, error) {
	var p Project
	var createdAt string
	err := db.QueryRow(`
		SELECT id, name, slug, public_key, COALESCE(secret_key, ''), created_at
		FROM projects
		WHERE public_key = ?
	`, publicKey).Scan(&p.ID, &p.Name, &p.Slug, &p.PublicKey, &p.SecretKey, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching project by key: %w", err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &p, nil
}

// CreateProject inserts a new project with an auto-generated public key if empty.
func (db *DB) CreateProject(name, slug, publicKey string) (*Project, error) {
	if publicKey == "" {
		publicKey = uuid.New().String()
	}
	res, err := db.Exec(`
		INSERT INTO projects (name, slug, public_key, secret_key)
		VALUES (?, ?, ?, ?)
	`, name, slug, publicKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed creating project: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetProjectByID(id)
}
