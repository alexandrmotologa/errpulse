package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexandrmotologa/errpulse/internal/alerts"
	"github.com/alexandrmotologa/errpulse/internal/ingest"
	"github.com/alexandrmotologa/errpulse/internal/sourcemap"
	"github.com/alexandrmotologa/errpulse/internal/storage"
)

func setupTestServer(t *testing.T) (*storage.DB, http.Handler) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed opening memory db: %v", err)
	}

	resolver := sourcemap.NewResolver(db)
	sseHub := NewSSEHub()
	dispatcher := alerts.NewDispatcher(db, "http://localhost:9988")
	ingestor := ingest.NewIngestHandler(db, resolver, sseHub, dispatcher)
	apiHandler := NewAPIHandler(db, resolver, dispatcher, ingestor, sseHub, "http://localhost:9988")
	sentryCLIHandler := NewSentryCLICompatHandler(db, resolver)
	spaHandler := NewSPAHandler()

	mux := http.NewServeMux()

	// Register ingestion endpoints
	mux.HandleFunc("/api/1/envelope/", ingestor.HandleEnvelope)
	mux.HandleFunc("/api/1/store/", ingestor.HandleStore)
	mux.HandleFunc("/api/1/security/", ingestor.HandleSecurity)

	// Register SSE stream
	mux.Handle("/api/v1/stream", sseHub)

	// Register Prometheus metrics
	mux.Handle("/metrics", NewMetricsHandler(db))

	// Register REST API
	mux.Handle("/api/v1/", apiHandler)

	// Register Sentry CLI compat
	mux.Handle("/api/0/", sentryCLIHandler)

	// Register SPA fallback
	mux.Handle("/", spaHandler)

	return db, mux
}

func TestEnvelopeIngestionAndTriage(t *testing.T) {
	db, handler := setupTestServer(t)
	defer db.Close()

	envelopePayload := `{"event_id":"4f29a0086202475abc81540eeea03e62","sent_at":"2026-10-30T17:42:00Z"}
{"type":"event","content_type":"application/json"}
{"event_id":"4f29a0086202475abc81540eeea03e62","platform":"javascript","environment":"production","release":"webapp@1.0.0","exception":{"values":[{"type":"TypeError","value":"Failed to read property 'user'","stacktrace":{"frames":[{"filename":"app.js","lineno":10}]}}]}}
`

	t.Log("Step 1: Sending envelope...")
	// 1. Send Sentry Envelope
	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/", strings.NewReader(envelopePayload))
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=errpulse_default_key, sentry_version=7")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	t.Logf("Step 1 done: status %d, body: %s", rec.Code, rec.Body.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["id"] != "4f29a0086202475abc81540eeea03e62" {
		t.Errorf("Expected event ID 4f29a0086202475abc81540eeea03e62, got %s", resp["id"])
	}

	t.Log("Step 2: Querying stats...")
	// 2. Query Stats API
	reqStats := httptest.NewRequest(http.MethodGet, "/api/v1/stats?project_id=1", nil)
	recStats := httptest.NewRecorder()
	handler.ServeHTTP(recStats, reqStats)
	t.Logf("Step 2 done: status %d", recStats.Code)

	if recStats.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 for stats, got %d", recStats.Code)
	}

	var stats storage.StatsSummary
	_ = json.Unmarshal(recStats.Body.Bytes(), &stats)
	if stats.TotalEvents != 1 {
		t.Errorf("Expected total events 1, got %d", stats.TotalEvents)
	}
	if stats.UnresolvedIssues != 1 {
		t.Errorf("Expected unresolved issues 1, got %d", stats.UnresolvedIssues)
	}

	t.Log("Step 3: Querying issues...")
	// 3. Query Issues List API
	reqIssues := httptest.NewRequest(http.MethodGet, "/api/v1/issues?project_id=1&status=unresolved", nil)
	recIssues := httptest.NewRecorder()
	handler.ServeHTTP(recIssues, reqIssues)
	t.Logf("Step 3 done: status %d, body: %s", recIssues.Code, recIssues.Body.String())

	var issuesList struct {
		Issues []storage.Issue `json:"issues"`
		Total  int64           `json:"total"`
	}
	_ = json.Unmarshal(recIssues.Body.Bytes(), &issuesList)
	if len(issuesList.Issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issuesList.Issues))
	}
	issue := issuesList.Issues[0]
	if issue.Type != "TypeError" {
		t.Errorf("Expected issue type 'TypeError', got '%s'", issue.Type)
	}

	t.Log("Step 4: Patching issue...")
	// 4. Update Status to Resolved
	patchBody := `{"status":"resolved"}`
	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/issues/1", strings.NewReader(patchBody))
	reqPatch.Header.Set("Content-Type", "application/json")
	recPatch := httptest.NewRecorder()
	handler.ServeHTTP(recPatch, reqPatch)
	t.Logf("Step 4 done: status %d", recPatch.Code)

	if recPatch.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 on patch, got %d", recPatch.Code)
	}

	t.Log("Step 5: Simulating error...")
	// 5. Test Synthetic Error Simulator
	simBody := `{"scenario":"go_panic"}`
	reqSim := httptest.NewRequest(http.MethodPost, "/api/v1/simulate", strings.NewReader(simBody))
	reqSim.Header.Set("Content-Type", "application/json")
	recSim := httptest.NewRecorder()
	handler.ServeHTTP(recSim, reqSim)
	t.Logf("Step 5 done: status %d", recSim.Code)

	if recSim.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 on simulate, got %d: %s", recSim.Code, recSim.Body.String())
	}
}

