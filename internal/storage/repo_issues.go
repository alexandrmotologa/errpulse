package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// UpsertIssue records an incoming crash occurrence, creating a new issue or incrementing stats.
func (db *DB) UpsertIssue(
	projectID int64,
	fingerprint, title, culprit, exceptionType, release, userIdentifier string,
	timestamp time.Time,
) (*Issue, bool, bool, error) {
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	var existing Issue
	var firstSeenStr, lastSeenStr string
	err := db.QueryRow(`
		SELECT id, project_id, fingerprint, title, culprit, type, status,
		       first_seen_at, last_seen_at, event_count, user_count, COALESCE(last_release, '')
		FROM issues
		WHERE project_id = ? AND fingerprint = ?
	`, projectID, fingerprint).Scan(
		&existing.ID, &existing.ProjectID, &existing.Fingerprint, &existing.Title,
		&existing.Culprit, &existing.Type, &existing.Status,
		&firstSeenStr, &lastSeenStr, &existing.EventCount, &existing.UserCount, &existing.LastRelease,
	)

	isNew := false
	isRegression := false

	if errors.Is(err, sql.ErrNoRows) {
		// New issue
		isNew = true
		res, insertErr := db.Exec(`
			INSERT INTO issues (
				project_id, fingerprint, title, culprit, type, status,
				first_seen_at, last_seen_at, event_count, user_count, last_release
			) VALUES (?, ?, ?, ?, ?, 'unresolved', ?, ?, 1, 1, ?)
		`, projectID, fingerprint, title, culprit, exceptionType, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339), release)
		if insertErr != nil {
			return nil, false, false, fmt.Errorf("failed creating issue: %w", insertErr)
		}

		issueID, _ := res.LastInsertId()
		if userIdentifier != "" {
			_, _ = db.Exec(`INSERT OR IGNORE INTO issue_users (issue_id, user_identifier) VALUES (?, ?)`, issueID, userIdentifier)
		}

		_ = db.RecordHourlyStat(projectID, issueID, timestamp)

		issue, getErr := db.GetIssue(issueID)
		return issue, isNew, isRegression, getErr
	} else if err != nil {
		return nil, false, false, fmt.Errorf("failed querying issue by fingerprint: %w", err)
	}

	// Existing issue
	newStatus := existing.Status
	if existing.Status == StatusResolved {
		isRegression = true
		newStatus = StatusUnresolved
	}

	if userIdentifier != "" {
		_, _ = db.Exec(`INSERT OR IGNORE INTO issue_users (issue_id, user_identifier) VALUES (?, ?)`, existing.ID, userIdentifier)
	}

	var userCount int64 = 1
	_ = db.QueryRow(`SELECT COUNT(*) FROM issue_users WHERE issue_id = ?`, existing.ID).Scan(&userCount)
	if userCount == 0 {
		userCount = 1
	}

	_, updateErr := db.Exec(`
		UPDATE issues
		SET event_count = event_count + 1,
		    user_count = ?,
		    last_seen_at = ?,
		    status = ?,
		    last_release = CASE WHEN ? != '' THEN ? ELSE last_release END
		WHERE id = ?
	`, userCount, timestamp.Format(time.RFC3339), newStatus, release, release, existing.ID)
	if updateErr != nil {
		return nil, false, false, fmt.Errorf("failed updating issue %d: %w", existing.ID, updateErr)
	}

	_ = db.RecordHourlyStat(projectID, existing.ID, timestamp)

	issue, getErr := db.GetIssue(existing.ID)
	return issue, isNew, isRegression, getErr
}

// GetIssue returns a single issue with calculated sparkline history.
func (db *DB) GetIssue(id int64) (*Issue, error) {
	var issue Issue
	var firstSeenStr, lastSeenStr string
	err := db.QueryRow(`
		SELECT id, project_id, fingerprint, title, culprit, type, status,
		       first_seen_at, last_seen_at, event_count, user_count, COALESCE(last_release, '')
		FROM issues
		WHERE id = ?
	`, id).Scan(
		&issue.ID, &issue.ProjectID, &issue.Fingerprint, &issue.Title,
		&issue.Culprit, &issue.Type, &issue.Status,
		&firstSeenStr, &lastSeenStr, &issue.EventCount, &issue.UserCount, &issue.LastRelease,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed fetching issue %d: %w", id, err)
	}

	issue.FirstSeenAt, _ = time.Parse(time.RFC3339, firstSeenStr)
	issue.LastSeenAt, _ = time.Parse(time.RFC3339, lastSeenStr)

	sparkline, _ := db.GetIssueSparkline(issue.ProjectID, issue.ID, 24)
	issue.Sparkline = sparkline

	return &issue, nil
}

