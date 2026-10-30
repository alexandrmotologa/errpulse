package ingest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/alexandrmotologa/errpulse/internal/grouping"
	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// StackResolver remaps minified frames against uploaded release assets.
type StackResolver interface {
	RemapStacktrace(projectID int64, release string, frames []SentryFrame) []SentryFrame
}

// Broadcaster sends real-time event notifications to connected frontend clients.
type Broadcaster interface {
	Broadcast(projectID int64, eventType string, data interface{})
}

// AlertQueue delivers instant incident alerts to external webhook channels.
type AlertQueue interface {
	Enqueue(projectID int64, issue *storage.Issue, reason string)
}

// IngestHandler serves Sentry envelope and legacy store HTTP endpoints.
type IngestHandler struct {
	db          *storage.DB
	resolver    StackResolver
	broadcaster Broadcaster
	alertQueue  AlertQueue
}

// NewIngestHandler creates an ingestion handler instance.
func NewIngestHandler(
	db *storage.DB,
	resolver StackResolver,
	broadcaster Broadcaster,
	alertQueue AlertQueue,
) *IngestHandler {
	return &IngestHandler{
		db:          db,
		resolver:    resolver,
		broadcaster: broadcaster,
		alertQueue:  alertQueue,
	}
}

// HandleEnvelope processes POST /api/{project_id}/envelope/ requests.
func (h *IngestHandler) HandleEnvelope(w http.ResponseWriter, r *http.Request) {
	projectID, proj, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := DecompressBody(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Decompression error: %v", err), http.StatusBadRequest)
		return
	}

	envelope, err := ParseEnvelope(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Envelope error: %v", err), http.StatusBadRequest)
		return
	}

	event, err := envelope.ExtractEvent()
	if err != nil {
		log.Printf("[ErrPulse] ExtractEvent notice: %v (items count: %d)", err, len(envelope.Items))
		// Envelopes without events (e.g. pure session pings or client reports) return success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"%s"}`, envelope.Header.EventID)))
		return
	}

	if event.EventID == "" {
		event.EventID = envelope.Header.EventID
	}

	h.processAndPersistEvent(w, projectID, proj, event, string(body))
}

// HandleStore processes legacy POST /api/{project_id}/store/ requests.
func (h *IngestHandler) HandleStore(w http.ResponseWriter, r *http.Request) {
	projectID, proj, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := DecompressBody(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Decompression error: %v", err), http.StatusBadRequest)
		return
	}

	var event SentryEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, fmt.Sprintf("JSON parse error: %v", err), http.StatusBadRequest)
		return
	}

	if event.EventID == "" {
		event.EventID = stringsTrimHyphens(uuid.New().String())
	}

	h.processAndPersistEvent(w, projectID, proj, &event, string(body))
}

