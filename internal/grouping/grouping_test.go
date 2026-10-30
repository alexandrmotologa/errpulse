package grouping

import (
	"testing"

	"github.com/alexandrmotologa/errpulse/internal/models"
)

func TestNormalizer(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "Failed to fetch user at 0x7ffee12a4b80",
			expected: "Failed to fetch user at <HEX>",
		},
		{
			input:    "Order 4f29a008-6202-475a-bc81-540eeea03e62 processing timed out",
			expected: "Order <UUID> processing timed out",
		},
		{
			input:    "Connection failed to 192.168.1.50 at 2026-10-30T17:42:00Z",
			expected: "Connection failed to <IP> at <TIME>",
		},
		{
			input:    "Route /users/49182/settings failed",
			expected: "Route /users/<ID>/settings failed",
		},
	}

	for _, c := range cases {
		out := NormalizeMessage(c.input)
		if out != c.expected {
			t.Errorf("NormalizeMessage(%q) = %q; want %q", c.input, out, c.expected)
		}
	}
}

func TestFingerprintDeduplication(t *testing.T) {
	// Two events with different UUIDs, memory addresses, and timestamps, but same root cause
	event1 := &models.SentryEvent{
		Exception: &models.SentryExceptionPayload{
			Values: []models.SentryException{
				{
					Type:  "TypeError",
					Value: "Cannot read property 'name' of undefined at 0xdeadbeef",
					Stacktrace: &models.SentryStacktrace{
						Frames: []models.SentryFrame{
							{Filename: "node_modules/react/index.js", Function: "render"},
							{Filename: "src/components/UserCard.tsx", Function: "UserCard", Lineno: 45},
						},
					},
				},
			},
		},
	}

	event2 := &models.SentryEvent{
		Exception: &models.SentryExceptionPayload{
			Values: []models.SentryException{
				{
					Type:  "TypeError",
					Value: "Cannot read property 'name' of undefined at 0x12345678",
					Stacktrace: &models.SentryStacktrace{
						Frames: []models.SentryFrame{
							{Filename: "node_modules/react/index.js", Function: "render"},
							{Filename: "src/components/UserCard.tsx", Function: "UserCard", Lineno: 45},
						},
					},
				},
			},
		},
	}

	fp1, title1, culprit1, _ := GenerateFingerprint(event1)
	fp2, title2, culprit2, _ := GenerateFingerprint(event2)

	if fp1 != fp2 {
		t.Fatalf("Expected deterministic matching fingerprints: %s vs %s", fp1, fp2)
	}

	if culprit1 != culprit2 {
		t.Errorf("Expected identical culprits: %s vs %s", culprit1, culprit2)
	}

	_ = title1
	_ = title2
}
