package ingest

import (
	"testing"

	"github.com/alexandrmotologa/errpulse/internal/models"
)

func TestSanitizeEvent(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	card := "4111 2222 3333 4444"

	event := &models.SentryEvent{
		Message: "Authentication failed with token " + jwt,
		Tags: map[string]string{
			"api_key":   "sk_live_secret123",
			"user_type": "admin",
		},
		Extra: map[string]interface{}{
			"auth_header": "Bearer " + jwt,
			"password":    "supersecret123",
			"nested": map[string]interface{}{
				"credit_card": card,
				"safe_field":  "hello world",
			},
		},
		Request: &models.SentryRequest{
			Headers: map[string]string{
				"Authorization": "Bearer " + jwt,
				"User-Agent":    "Mozilla/5.0",
			},
			Cookies: "session=xyz123",
		},
	}

	SanitizeEvent(event)

	if event.Tags["api_key"] != "[REDACTED]" {
		t.Errorf("Expected api_key tag to be [REDACTED], got %s", event.Tags["api_key"])
	}
	if event.Tags["user_type"] != "admin" {
		t.Errorf("Expected user_type to be preserved, got %s", event.Tags["user_type"])
	}

	extra := event.Extra
	if extra["password"] != "[REDACTED]" {
		t.Errorf("Expected password in extra to be [REDACTED], got %v", extra["password"])
	}

	nested := extra["nested"].(map[string]interface{})
	if nested["credit_card"] != "[REDACTED]" {
		t.Errorf("Expected credit_card to be [REDACTED], got %v", nested["credit_card"])
	}
	if nested["safe_field"] != "hello world" {
		t.Errorf("Expected safe_field to be preserved, got %v", nested["safe_field"])
	}

	if event.Request.Headers["Authorization"] != "[REDACTED]" {
		t.Errorf("Expected Authorization header to be [REDACTED], got %s", event.Request.Headers["Authorization"])
	}
	if event.Request.Cookies != "[REDACTED_COOKIES]" {
		t.Errorf("Expected cookies to be [REDACTED_COOKIES], got %s", event.Request.Cookies)
	}

	if event.Message != "Authentication failed with token [REDACTED_JWT]" {
		t.Errorf("Expected JWT to be scrubbed from message, got: %s", event.Message)
	}
}
