package alerts

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

func TestAlertDispatch(t *testing.T) {
	receivedPayload := make(chan map[string]interface{}, 1)

	// Mock server simulating Slack / Discord / Webhook endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		receivedPayload <- payload
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	issue := &storage.Issue{
		ID:          42,
		ProjectID:   1,
		Title:       "ReferenceError: x is not defined",
		Culprit:     "render(app.tsx:12)",
		Type:        "ReferenceError",
		Status:      storage.StatusUnresolved,
		FirstSeenAt: time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
		EventCount:  1,
	}

	// 1. Test Generic Webhook
	err := SendGenericWebhook(server.URL, "http://localhost:9988", "NEW_ISSUE", issue)
	if err != nil {
		t.Fatalf("Failed sending generic webhook: %v", err)
	}

	select {
	case p := <-receivedPayload:
		if p["reason"] != "NEW_ISSUE" {
			t.Errorf("Expected reason NEW_ISSUE, got %v", p["reason"])
		}
		if p["title"] != issue.Title {
			t.Errorf("Expected title %s, got %v", issue.Title, p["title"])
		}
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for webhook payload")
	}

	// 2. Test Slack Block Kit
	err = SendSlackNotification(server.URL, "http://localhost:9988", "REGRESSION", issue)
	if err != nil {
		t.Fatalf("Failed sending Slack notification: %v", err)
	}

	select {
	case p := <-receivedPayload:
		if _, ok := p["blocks"]; !ok {
			t.Errorf("Expected 'blocks' in Slack payload")
		}
	case <-time.After(time.Second):
		t.Fatal("Timed out waiting for Slack payload")
	}
}
