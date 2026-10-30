package ingest

import (
	"regexp"
	"strings"

	"github.com/alexandrmotologa/errpulse/internal/models"
)

var (
	sensitiveKeySubstrings = []string{
		"password",
		"passwd",
		"secret",
		"token",
		"api_key",
		"apikey",
		"access_token",
		"auth_token",
		"authorization",
		"private_key",
		"credit_card",
		"card_number",
		"cvv",
		"cvc",
		"ssn",
	}

	reJWT  = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)
	reCard = regexp.MustCompile(`\b(?:\d{4}[ -]?){3}\d{4}\b`)
)

// SanitizeEvent recursively scrubs sensitive PII, passwords, cards, and authorization tokens.
func SanitizeEvent(event *models.SentryEvent) {
	if event == nil {
		return
	}

	// 1. Sanitize top-level message
	if event.Message != "" {
		event.Message = scrubString(event.Message)
	}

	// 2. Sanitize exception messages
	if event.Exception != nil {
		for i := range event.Exception.Values {
			ex := &event.Exception.Values[i]
			ex.Value = scrubString(ex.Value)
		}
	}

	// 3. Sanitize tags
	if event.Tags != nil {
		for k, v := range event.Tags {
			if isSensitiveKey(k) {
				event.Tags[k] = "[REDACTED]"
			} else {
				event.Tags[k] = scrubString(v)
			}
		}
	}

	// 4. Sanitize extra parameters
	if event.Extra != nil {
		event.Extra = sanitizeMap(event.Extra)
	}

	// 5. Sanitize HTTP Request details
	if event.Request != nil {
		if event.Request.Headers != nil {
			for k, v := range event.Request.Headers {
				if isSensitiveKey(k) {
					event.Request.Headers[k] = "[REDACTED]"
				} else {
					event.Request.Headers[k] = scrubString(v)
				}
			}
		}
		if event.Request.Cookies != "" {
			event.Request.Cookies = "[REDACTED_COOKIES]"
		}
		if event.Request.Data != nil {
			if m, ok := event.Request.Data.(map[string]interface{}); ok {
				event.Request.Data = sanitizeMap(m)
			} else if s, ok := event.Request.Data.(string); ok {
				event.Request.Data = scrubString(s)
			}
		}
	}

	// 6. Sanitize breadcrumb data
	if event.Breadcrumbs != nil {
		for i := range event.Breadcrumbs.Values {
			b := &event.Breadcrumbs.Values[i]
			b.Message = scrubString(b.Message)
			if b.Data != nil {
				b.Data = sanitizeMap(b.Data)
			}
		}
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, sub := range sensitiveKeySubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

func scrubString(s string) string {
	if s == "" {
		return s
	}
	s = reJWT.ReplaceAllString(s, "[REDACTED_JWT]")
	s = reCard.ReplaceAllString(s, "[REDACTED_CARD]")
	return s
}

func sanitizeMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	clean := make(map[string]interface{}, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			clean[k] = "[REDACTED]"
			continue
		}

		switch val := v.(type) {
		case string:
			clean[k] = scrubString(val)
		case map[string]interface{}:
			clean[k] = sanitizeMap(val)
		case []interface{}:
			clean[k] = sanitizeSlice(val)
		default:
			clean[k] = val
		}
	}
	return clean
}

func sanitizeSlice(s []interface{}) []interface{} {
	clean := make([]interface{}, len(s))
	for i, item := range s {
		switch val := item.(type) {
		case string:
			clean[i] = scrubString(val)
		case map[string]interface{}:
			clean[i] = sanitizeMap(val)
		case []interface{}:
			clean[i] = sanitizeSlice(val)
		default:
			clean[i] = val
		}
	}
	return clean
}