// HandleSecurity processes CSP and security report endpoints.
func (h *IngestHandler) HandleSecurity(w http.ResponseWriter, r *http.Request) {
	projectID, proj, err := h.authenticate(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := DecompressBody(r)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	event := &SentryEvent{
		EventID:     stringsTrimHyphens(uuid.New().String()),
		Timestamp:   FlexibleTime{Time: time.Now().UTC()},
		Platform:    "csp",
		Message:     "CSP Security Violation",
		Environment: "production",
		Extra: map[string]interface{}{
			"raw_csp": string(body),
		},
	}

	h.processAndPersistEvent(w, projectID, proj, event, string(body))
}

func (h *IngestHandler) authenticate(r *http.Request) (int64, *storage.Project, error) {
	// Extract project_id from URL path e.g. /api/{project_id}/envelope/
	projectID := extractProjectIDFromPath(r.URL.Path)

	auth := ParseSentryAuth(r)

	// Fetch project by ID or by Public Key
	var proj *storage.Project
	var err error

	if projectID > 0 {
		proj, err = h.db.GetProjectByID(projectID)
	}

	if proj == nil && auth.PublicKey != "" {
		proj, err = h.db.GetProjectByKey(auth.PublicKey)
		if proj != nil {
			projectID = proj.ID
		}
	}

	if err != nil {
		return 0, nil, fmt.Errorf("database lookup failed: %w", err)
	}

	// If no project was found, attempt fallback to default project (ID: 1)
	if proj == nil {
		proj, _ = h.db.GetProjectByID(1)
		if proj != nil {
			projectID = 1
		}
	}

	if proj == nil {
		return 0, nil, fmt.Errorf("project not found")
	}

	return projectID, proj, nil
}

func (h *IngestHandler) processAndPersistEvent(
	w http.ResponseWriter,
	projectID int64,
	proj *storage.Project,
	event *SentryEvent,
	rawPayload string,
) {
	// 1. Remap stack trace frames using source maps if available
	if h.resolver != nil && event.Exception != nil {
		for i := range event.Exception.Values {
			ex := &event.Exception.Values[i]
			if ex.Stacktrace != nil && len(ex.Stacktrace.Frames) > 0 {
				ex.Stacktrace.Frames = h.resolver.RemapStacktrace(projectID, event.Release, ex.Stacktrace.Frames)
			}
		}
	}

	// 2. Generate deterministic issue fingerprint
	fingerprint, title, culprit, exceptionType := grouping.GenerateFingerprint(event)

	// 3. User identification for distinct user metrics
	userIdentifier := ""
	if event.User != nil {
		if event.User.ID != "" {
			userIdentifier = event.User.ID
		} else if event.User.Email != "" {
			userIdentifier = event.User.Email
		} else if event.User.IPAddress != "" {
			userIdentifier = event.User.IPAddress
		}
	}

	eventTime := event.Timestamp.Time
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}

	// 4. Upsert issue in SQLite
	issue, isNew, isRegression, err := h.db.UpsertIssue(
		projectID, fingerprint, title, culprit, exceptionType,
		event.Release, userIdentifier, eventTime,
	)
	if err != nil {
		log.Printf("[ErrPulse] UpsertIssue error: %v", err)
		http.Error(w, "Failed updating issue", http.StatusInternalServerError)
		return
	}

	// 5. Serialize complex event fields to JSON
	userJSON, _ := json.Marshal(event.User)
	tagsJSON, _ := json.Marshal(event.Tags)
	extraJSON, _ := json.Marshal(event.Extra)

	var breadcrumbsJSON []byte
	if event.Breadcrumbs != nil {
		breadcrumbsJSON, _ = json.Marshal(event.Breadcrumbs.Values)
	}

	var stacktraceJSON []byte
	if event.Exception != nil {
		stacktraceJSON, _ = json.Marshal(event.Exception.Values)
	}

	// 6. Persist event
	storageEvent := &storage.Event{
		ID:              event.EventID,
		IssueID:         issue.ID,
		ProjectID:       projectID,
		Timestamp:       eventTime,
		Environment:     event.Environment,
		Release:         event.Release,
		Platform:        event.Platform,
		Message:         title,
		UserJSON:        string(userJSON),
		TagsJSON:        string(tagsJSON),
		ExtraJSON:       string(extraJSON),
		BreadcrumbsJSON: string(breadcrumbsJSON),
		StacktraceJSON:  string(stacktraceJSON),
		RawPayload:      rawPayload,
	}

	if err := h.db.CreateEvent(storageEvent); err != nil {
		log.Printf("[ErrPulse] CreateEvent error: %v", err)
	}

	// 7. Enqueue alerts if new issue or regression
	if h.alertQueue != nil {
		if isNew {
			h.alertQueue.Enqueue(projectID, issue, "NEW_ISSUE")
		} else if isRegression {
			h.alertQueue.Enqueue(projectID, issue, "REGRESSION")
		}
	}

	// 8. Broadcast real-time SSE update
	if h.broadcaster != nil {
		h.broadcaster.Broadcast(projectID, "new_event", map[string]interface{}{
			"issue":         issue,
			"event_id":      event.EventID,
			"is_new":        isNew,
			"is_regression": isRegression,
		})
	}

	// 9. Respond with Sentry standard HTTP 200 JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"%s"}`, event.EventID)))
}

func extractProjectIDFromPath(path string) int64 {
	// Patterns: /api/1/envelope/, /api/1/store/, /api/1/security/
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "api" && i+1 < len(parts) {
			if id, err := strconv.ParseInt(parts[i+1], 10, 64); err == nil {
				return id
			}
		}
	}
	return 0
}
