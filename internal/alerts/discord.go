package alerts

import (
	"fmt"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// SendDiscordNotification sends a rich embed card to a Discord webhook.
func SendDiscordNotification(webhookURL, baseURL, reason string, issue *storage.Issue) error {
	issueURL := fmt.Sprintf("%s/#/issues/%d", baseURL, issue.ID)
	color := 0xe11d48 // Red for new issue
	statusText := "New Issue Detected"
	if reason == "REGRESSION" {
		color = 0xf59e0b // Amber for regression
		statusText = "Issue Regression Detected"
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       fmt.Sprintf("[%s] %s", statusText, truncateStr(issue.Title, 100)),
				"url":         issueURL,
				"color":       color,
				"description": fmt.Sprintf("**Culprit:** `%s`", truncateStr(issue.Culprit, 120)),
				"fields": []map[string]interface{}{
					{
						"name":   "Type",
						"value":  defaultStr(issue.Type, "Error"),
						"inline": true,
					},
					{
						"name":   "Release",
						"value":  defaultStr(issue.LastRelease, "unspecified"),
						"inline": true,
					},
					{
						"name":   "Occurrences",
						"value":  fmt.Sprintf("%d", issue.EventCount),
						"inline": true,
					},
				},
				"footer": map[string]string{
					"text": "ErrPulse Crash Monitor",
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	return postJSON(webhookURL, payload)
}
