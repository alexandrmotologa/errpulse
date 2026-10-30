package alerts

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// AlertJob encapsulates an alert delivery task.
type AlertJob struct {
	ProjectID int64
	Issue     *storage.Issue
	Reason    string // "NEW_ISSUE" or "REGRESSION"
	BaseURL   string
}

// Dispatcher processes and routes alert jobs asynchronously.
type Dispatcher struct {
	db      *storage.DB
	queue   chan AlertJob
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	baseURL string
}

// NewDispatcher creates a new alert dispatcher with background workers.
func NewDispatcher(db *storage.DB, baseURL string) *Dispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	d := &Dispatcher{
		db:      db,
		queue:   make(chan AlertJob, 100),
		ctx:     ctx,
		cancel:  cancel,
		baseURL: strings.TrimRight(baseURL, "/"),
	}

	// Start worker pool
	d.wg.Add(1)
	go d.worker()

	return d
}

// Enqueue puts an alert job onto the delivery queue.
func (d *Dispatcher) Enqueue(projectID int64, issue *storage.Issue, reason string) {
	if issue == nil {
		return
	}
	select {
	case d.queue <- AlertJob{
		ProjectID: projectID,
		Issue:     issue,
		Reason:    reason,
		BaseURL:   d.baseURL,
	}:
	default:
		log.Printf("[ErrPulse] Alert queue full, dropping alert for issue %d", issue.ID)
	}
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case job := <-d.queue:
			d.processJob(job)
		}
	}
}

func (d *Dispatcher) processJob(job AlertJob) {
	configs, err := d.db.ListAlertConfigs(job.ProjectID)
	if err != nil || len(configs) == 0 {
		return
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		shouldSend := false
		if job.Reason == "NEW_ISSUE" && cfg.NotifyOnNew {
			shouldSend = true
		} else if job.Reason == "REGRESSION" && cfg.NotifyOnRegression {
			shouldSend = true
		}

		if !shouldSend {
			continue
		}

		go func(c storage.AlertConfig) {
			err := d.sendToChannel(c.Channel, c.TargetURL, job.BaseURL, job.Reason, job.Issue)
			status := "success"
			respMsg := "OK"
			if err != nil {
				status = "error"
				respMsg = err.Error()
				log.Printf("[ErrPulse] Failed sending alert to %s (%s): %v", c.Channel, c.TargetURL, err)
			}
			_ = d.db.LogAlert(job.ProjectID, job.Issue.ID, c.Channel, status, respMsg)
		}(cfg)
	}
}

func (d *Dispatcher) sendToChannel(channel, targetURL, baseURL, reason string, issue *storage.Issue) error {
	switch strings.ToLower(channel) {
	case "slack":
		return SendSlackNotification(targetURL, baseURL, reason, issue)
	case "discord":
		return SendDiscordNotification(targetURL, baseURL, reason, issue)
	case "telegram":
		return SendTelegramNotification(targetURL, baseURL, reason, issue)
	case "webhook":
		fallthrough
	default:
		return SendGenericWebhook(targetURL, baseURL, reason, issue)
	}
}

// TestAlert sends a simulated verification alert to a configured destination.
func (d *Dispatcher) TestAlert(cfg *storage.AlertConfig, baseURL string) error {
	dummyIssue := &storage.Issue{
		ID:          999,
		ProjectID:   cfg.ProjectID,
		Title:       "TestException: Verification alert from ErrPulse",
		Culprit:     "testAlertHandler(test.go:42)",
		Type:        "TestException",
		Status:      storage.StatusUnresolved,
		FirstSeenAt: time.Now().UTC(),
		LastSeenAt:  time.Now().UTC(),
		EventCount:  1,
		UserCount:   1,
		LastRelease: "v1.0.0-test",
	}

	return d.sendToChannel(cfg.Channel, cfg.TargetURL, baseURL, "NEW_ISSUE", dummyIssue)
}

// Close gracefully flushes pending alerts and stops workers.
func (d *Dispatcher) Close() {
	d.cancel()
	d.wg.Wait()
}
