package alerts

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// SendTelegramNotification posts an alert to a Telegram channel or group.
func SendTelegramNotification(targetURL, baseURL, reason string, issue *storage.Issue) error {
	issueURL := fmt.Sprintf("%s/#/issues/%d", baseURL, issue.ID)
	statusText := "<b>[NEW ISSUE]</b>"
	if reason == "REGRESSION" {
		statusText = "<b>[REGRESSION]</b>"
	}

	text := fmt.Sprintf(
		"%s <b>%s</b>\n\n<b>Culprit:</b> <code>%s</code>\n<b>Release:</b> <code>%s</code>\n\n<a href=\"%s\">Open in ErrPulse Studio</a>",
		statusText,
		html.EscapeString(truncateStr(issue.Title, 120)),
		html.EscapeString(truncateStr(issue.Culprit, 120)),
		html.EscapeString(defaultStr(issue.LastRelease, "none")),
		issueURL,
	)

	// If targetURL contains chat_id query param, use standard Telegram Bot API
	u, err := url.Parse(targetURL)
	if err == nil && strings.Contains(u.Host, "telegram.org") {
		chatID := u.Query().Get("chat_id")
		payload := map[string]interface{}{
			"chat_id":    chatID,
			"text":       text,
			"parse_mode": "HTML",
		}
		// Strip chat_id from POST URL if present
		q := u.Query()
		q.Del("chat_id")
		u.RawQuery = q.Encode()
		return postJSON(u.String(), payload)
	}

	// Fallback to standard JSON payload
	payload := map[string]interface{}{
		"text":       text,
		"parse_mode": "HTML",
		"issue_id":   issue.ID,
		"title":      issue.Title,
	}
	return postJSON(targetURL, payload)
}
