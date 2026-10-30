package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ListAlertConfigs returns active alert destinations for a project.
func (db *DB) ListAlertConfigs(projectID int64) ([]AlertConfig, error) {
	rows, err := db.Query(`
		SELECT id, project_id, channel, target_url, enabled, notify_on_new, notify_on_regression, created_at
		FROM alert_configs
		WHERE project_id = ?
		ORDER BY id ASC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed listing alert configs: %w", err)
	}
	defer rows.Close()

	var configs []AlertConfig
	for rows.Next() {
		var cfg AlertConfig
		var createdAtStr string
		if err := rows.Scan(
			&cfg.ID, &cfg.ProjectID, &cfg.Channel, &cfg.TargetURL,
			&cfg.Enabled, &cfg.NotifyOnNew, &cfg.NotifyOnRegression, &createdAtStr,
		); err != nil {
			return nil, err
		}
		cfg.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

// SaveAlertConfig creates or updates an alert destination.
func (db *DB) SaveAlertConfig(cfg *AlertConfig) (*AlertConfig, error) {
	if cfg.ID > 0 {
		_, err := db.Exec(`
			UPDATE alert_configs
			SET channel = ?, target_url = ?, enabled = ?, notify_on_new = ?, notify_on_regression = ?
			WHERE id = ?
		`, cfg.Channel, cfg.TargetURL, cfg.Enabled, cfg.NotifyOnNew, cfg.NotifyOnRegression, cfg.ID)
		if err != nil {
			return nil, fmt.Errorf("failed updating alert config: %w", err)
		}
		return cfg, nil
	}

	res, err := db.Exec(`
		INSERT INTO alert_configs (project_id, channel, target_url, enabled, notify_on_new, notify_on_regression)
		VALUES (?, ?, ?, ?, ?, ?)
	`, cfg.ProjectID, cfg.Channel, cfg.TargetURL, cfg.Enabled, cfg.NotifyOnNew, cfg.NotifyOnRegression)
	if err != nil {
		return nil, fmt.Errorf("failed creating alert config: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	cfg.ID = id
	return cfg, nil
}

// DeleteAlertConfig removes an alert destination.
func (db *DB) DeleteAlertConfig(id int64) error {
	_, err := db.Exec(`DELETE FROM alert_configs WHERE id = ?`, id)
	return err
}

// LogAlert records notification dispatch status.
func (db *DB) LogAlert(projectID, issueID int64, channel, status, response string) error {
	_, err := db.Exec(`
		INSERT INTO alert_logs (project_id, issue_id, channel, status, response)
		VALUES (?, ?, ?, ?, ?)
	`, projectID, issueID, channel, status, response)
	return err
}

// GetAlertConfigByID fetches an alert configuration by ID.
func (db *DB) GetAlertConfigByID(id int64) (*AlertConfig, error) {
	var cfg AlertConfig
	var createdAtStr string
	err := db.QueryRow(`
		SELECT id, project_id, channel, target_url, enabled, notify_on_new, notify_on_regression, created_at
		FROM alert_configs
		WHERE id = ?
	`, id).Scan(
		&cfg.ID, &cfg.ProjectID, &cfg.Channel, &cfg.TargetURL,
		&cfg.Enabled, &cfg.NotifyOnNew, &cfg.NotifyOnRegression, &createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	cfg.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &cfg, nil
}
