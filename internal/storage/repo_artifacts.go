package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SaveArtifact saves or updates a release artifact file (.map or source code).
func (db *DB) SaveArtifact(projectID int64, release, filename, contentType string, data []byte) error {
	_, err := db.Exec(`
		INSERT INTO release_artifacts (project_id, release, filename, content_type, data)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id, release, filename)
		DO UPDATE SET content_type = excluded.content_type, data = excluded.data, created_at = CURRENT_TIMESTAMP
	`, projectID, release, filename, contentType, data)
	if err != nil {
		return fmt.Errorf("failed saving artifact %s for release %s: %w", filename, release, err)
	}
	return nil
}

// GetArtifact retrieves a specific artifact by project, release, and filename.
func (db *DB) GetArtifact(projectID int64, release, filename string) (*ReleaseArtifact, error) {
	var a ReleaseArtifact
	var createdAtStr string
	err := db.QueryRow(`
		SELECT id, project_id, release, filename, content_type, data, created_at
		FROM release_artifacts
		WHERE project_id = ? AND release = ? AND filename = ?
	`, projectID, release, filename).Scan(&a.ID, &a.ProjectID, &a.Release, &a.Filename, &a.ContentType, &a.Data, &createdAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching artifact %s: %w", filename, err)
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &a, nil
}

// ListArtifacts returns all stored artifacts for a given release.
func (db *DB) ListArtifacts(projectID int64, release string) ([]ReleaseArtifact, error) {
	rows, err := db.Query(`
		SELECT id, project_id, release, filename, content_type, LENGTH(data), created_at
		FROM release_artifacts
		WHERE project_id = ? AND release = ?
		ORDER BY filename ASC
	`, projectID, release)
	if err != nil {
		return nil, fmt.Errorf("failed listing artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []ReleaseArtifact
	for rows.Next() {
		var a ReleaseArtifact
		var size int
		var createdAtStr string
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Release, &a.Filename, &a.ContentType, &size, &createdAtStr); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

// ListReleases returns all distinct release tags that have artifacts uploaded or have recorded issues.
func (db *DB) ListReleases(projectID int64) ([]string, error) {
	rows, err := db.Query(`
		SELECT DISTINCT release FROM (
			SELECT release FROM release_artifacts WHERE project_id = ? AND release != ''
			UNION
			SELECT DISTINCT release FROM events WHERE project_id = ? AND release != ''
		) ORDER BY release DESC
	`, projectID, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed querying releases: %w", err)
	}
	defer rows.Close()

	var releases []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err == nil {
			releases = append(releases, rel)
		}
	}
	return releases, nil
}

// DeleteArtifact removes an artifact from the release store.
func (db *DB) DeleteArtifact(id int64) error {
	_, err := db.Exec(`DELETE FROM release_artifacts WHERE id = ?`, id)
	return err
}
