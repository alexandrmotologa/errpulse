package alerts

import (
	"testing"
	"time"
)

func TestThrottler(t *testing.T) {
	throttler := NewThrottler(100*time.Millisecond, 5)

	// 1. First time NEW_ISSUE should always pass
	allowed, reason := throttler.ShouldSend(1, "slack", "NEW_ISSUE")
	if !allowed || reason != "NEW_ISSUE" {
		t.Fatalf("Expected NEW_ISSUE to be allowed")
	}

	// 2. Immediate repeat of REGRESSION should be throttled by cooldown
	allowed, _ = throttler.ShouldSend(1, "slack", "REGRESSION")
	if allowed {
		t.Fatalf("Expected second alert within cooldown to be throttled")
	}

	// 3. Different channel should be allowed
	allowed, _ = throttler.ShouldSend(1, "discord", "NEW_ISSUE")
	if !allowed {
		t.Fatalf("Expected different channel to be allowed")
	}

	// 4. Test volume spike detection (>5 events in 1 minute)
	for i := 0; i < 4; i++ {
		throttler.ShouldSend(2, "slack", "REGRESSION")
	}
	allowed, reason = throttler.ShouldSend(2, "slack", "REGRESSION")
	if !allowed || reason != "SPIKE_DETECTED" {
		t.Errorf("Expected SPIKE_DETECTED when crossing threshold, got %v (%s)", allowed, reason)
	}
}
