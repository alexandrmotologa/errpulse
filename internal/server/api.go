package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/alerts"
	"github.com/alexandrmotologa/errpulse/internal/ingest"
	"github.com/alexandrmotologa/errpulse/internal/sourcemap"
	"github.com/alexandrmotologa/errpulse/internal/storage"
)

// APIHandler handles REST API calls for the React 19 incident management studio.
type APIHandler struct {
	db          *storage.DB
	resolver    *sourcemap.Resolver
	dispatcher  *alerts.Dispatcher
	ingestor    *ingest.IngestHandler
	broadcaster *SSEHub
	baseURL     string
}

// NewAPIHandler constructs a new REST API router.
func NewAPIHandler(
	db *storage.DB,
	resolver *sourcemap.Resolver,
	dispatcher *alerts.Dispatcher,
	ingestor *ingest.IngestHandler,
	broadcaster *SSEHub,
	baseURL string,
) *APIHandler {
	return &APIHandler{
		db:          db,
		resolver:    resolver,
		dispatcher:  dispatcher,
		ingestor:    ingestor,
		broadcaster: broadcaster,
		baseURL:     baseURL,
	}
}

// ServeHTTP routes REST API requests based on method and URL path.
func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for local web development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Sentry-Auth")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	trimmed := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(trimmed, "/")

	// Minimum: api/v1/<resource>
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" {
		http.NotFound(w, r)
		return
	}

	resource := parts[2]

	switch resource {
	case "stats":
		h.handleStats(w, r)
	case "projects":
		h.handleProjects(w, r, parts[3:])
	case "issues":
		h.handleIssues(w, r, parts[3:])
	case "events":
		h.handleEvents(w, r, parts[3:])
	case "releases":
		h.handleReleases(w, r, parts[3:])
	case "artifacts":
		h.handleArtifacts(w, r, parts[3:])
	case "alerts":
		h.handleAlerts(w, r, parts[3:])
	case "simulate":
		h.handleSimulate(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	projID, _ := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	summary, err := h.db.GetStatsSummary(projID)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(summary)
}

func (h *APIHandler) handleProjects(w http.ResponseWriter, r *http.Request, sub []string) {
	if len(sub) == 0 {
		if r.Method == http.MethodGet {
			projects, err := h.db.ListProjects()
			if err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(projects)
			return
		}

		if r.Method == http.MethodPost {
			var body struct {
				Name string `json:"name"`
				Slug string `json:"slug"`
				Key  string `json:"public_key"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSONError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if body.Name == "" {
				body.Name = "New Project"
			}
			if body.Slug == "" {
				body.Slug = fmt.Sprintf("proj-%d", time.Now().Unix())
			}
			p, err := h.db.CreateProject(body.Name, body.Slug, body.Key)
			if err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(p)
			return
		}
	}

	if len(sub) == 1 {
		id, err := strconv.ParseInt(sub[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		p, err := h.db.GetProjectByID(id)
		if err != nil || p == nil {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(p)
		return
	}

	http.NotFound(w, r)
}

func (h *APIHandler) handleIssues(w http.ResponseWriter, r *http.Request, sub []string) {
	// GET /api/v1/issues
	if len(sub) == 0 && r.Method == http.MethodGet {
		q := r.URL.Query()
		projectID, _ := strconv.ParseInt(q.Get("project_id"), 10, 64)
		status := storage.IssueStatus(q.Get("status"))
		environment := q.Get("environment")
		search := q.Get("search")
		if search == "" {
			search = q.Get("query")
		}
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))

		issues, total, err := h.db.ListIssues(projectID, status, environment, search, limit, offset)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"issues": issues,
			"total":  total,
		})
		return
	}

	if len(sub) == 1 {
		id, err := strconv.ParseInt(sub[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// GET /api/v1/issues/{id}
		if r.Method == http.MethodGet {
			issue, err := h.db.GetIssue(id)
			if err != nil || issue == nil {
				http.NotFound(w, r)
				return
			}

			// Include latest event
			latestEvent, _ := h.db.GetLatestEventForIssue(id)

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"issue":        issue,
				"latest_event": latestEvent,
			})
			return
		}

		// PATCH /api/v1/issues/{id} (status change)
		if r.Method == http.MethodPatch {
			var body struct {
				Status storage.IssueStatus `json:"status"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSONError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			if err := h.db.UpdateIssueStatus(id, body.Status); err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			issue, _ := h.db.GetIssue(id)
			_ = json.NewEncoder(w).Encode(issue)
			return
		}

		// DELETE /api/v1/issues/{id}
		if r.Method == http.MethodDelete {
			if err := h.db.DeleteIssue(id); err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
			return
		}
	}

	// GET /api/v1/issues/{id}/occurrence/{offset} (or /occurrences/{offset})
	if len(sub) == 3 && (sub[1] == "occurrence" || sub[1] == "occurrences") && r.Method == http.MethodGet {
		id, err := strconv.ParseInt(sub[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		offset, _ := strconv.Atoi(sub[2])

		event, err := h.db.GetEventByOccurrence(id, offset)
		if err != nil || event == nil {
			http.NotFound(w, r)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"event":      event,
			"occurrence": offset,
		})
		return
	}

	// GET /api/v1/issues/{id}/events
	if len(sub) == 2 && sub[1] == "events" && r.Method == http.MethodGet {
		id, err := strconv.ParseInt(sub[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		events, total, err := h.db.ListEventsForIssue(id, limit, offset)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"events": events,
			"total":  total,
		})
		return
	}

	http.NotFound(w, r)
}

func (h *APIHandler) handleEvents(w http.ResponseWriter, r *http.Request, sub []string) {
	// GET /api/v1/events/{id}
	if len(sub) == 1 && r.Method == http.MethodGet {
		eventID := sub[0]
		event, err := h.db.GetEvent(eventID)
		if err != nil || event == nil {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(event)
		return
	}
	http.NotFound(w, r)
}

func (h *APIHandler) handleReleases(w http.ResponseWriter, r *http.Request, sub []string) {
	projectID, _ := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	if projectID == 0 {
		projectID = 1
	}

	// GET /api/v1/releases
	if len(sub) == 0 && r.Method == http.MethodGet {
		releases, err := h.db.ListReleases(projectID)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(releases)
		return
	}

	// GET /api/v1/releases/health
	if len(sub) == 1 && sub[0] == "health" && r.Method == http.MethodGet {
		healthList, err := h.db.ListReleasesHealth(projectID)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(healthList)
		return
	}

	// GET /api/v1/releases/{release}/health
	if len(sub) == 2 && sub[1] == "health" && r.Method == http.MethodGet {
		health, err := h.db.GetReleaseHealth(projectID, sub[0])
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(health)
		return
	}

	// GET /api/v1/releases/{release}/artifacts
	if len(sub) == 2 && sub[1] == "artifacts" {
		release := sub[0]
		if r.Method == http.MethodGet {
			artifacts, err := h.db.ListArtifacts(projectID, release)
			if err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(artifacts)
			return
		}

		// POST /api/v1/releases/{release}/artifacts (upload source map)
		if r.Method == http.MethodPost {
			if err := r.ParseMultipartForm(50 << 20); err != nil {
				writeJSONError(w, "Multipart form error", http.StatusBadRequest)
				return
			}

			file, header, err := r.FormFile("file")
			if err != nil {
				writeJSONError(w, "Missing file parameter", http.StatusBadRequest)
				return
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				writeJSONError(w, "Failed reading file data", http.StatusInternalServerError)
				return
			}

			filename := r.FormValue("filename")
			if filename == "" {
				filename = header.Filename
			}
			if filename == "" {
				filename = path.Base(header.Filename)
			}

			contentType := header.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/json"
			}

			if err := h.db.SaveArtifact(projectID, release, filename, contentType, data); err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if h.resolver != nil {
				h.resolver.InvalidateCache(projectID, release)
			}

			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"filename": filename,
				"release":  release,
				"size":     len(data),
			})
			return
		}
	}

	http.NotFound(w, r)
}

func (h *APIHandler) handleArtifacts(w http.ResponseWriter, r *http.Request, sub []string) {
	// DELETE /api/v1/artifacts/{id}
	if len(sub) == 1 && r.Method == http.MethodDelete {
		id, err := strconv.ParseInt(sub[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := h.db.DeleteArtifact(id); err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
		return
	}
	http.NotFound(w, r)
}

func (h *APIHandler) handleAlerts(w http.ResponseWriter, r *http.Request, sub []string) {
	projectID, _ := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	if projectID == 0 {
		projectID = 1
	}

	// GET /api/v1/alerts
	if len(sub) == 0 && r.Method == http.MethodGet {
		configs, err := h.db.ListAlertConfigs(projectID)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(configs)
		return
	}

	// POST /api/v1/alerts (create / update)
	if len(sub) == 0 && r.Method == http.MethodPost {
		var cfg storage.AlertConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSONError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if cfg.ProjectID == 0 {
			cfg.ProjectID = projectID
		}
		saved, err := h.db.SaveAlertConfig(&cfg)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(saved)
		return
	}

	// POST /api/v1/alerts/test (dispatch test alert)
	if len(sub) == 1 && sub[0] == "test" && r.Method == http.MethodPost {
		var cfg storage.AlertConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSONError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if cfg.ProjectID == 0 {
			cfg.ProjectID = projectID
		}

		err := h.dispatcher.TestAlert(&cfg, h.baseURL)
		if err != nil {
			writeJSONError(w, fmt.Sprintf("Alert test failed: %v", err), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Test alert sent successfully"})
		return
	}

	// DELETE /api/v1/alerts/{id}
	if len(sub) == 1 && r.Method == http.MethodDelete {
		id, err := strconv.ParseInt(sub[0], 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if err := h.db.DeleteAlertConfig(id); err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
		return
	}

	http.NotFound(w, r)
}

func (h *APIHandler) handleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	var req struct {
		Scenario string `json:"scenario"` // react_type_error, node_unhandled, go_panic, python_key_error
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if req.Scenario == "" {
		req.Scenario = "react_type_error"
	}

	event := GenerateSyntheticEvent(req.Scenario)

	// Send through internal ingestion pipeline
	inApp := true
	_ = inApp
	bodyJSON, _ := json.Marshal(event)

	// Process directly
	wDirect := &fakeResponseWriter{header: make(http.Header)}
	reqDirect, _ := http.NewRequest(http.MethodPost, "/api/1/store/", strings.NewReader(string(bodyJSON)))
	reqDirect.Header.Set("Content-Type", "application/json")
	h.ingestor.HandleStore(wDirect, reqDirect)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "simulated",
		"scenario": req.Scenario,
		"event_id": event.EventID,
		"message":  event.Message,
	})
}

type fakeResponseWriter struct {
	header     http.Header
	statusCode int
	body       []byte
}

func (f *fakeResponseWriter) Header() http.Header { return f.header }
func (f *fakeResponseWriter) Write(b []byte) (int, error) {
	f.body = append(f.body, b...)
	return len(b), nil
}
func (f *fakeResponseWriter) WriteHeader(statusCode int) { f.statusCode = statusCode }

// GenerateSyntheticEvent builds realistic sample Sentry crash events for evaluation.
func GenerateSyntheticEvent(scenario string) *ingest.SentryEvent {
	inAppTrue := true
	inAppFalse := false

	now := time.Now().UTC()
	eventID := strings.ReplaceAll(fmt.Sprintf("%d%d", now.UnixNano(), time.Now().Unix()), "-", "")
	if len(eventID) > 32 {
		eventID = eventID[:32]
	}

	switch scenario {
	case "go_panic":
		return &ingest.SentryEvent{
			EventID:     eventID,
			Timestamp:   ingest.FlexibleTime{Time: now},
			Platform:    "go",
			Level:       "fatal",
			Release:     "checkout-service@1.4.2",
			Environment: "production",
			User: &ingest.SentryUser{
				ID:        "usr_9918",
				Email:     "devops@errpulse.internal",
				IPAddress: "10.0.4.12",
			},
			Exception: &ingest.SentryExceptionPayload{
				Values: []ingest.SentryException{
					{
						Type:  "runtime.panic",
						Value: "runtime error: invalid memory address or nil pointer dereference [signal SIGSEGV]",
						Stacktrace: &ingest.SentryStacktrace{
							Frames: []ingest.SentryFrame{
								{Filename: "runtime/panic.go", Function: "gopanic", Lineno: 890, InApp: &inAppFalse},
								{Filename: "internal/payment/stripe.go", Function: "ProcessPayment", Lineno: 112, InApp: &inAppTrue, ContextLine: "amount := customer.DefaultCard.Balance", PreContext: []string{"func ProcessPayment(customer *Customer) error {", "    log.Println(\"Processing...\")"}, PostContext: []string{"    return stripe.Charge(amount)", "}"}},
								{Filename: "cmd/server/main.go", Function: "handleCheckout", Lineno: 58, InApp: &inAppTrue},
							},
						},
					},
				},
			},
			Breadcrumbs: &ingest.SentryBreadcrumbPayload{
				Values: []ingest.SentryBreadcrumb{
					{Category: "http", Message: "POST /v1/checkout - 200 OK", Level: "info"},
					{Category: "db", Message: "SELECT * FROM customers WHERE id = 9918", Level: "info"},
					{Category: "log", Message: "Customer profile loaded without active card", Level: "warn"},
				},
			},
			Tags: map[string]string{
				"service": "checkout",
				"region":  "us-east-1",
			},
		}

	case "python_key_error":
		return &ingest.SentryEvent{
			EventID:     eventID,
			Timestamp:   ingest.FlexibleTime{Time: now},
			Platform:    "python",
			Level:       "error",
			Release:     "analytics-worker@2.1.0",
			Environment: "staging",
			User: &ingest.SentryUser{
				ID:        "usr_py44",
				Email:     "analyst@example.org",
				IPAddress: "192.168.1.100",
			},
			Exception: &ingest.SentryExceptionPayload{
				Values: []ingest.SentryException{
					{
						Type:  "KeyError",
						Value: "'subscription_tier' missing from payload",
						Stacktrace: &ingest.SentryStacktrace{
							Frames: []ingest.SentryFrame{
								{Filename: "site-packages/celery/app/trace.py", Function: "trace_task", Lineno: 385, InApp: &inAppFalse},
								{Filename: "services/billing/sync.py", Function: "sync_customer_tier", Lineno: 74, InApp: &inAppTrue, ContextLine: "tier = data['subscription_tier']", PreContext: []string{"def sync_customer_tier(data: dict):", "    validate_payload(data)"}, PostContext: []string{"    update_redis_cache(tier)", "    return True"}},
							},
						},
					},
				},
			},
			Breadcrumbs: &ingest.SentryBreadcrumbPayload{
				Values: []ingest.SentryBreadcrumb{
					{Category: "celery", Message: "Received task: billing.sync_customer_tier[441]", Level: "info"},
				},
			},
		}

	default: // react_type_error
		return &ingest.SentryEvent{
			EventID:     eventID,
			Timestamp:   ingest.FlexibleTime{Time: now},
			Platform:    "javascript",
			Level:       "error",
			Release:     "frontend@2.8.0",
			Environment: "production",
			User: &ingest.SentryUser{
				ID:        "usr_88214",
				Email:     "alex@example.com",
				IPAddress: "203.0.113.195",
			},
			Exception: &ingest.SentryExceptionPayload{
				Values: []ingest.SentryException{
					{
						Type:  "TypeError",
						Value: "Cannot read properties of undefined (reading 'calculateTotal')",
						Stacktrace: &ingest.SentryStacktrace{
							Frames: []ingest.SentryFrame{
								{Filename: "node_modules/react-dom/client.js", Function: "dispatchDiscreteEvent", Lineno: 48, InApp: &inAppFalse},
								{Filename: "src/components/CheckoutModal.tsx", Function: "handlePayClick", Lineno: 142, Colno: 18, InApp: &inAppTrue, ContextLine: "  const total = cart.items.calculateTotal();", PreContext: []string{"const handlePayClick = async () => {", "  setIsSubmitting(true);"}, PostContext: []string{"  await submitOrder(total);", "};"}},
								{Filename: "src/hooks/useCart.ts", Function: "useCart", Lineno: 36, Colno: 9, InApp: &inAppTrue},
							},
						},
					},
				},
			},
			Breadcrumbs: &ingest.SentryBreadcrumbPayload{
				Values: []ingest.SentryBreadcrumb{
					{Category: "navigation", Message: "from: /cart to: /checkout", Level: "info"},
					{Category: "ui.click", Message: "button#select-payment-method", Level: "info"},
					{Category: "xhr", Message: "GET /api/v1/discounts/promo-sale [200]", Level: "info"},
					{Category: "ui.click", Message: "button#submit-order", Level: "info"},
				},
			},
			Tags: map[string]string{
				"browser": "Chrome 129.0",
				"os":      "macOS 15.0",
			},
		}
	}
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