func TestSentryCLIUploadCompat(t *testing.T) {
	db, handler := setupTestServer(t)
	defer db.Close()

	// 1. Create Release: POST /api/0/organizations/default/releases/
	relReqBody := `{"version":"webapp@2.0.0"}`
	reqRel := httptest.NewRequest(http.MethodPost, "/api/0/organizations/default/releases/", strings.NewReader(relReqBody))
	reqRel.Header.Set("Content-Type", "application/json")
	recRel := httptest.NewRecorder()
	handler.ServeHTTP(recRel, reqRel)

	if recRel.Code != http.StatusCreated {
		t.Fatalf("Expected HTTP 201 on create release, got %d", recRel.Code)
	}

	// 2. Upload Source Map: POST /api/0/organizations/default/releases/webapp@2.0.0/files/
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("name", "~/assets/bundle.js.map")
	part, _ := writer.CreateFormFile("file", "bundle.js.map")
	_, _ = part.Write([]byte(`{"version":3,"sources":["src/index.ts"],"mappings":"AAAA"}`))
	_ = writer.Close()

	reqUpload := httptest.NewRequest(http.MethodPost, "/api/0/organizations/default/releases/webapp@2.0.0/files/", &body)
	reqUpload.Header.Set("Content-Type", writer.FormDataContentType())
	recUpload := httptest.NewRecorder()
	handler.ServeHTTP(recUpload, reqUpload)

	if recUpload.Code != http.StatusCreated {
		t.Fatalf("Expected HTTP 201 on file upload, got %d: %s", recUpload.Code, recUpload.Body.String())
	}

	// 3. List uploaded files
	reqList := httptest.NewRequest(http.MethodGet, "/api/0/organizations/default/releases/webapp@2.0.0/files/", nil)
	recList := httptest.NewRecorder()
	handler.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 on files list, got %d", recList.Code)
	}

	var files []map[string]interface{}
	_ = json.Unmarshal(recList.Body.Bytes(), &files)
	if len(files) != 1 {
		t.Fatalf("Expected 1 uploaded file, got %d", len(files))
	}
	if files[0]["name"] != "~/assets/bundle.js.map" {
		t.Errorf("Expected filename '~/assets/bundle.js.map', got %v", files[0]["name"])
	}
}

func TestSPAStaticFallback(t *testing.T) {
	db, handler := setupTestServer(t)
	defer db.Close()

	// GET /
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 for index.html, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "ErrPulse") {
		t.Errorf("Expected body to contain 'ErrPulse', got: %s", rec.Body.String())
	}

	// Deep SPA route: GET /issues/42
	reqDeep := httptest.NewRequest(http.MethodGet, "/issues/42", nil)
	recDeep := httptest.NewRecorder()
	handler.ServeHTTP(recDeep, reqDeep)

	if recDeep.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 for deep SPA route, got %d", recDeep.Code)
	}
	if !strings.Contains(recDeep.Body.String(), "<div id=\"root\"></div>") {
		t.Errorf("Expected index.html fallback for client-side routing")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	db, handler := setupTestServer(t)
	defer db.Close()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected HTTP 200 on /metrics, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "errpulse_uptime_seconds") {
		t.Errorf("Expected body to contain errpulse_uptime_seconds")
	}
	if !strings.Contains(body, "errpulse_events_total") {
		t.Errorf("Expected body to contain errpulse_events_total")
	}
	if !strings.Contains(body, "errpulse_memory_alloc_bytes") {
		t.Errorf("Expected body to contain errpulse_memory_alloc_bytes")
	}
}
