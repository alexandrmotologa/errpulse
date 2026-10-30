package alerts

import (
	"fmt"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// SendGenericWebhook dispatches a standard JSON payload for custom automation.
func SendGenericWebhook(targetURL, baseURL, reason string, issue *storage.Issue) error {
	payload := map[string]interface{}{
		"event":       "issue_alert",
		"reason":      reason,
		"issue_id":    issue.ID,
		"project_id":  issue.ProjectID,
		"title":       issue.Title,
		"culprit":     issue.Culprit,
		"type":        issue.Type,
		"status":      issue.Status,
		"release":     issue.LastRelease,
		"event_count": issue.EventCount,
		"user_count":  issue.UserCount,
		"first_seen":  issue.FirstSeenAt.Format(time.RFC3339),
		"last_seen":   issue.LastSeenAt.Format(time.RFC3339),
		"issue_url":   fmt.Sprintf("%s/#/issues/%d", baseURL, issue.ID),
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	return postJSON(targetURL, payload)
}