// ListIssues queries issues with syntax DSL filtering and pagination.
func (db *DB) ListIssues(
	projectID int64,
	status IssueStatus,
	environment string,
	search string,
	limit, offset int,
) ([]Issue, int64, error) {
	if limit <= 0 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}

	if projectID > 0 {
		conditions = append(conditions, "issues.project_id = ?")
		args = append(args, projectID)
	}

	// Parse search tokens if present: is:unresolved, env:prod, release:v1, type:TypeError
	var freeTextWords []string
	if search != "" {
		tokens := strings.Fields(search)
		for _, token := range tokens {
			lower := strings.ToLower(token)
			if strings.HasPrefix(lower, "is:") {
				val := strings.TrimPrefix(lower, "is:")
				if val == "unresolved" || val == "resolved" || val == "ignored" {
					status = IssueStatus(val)
				}
			} else if strings.HasPrefix(lower, "env:") {
				environment = strings.TrimPrefix(token, "env:")
			} else if strings.HasPrefix(lower, "release:") {
				relVal := strings.TrimPrefix(token, "release:")
				conditions = append(conditions, "issues.last_release LIKE ?")
				args = append(args, "%"+relVal+"%")
			} else if strings.HasPrefix(lower, "type:") {
				typeVal := strings.TrimPrefix(token, "type:")
				conditions = append(conditions, "issues.type LIKE ?")
				args = append(args, "%"+typeVal+"%")
			} else {
				freeTextWords = append(freeTextWords, token)
			}
		}
	}

	if status != "" && status != "all" {
		conditions = append(conditions, "issues.status = ?")
		args = append(args, string(status))
	}

	joinClause := ""
	if environment != "" && environment != "all" {
		joinClause = "JOIN events ON issues.id = events.issue_id"
		conditions = append(conditions, "events.environment = ?")
		args = append(args, environment)
	}

	if len(freeTextWords) > 0 {
		freeText := "%" + strings.Join(freeTextWords, " ") + "%"
		conditions = append(conditions, "(issues.title LIKE ? OR issues.culprit LIKE ? OR issues.fingerprint LIKE ?)")
		args = append(args, freeText, freeText, freeText)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var totalCount int64
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT issues.id) FROM issues %s %s", joinClause, whereClause)
	if err := db.QueryRow(countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed counting issues: %w", err)
	}

	selectQuery := fmt.Sprintf(`
		SELECT DISTINCT issues.id, issues.project_id, issues.fingerprint, issues.title,
		       issues.culprit, issues.type, issues.status,
		       issues.first_seen_at, issues.last_seen_at, issues.event_count, issues.user_count, COALESCE(issues.last_release, '')
		FROM issues
		%s
		%s
		ORDER BY issues.last_seen_at DESC
		LIMIT ? OFFSET ?
	`, joinClause, whereClause)

	queryArgs := append(args, limit, offset)
	rows, err := db.Query(selectQuery, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed querying issues: %w", err)
	}
	defer rows.Close()

	var issues []Issue
	for rows.Next() {
		var item Issue
		var firstSeenStr, lastSeenStr string
		if err := rows.Scan(
			&item.ID, &item.ProjectID, &item.Fingerprint, &item.Title,
			&item.Culprit, &item.Type, &item.Status,
			&firstSeenStr, &lastSeenStr, &item.EventCount, &item.UserCount, &item.LastRelease,
		); err != nil {
			return nil, 0, err
		}
		item.FirstSeenAt, _ = time.Parse(time.RFC3339, firstSeenStr)
		item.LastSeenAt, _ = time.Parse(time.RFC3339, lastSeenStr)

		issues = append(issues, item)
	}
	rows.Close()

	// Populate sparklines after closing the issues rows cursor
	for i := range issues {
		spark, _ := db.GetIssueSparkline(issues[i].ProjectID, issues[i].ID, 24)
		issues[i].Sparkline = spark
	}

	return issues, totalCount, rows.Err()
}

// UpdateIssueStatus changes issue triage state (unresolved, resolved, ignored).
func (db *DB) UpdateIssueStatus(id int64, status IssueStatus) error {
	_, err := db.Exec(`UPDATE issues SET status = ? WHERE id = ?`, string(status), id)
	return err
}

// DeleteIssue deletes an issue and cascades to all child events.
func (db *DB) DeleteIssue(id int64) error {
	_, err := db.Exec(`DELETE FROM issues WHERE id = ?`, id)
	return err
}

// RecordHourlyStat increments the hourly activity bucket counter.
func (db *DB) RecordHourlyStat(projectID, issueID int64, t time.Time) error {
	hourBucket := t.UTC().Format("2006-01-02 15:00")
	_, err := db.Exec(`
		INSERT INTO hourly_stats (project_id, issue_id, hour_bucket, count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(project_id, issue_id, hour_bucket)
		DO UPDATE SET count = count + 1
	`, projectID, issueID, hourBucket)
	return err
}

// GetIssueSparkline retrieves the event count for each hour over the last N hours.
func (db *DB) GetIssueSparkline(projectID, issueID int64, hours int) ([]int64, error) {
	if hours <= 0 {
		hours = 24
	}

	sparkline := make([]int64, hours)
	now := time.Now().UTC()

	// Pre-populate time bucket map
	bucketMap := make(map[string]int) // hourBucket string -> index in sparkline
	for i := 0; i < hours; i++ {
		t := now.Add(-time.Duration(hours-1-i) * time.Hour)
		bucketStr := t.Format("2006-01-02 15:00")
		bucketMap[bucketStr] = i
	}

	oldestBucket := now.Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:00")

	rows, err := db.Query(`
		SELECT hour_bucket, count
		FROM hourly_stats
		WHERE project_id = ? AND issue_id = ? AND hour_bucket >= ?
	`, projectID, issueID, oldestBucket)
	if err != nil {
		return sparkline, err
	}
	defer rows.Close()

	for rows.Next() {
		var bucket string
		var count int64
		if err := rows.Scan(&bucket, &count); err == nil {
			if idx, ok := bucketMap[bucket]; ok && idx < len(sparkline) {
				sparkline[idx] = count
			}
		}
	}

	return sparkline, nil
}
