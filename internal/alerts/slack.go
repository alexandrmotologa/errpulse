package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// SendSlackNotification builds and dispatches a Slack Block Kit alert.
func SendSlackNotification(webhookURL, baseURL, reason string, issue *storage.Issue) error {
	issueURL := fmt.Sprintf("%s/#/issues/%d", baseURL, issue.ID)
	badge := "[NEW ISSUE]"
	if reason == "REGRESSION" {
		badge = "[REGRESSION]"
	}

	payload := map[string]interface{}{
		"text": fmt.Sprintf("%s %s", badge, issue.Title),
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("%s %s", badge, truncateStr(issue.Title, 140)),
				},
			},
			{
				"type": "section",
				"fields": []map[string]string{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Culprit:*\n`%s`", truncateStr(issue.Culprit, 60)),
					},
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*Release:*\n`%s`", defaultStr(issue.LastRelease, "none")),
					},
				},
			},
			{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type": "button",
						"text": map[string]string{
							"type": "plain_text",
							"text": "View in ErrPulse",
						},
						"url":   issueURL,
						"style": "primary",
					},
				},
			},
		},
	}

	return postJSON(webhookURL, payload)
}

func postJSON(url string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook responded with HTTP %d", resp.StatusCode)
	}
	return nil
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
