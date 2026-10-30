package storage

import (
	"testing"
	"time"
)

func TestStorageLifecycle(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed opening memory database: %v", err)
	}
	defer db.Close()

	// 1. Verify default project
	projects, err := db.ListProjects()
	if err != nil {
		t.Fatalf("Failed listing projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("Expected 1 default project, got %d", len(projects))
	}
	defaultProj := projects[0]
	if defaultProj.PublicKey != "errpulse_default_key" {
		t.Errorf("Expected public key 'errpulse_default_key', got '%s'", defaultProj.PublicKey)
	}

	// 2. Test UpsertIssue (New Issue)
	fingerprint := "e3b0c44298fc1c149afbf4c8996fb924"
	title := "TypeError: Cannot read properties of undefined"
	culprit := "calculateTotal(cart.ts:42)"
	now := time.Now().UTC()

	issue, isNew, isRegression, err := db.UpsertIssue(
		defaultProj.ID, fingerprint, title, culprit, "TypeError", "v1.0.0", "user-123", now,
	)
	if err != nil {
		t.Fatalf("Failed upserting new issue: %v", err)
	}
	if !isNew {
		t.Errorf("Expected isNew to be true")
	}
	if isRegression {
		t.Errorf("Expected isRegression to be false")
	}
	if issue.EventCount != 1 {
		t.Errorf("Expected event_count 1, got %d", issue.EventCount)
	}

	// 3. Test UpsertIssue (Duplicate Issue)
	issue2, isNew2, isReg2, err := db.UpsertIssue(
		defaultProj.ID, fingerprint, title, culprit, "TypeError", "v1.0.0", "user-456", now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("Failed upserting duplicate issue: %v", err)
	}
	if isNew2 {
		t.Errorf("Expected isNew to be false for duplicate")
	}
	if isReg2 {
		t.Errorf("Expected isRegression to be false")
	}
	if issue2.EventCount != 2 {
		t.Errorf("Expected event_count 2, got %d", issue2.EventCount)
	}
	if issue2.UserCount != 2 {
		t.Errorf("Expected user_count 2, got %d", issue2.UserCount)
	}

	// 4. Test Regression Detection
	err = db.UpdateIssueStatus(issue.ID, StatusResolved)
	if err != nil {
		t.Fatalf("Failed resolving issue: %v", err)
	}

	issue3, isNew3, isReg3, err := db.UpsertIssue(
		defaultProj.ID, fingerprint, title, culprit, "TypeError", "v1.0.1", "user-789", now.Add(2*time.Hour),
	)
	if err != nil {
		t.Fatalf("Failed upserting regressed issue: %v", err)
	}
	if isNew3 {
		t.Errorf("Expected isNew to be false for regression")
	}
	if !isReg3 {
		t.Errorf("Expected isRegression to be true")
	}
	if issue3.Status != StatusUnresolved {
		t.Errorf("Expected status to revert to 'unresolved', got '%s'", issue3.Status)
	}

	// 5. Test Event Insertion & Retrieval
	eventID := "a1b2c3d4e5f67890"
	testEvent := &Event{
		ID:          eventID,
		IssueID:     issue.ID,
		ProjectID:   defaultProj.ID,
		Timestamp:   now,
		Environment: "production",
		Release:     "v1.0.1",
		Platform:    "javascript",
		Message:     title,
	}
	if err := db.CreateEvent(testEvent); err != nil {
		t.Fatalf("Failed creating event: %v", err)
	}

	fetchedEvent, err := db.GetEvent(eventID)
	if err != nil {
		t.Fatalf("Failed fetching event: %v", err)
	}
	if fetchedEvent == nil || fetchedEvent.ID != eventID {
		t.Errorf("Fetched event mismatch: %+v", fetchedEvent)
	}

	// 6. Test Stats Summary
	summary, err := db.GetStatsSummary(defaultProj.ID)
	if err != nil {
		t.Fatalf("Failed getting stats summary: %v", err)
	}
	if summary.TotalEvents != 1 {
		t.Errorf("Expected total events 1, got %d", summary.TotalEvents)
	}
	if summary.UnresolvedIssues != 1 {
		t.Errorf("Expected unresolved issues 1, got %d", summary.UnresolvedIssues)
	}

	// 7. Test Release Artifacts
	mapData := []byte(`{"version":3,"sources":["test.ts"],"mappings":"AAAA"}`)
	err = db.SaveArtifact(defaultProj.ID, "v1.0.0", "bundle.js.map", "application/json", mapData)
	if err != nil {
		t.Fatalf("Failed saving artifact: %v", err)
	}

	art, err := db.GetArtifact(defaultProj.ID, "v1.0.0", "bundle.js.map")
	if err != nil {
		t.Fatalf("Failed fetching artifact: %v", err)
	}
	if art == nil || string(art.Data) != string(mapData) {
		t.Errorf("Artifact data mismatch")
	}

	releases, err := db.ListReleases(defaultProj.ID)
	if err != nil {
		t.Fatalf("Failed listing releases: %v", err)
	}
	if len(releases) == 0 {
		t.Errorf("Expected non-empty releases list")
	}

	// 8. Test Alert Configurations
	cfg := &AlertConfig{
		ProjectID:          defaultProj.ID,
		Channel:            "slack",
		TargetURL:          "https://hooks.slack.com/services/test",
		Enabled:            true,
		NotifyOnNew:        true,
		NotifyOnRegression: true,
	}
	savedCfg, err := db.SaveAlertConfig(cfg)
	if err != nil {
		t.Fatalf("Failed saving alert config: %v", err)
	}
	if savedCfg.ID == 0 {
		t.Errorf("Expected non-zero ID for saved alert config")
	}

	configs, err := db.ListAlertConfigs(defaultProj.ID)
	if err != nil || len(configs) != 1 {
		t.Fatalf("Expected 1 alert config, got %d (err: %v)", len(configs), err)
	}
}
