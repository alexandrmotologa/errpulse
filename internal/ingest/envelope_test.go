package ingest

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseEnvelope(t *testing.T) {
	envelopeData := `{"event_id":"9ec69572404f4357b988f57297e68406","sent_at":"2026-10-30T17:42:00Z"}
{"type":"event","length":122,"content_type":"application/json"}
{"event_id":"9ec69572404f4357b988f57297e68406","platform":"javascript","message":"Test error","environment":"production"}
`

	env, err := ParseEnvelope([]byte(envelopeData))
	if err != nil {
		t.Fatalf("Failed parsing envelope: %v", err)
	}

	if env.Header.EventID != "9ec69572404f4357b988f57297e68406" {
		t.Errorf("Unexpected event ID: %s", env.Header.EventID)
	}

	if len(env.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(env.Items))
	}

	event, err := env.ExtractEvent()
	if err != nil {
		t.Fatalf("Failed extracting event: %v", err)
	}

	if event.Message != "Test error" {
		t.Errorf("Expected message 'Test error', got '%s'", event.Message)
	}
	if event.Platform != "javascript" {
		t.Errorf("Expected platform 'javascript', got '%s'", event.Platform)
	}
}

func TestDecompressGzip(t *testing.T) {
	raw := []byte(`{"event_id":"test"}`)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write(raw)
	_ = gw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/", &buf)
	req.Header.Set("Content-Encoding", "gzip")

	decompressed, err := DecompressBody(req)
	if err != nil {
		t.Fatalf("Failed decompressing body: %v", err)
	}

	if !strings.Contains(string(decompressed), `"test"`) {
		t.Errorf("Decompressed content mismatch: %s", string(decompressed))
	}
}

func TestParseSentryAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/", nil)
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_client=sentry.javascript.browser/8.26.0, sentry_key=my_public_key")

	creds := ParseSentryAuth(req)
	if creds.PublicKey != "my_public_key" {
		t.Errorf("Expected sentry_key 'my_public_key', got '%s'", creds.PublicKey)
	}
	if creds.Version != "7" {
		t.Errorf("Expected sentry_version '7', got '%s'", creds.Version)
	}
	if creds.Client != "sentry.javascript.browser/8.26.0" {
		t.Errorf("Expected client 'sentry.javascript.browser/8.26.0', got '%s'", creds.Client)
	}
}
