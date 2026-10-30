package storage

import (
	"time"
)

// Project represents an application monitored by ErrPulse.
type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	PublicKey string    `json:"public_key"`
	SecretKey string    `json:"secret_key,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// IssueStatus represents the triage status of an issue.
type IssueStatus string

const (
	StatusUnresolved IssueStatus = "unresolved"
	StatusResolved   IssueStatus = "resolved"
	StatusIgnored    IssueStatus = "ignored"
)

// Issue represents a clustered group of crash events sharing a fingerprint.
type Issue struct {
	ID          int64       `json:"id"`
	ProjectID   int64       `json:"project_id"`
	Fingerprint string      `json:"fingerprint"`
	Title       string      `json:"title"`
	Culprit     string      `json:"culprit"`
	Type        string      `json:"type"`
	Status      IssueStatus `json:"status"`
	FirstSeenAt time.Time   `json:"first_seen_at"`
	LastSeenAt  time.Time   `json:"last_seen_at"`
	EventCount  int64       `json:"event_count"`
	UserCount   int64       `json:"user_count"`
	LastRelease string      `json:"last_release"`
	Sparkline   []int64     `json:"sparkline,omitempty"`
}

// Event represents an individual crash instance received via Sentry ingestion.
type Event struct {
	ID              string    `json:"id"`
	IssueID         int64     `json:"issue_id"`
	ProjectID       int64     `json:"project_id"`
	Timestamp       time.Time `json:"timestamp"`
	Environment     string    `json:"environment"`
	Release         string    `json:"release"`
	Platform        string    `json:"platform"`
	Message         string    `json:"message"`
	UserJSON        string    `json:"user_json,omitempty"`
	TagsJSON        string    `json:"tags_json,omitempty"`
	ExtraJSON       string    `json:"extra_json,omitempty"`
	BreadcrumbsJSON string    `json:"breadcrumbs_json,omitempty"`
	StacktraceJSON  string    `json:"stacktrace_json,omitempty"`
	RawPayload      string    `json:"raw_payload,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// ReleaseArtifact stores uploaded source maps and bundles for de-minification.
type ReleaseArtifact struct {
	ID          int64     `json:"id"`
	ProjectID   int64     `json:"project_id"`
	Release     string    `json:"release"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Data        []byte    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

// AlertConfig defines notification endpoints for Slack, Discord, Telegram, or webhooks.
type AlertConfig struct {
	ID                 int64     `json:"id"`
	ProjectID          int64     `json:"project_id"`
	Channel            string    `json:"channel"` // slack, discord, telegram, webhook
	TargetURL          string    `json:"target_url"`
	Enabled            bool      `json:"enabled"`
	NotifyOnNew        bool      `json:"notify_on_new"`
	NotifyOnRegression bool      `json:"notify_on_regression"`
	CreatedAt          time.Time `json:"created_at"`
}

// AlertLog stores sent notification delivery records.
type AlertLog struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"project_id"`
	IssueID   int64     `json:"issue_id"`
	Channel   string    `json:"channel"`
	SentAt    time.Time `json:"sent_at"`
	Status    string    `json:"status"` // success, error
	Response  string    `json:"response,omitempty"`
}

// StatsSummary gives high-level overview metrics for the dashboard.
type StatsSummary struct {
	TotalEvents      int64 `json:"total_events"`
	UnresolvedIssues int64 `json:"unresolved_issues"`
	Events24h        int64 `json:"events_24h"`
	TotalProjects    int64 `json:"total_projects"`
}
