package alerts

import (
	"sync"
	"time"
)

// Throttler prevents alert storms by enforcing cooldowns and detecting traffic spikes.
type Throttler struct {
	cooldownDuration time.Duration
	spikeThreshold   int // events per minute to trigger emergency spike alert
	lastSent         map[string]time.Time
	eventCounts      map[int64][]time.Time
	mu               sync.Mutex
}

// NewThrottler initializes alert rate limiting and spike detection.
func NewThrottler(cooldown time.Duration, spikeThreshold int) *Throttler {
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	if spikeThreshold <= 0 {
		spikeThreshold = 20
	}
	return &Throttler{
		cooldownDuration: cooldown,
		spikeThreshold:   spikeThreshold,
		lastSent:         make(map[string]time.Time),
		eventCounts:      make(map[int64][]time.Time),
	}
}

// ShouldSend checks if an alert should be dispatched or throttled.
// Returns (allowed, effectiveReason).
func (t *Throttler) ShouldSend(issueID int64, channel, reason string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()
	key := channelKey(issueID, channel)

	// Record event occurrence for sliding 1-minute window
	t.recordEvent(issueID, now)
	recentEvents := len(t.eventCounts[issueID])

	// Check for emergency volume spike
	if recentEvents >= t.spikeThreshold {
		// Clear window to avoid immediately re-spiking
		t.eventCounts[issueID] = nil
		t.lastSent[key] = now
		return true, "SPIKE_DETECTED"
	}

	// Always allow new issue first detection
	if reason == "NEW_ISSUE" {
		t.lastSent[key] = now
		return true, reason
	}

	// Cooldown enforcement for repeat/regression alerts
	last, exists := t.lastSent[key]
	if exists && now.Sub(last) < t.cooldownDuration {
		return false, ""
	}

	t.lastSent[key] = now
	return true, reason
}

func (t *Throttler) recordEvent(issueID int64, now time.Time) {
	cutoff := now.Add(-1 * time.Minute)
	var filtered []time.Time
	for _, ts := range t.eventCounts[issueID] {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	filtered = append(filtered, now)
	t.eventCounts[issueID] = filtered
}

func channelKey(issueID int64, channel string) string {
	return string(rune(issueID)) + ":" + channel
}
