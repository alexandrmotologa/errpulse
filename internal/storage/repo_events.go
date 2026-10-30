package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateEvent writes an individual crash occurrence to the events table.
func (db *DB) CreateEvent(e *Event) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = e.CreatedAt
	}

	_, err := db.Exec(`
		INSERT INTO events (
			id, issue_id, project_id, timestamp, environment, release,
			platform, message, user_json, tags_json, extra_json,
			breadcrumbs_json, stacktrace_json, raw_payload, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.IssueID, e.ProjectID, e.Timestamp.Format(time.RFC3339), e.Environment, e.Release,
		e.Platform, e.Message, e.UserJSON, e.TagsJSON, e.ExtraJSON,
		e.BreadcrumbsJSON, e.StacktraceJSON, e.RawPayload, e.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("failed creating event %s: %w", e.ID, err)
	}
	return nil
}

// GetEvent fetches an event by its Sentry event ID.
func (db *DB) GetEvent(id string) (*Event, error) {
	var e Event
	var timestampStr, createdAtStr string
	err := db.QueryRow(`
		SELECT id, issue_id, project_id, timestamp, COALESCE(environment, ''), COALESCE(release, ''),
		       COALESCE(platform, ''), COALESCE(message, ''), COALESCE(user_json, ''),
		       COALESCE(tags_json, ''), COALESCE(extra_json, ''), COALESCE(breadcrumbs_json, ''),
		       COALESCE(stacktrace_json, ''), COALESCE(raw_payload, ''), created_at
		FROM events
		WHERE id = ?
	`, id).Scan(
		&e.ID, &e.IssueID, &e.ProjectID, &timestampStr, &e.Environment, &e.Release,
		&e.Platform, &e.Message, &e.UserJSON, &e.TagsJSON, &e.ExtraJSON,
		&e.BreadcrumbsJSON, &e.StacktraceJSON, &e.RawPayload, &createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching event %s: %w", id, err)
	}
	e.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &e, nil
}

// GetLatestEventForIssue retrieves the most recent event attached to a specific issue.
func (db *DB) GetLatestEventForIssue(issueID int64) (*Event, error) {
	return db.GetEventByOccurrence(issueID, 0)
}

// GetEventByOccurrence retrieves a specific zero-based occurrence (0 = latest, 1 = second latest, etc.).
func (db *DB) GetEventByOccurrence(issueID int64, offset int) (*Event, error) {
	if offset < 0 {
		offset = 0
	}

	var e Event
	var timestampStr, createdAtStr string
	err := db.QueryRow(`
		SELECT id, issue_id, project_id, timestamp, COALESCE(environment, ''), COALESCE(release, ''),
		       COALESCE(platform, ''), COALESCE(message, ''), COALESCE(user_json, ''),
		       COALESCE(tags_json, ''), COALESCE(extra_json, ''), COALESCE(breadcrumbs_json, ''),
		       COALESCE(stacktrace_json, ''), COALESCE(raw_payload, ''), created_at
		FROM events
		WHERE issue_id = ?
		ORDER BY timestamp DESC
		LIMIT 1 OFFSET ?
	`, issueID, offset).Scan(
		&e.ID, &e.IssueID, &e.ProjectID, &timestampStr, &e.Environment, &e.Release,
		&e.Platform, &e.Message, &e.UserJSON, &e.TagsJSON, &e.ExtraJSON,
		&e.BreadcrumbsJSON, &e.StacktraceJSON, &e.RawPayload, &createdAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching event occurrence %d for issue %d: %w", offset, issueID, err)
	}
	e.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	return &e, nil
}

// ListEventsForIssue queries paginated events for a specific issue.
func (db *DB) ListEventsForIssue(issueID int64, limit, offset int) ([]Event, int64, error) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	var totalCount int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE issue_id = ?`, issueID).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, issue_id, project_id, timestamp, COALESCE(environment, ''), COALESCE(release, ''),
		       COALESCE(platform, ''), COALESCE(message, ''), COALESCE(user_json, ''),
		       COALESCE(tags_json, ''), COALESCE(extra_json, ''), COALESCE(breadcrumbs_json, ''),
		       COALESCE(stacktrace_json, ''), created_at
		FROM events
		WHERE issue_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`, issueID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var timestampStr, createdAtStr string
		if err := rows.Scan(
			&e.ID, &e.IssueID, &e.ProjectID, &timestampStr, &e.Environment, &e.Release,
			&e.Platform, &e.Message, &e.UserJSON, &e.TagsJSON, &e.ExtraJSON,
			&e.BreadcrumbsJSON, &e.StacktraceJSON, &createdAtStr,
		); err != nil {
			return nil, 0, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, timestampStr)
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		events = append(events, e)
	}

	return events, totalCount, rows.Err()
}

// GetStatsSummary computes high-level dashboard metrics for the current project or all projects.
func (db *DB) GetStatsSummary(projectID int64) (*StatsSummary, error) {
	summary := &StatsSummary{}

	whereClause := ""
	var args []interface{}
	if projectID > 0 {
		whereClause = "WHERE project_id = ?"
		args = append(args, projectID)
	}

	// Total events
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM events %s", whereClause)
	_ = db.QueryRow(countQuery, args...).Scan(&summary.TotalEvents)

	// Unresolved issues
	issueWhere := "WHERE status = 'unresolved'"
	if projectID > 0 {
		issueWhere += " AND project_id = ?"
	}
	unresolvedQuery := fmt.Sprintf("SELECT COUNT(*) FROM issues %s", issueWhere)
	_ = db.QueryRow(unresolvedQuery, args...).Scan(&summary.UnresolvedIssues)

	// Events in last 24h
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	recentWhere := "WHERE timestamp >= ?"
	recentArgs := []interface{}{cutoff}
	if projectID > 0 {
		recentWhere += " AND project_id = ?"
		recentArgs = append(recentArgs, projectID)
	}
	recentQuery := fmt.Sprintf("SELECT COUNT(*) FROM events %s", recentWhere)
	_ = db.QueryRow(recentQuery, recentArgs...).Scan(&summary.Events24h)

	// Total projects
	_ = db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&summary.TotalProjects)

	return summary, nil
}
