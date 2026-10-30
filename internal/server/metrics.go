package server

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/storage"
)

var startTime = time.Now().UTC()

// MetricsHandler serves standard Prometheus exposition format metrics.
type MetricsHandler struct {
	db *storage.DB
}

// NewMetricsHandler creates a new Prometheus metrics handler.
func NewMetricsHandler(db *storage.DB) *MetricsHandler {
	return &MetricsHandler{db: db}
}

// ServeHTTP exports real-time system and application gauges for Grafana/Prometheus.
func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	summary, _ := h.db.GetStatsSummary(0)
	totalEvents := int64(0)
	unresolved := int64(0)
	projects := int64(1)
	events24h := int64(0)

	if summary != nil {
		totalEvents = summary.TotalEvents
		unresolved = summary.UnresolvedIssues
		projects = summary.TotalProjects
		events24h = summary.Events24h
	}

	uptime := time.Since(startTime).Seconds()

	fmt.Fprintf(w, "# HELP errpulse_uptime_seconds Total runtime of ErrPulse daemon in seconds.\n")
	fmt.Fprintf(w, "# TYPE errpulse_uptime_seconds gauge\n")
	fmt.Fprintf(w, "errpulse_uptime_seconds %.2f\n\n", uptime)

	fmt.Fprintf(w, "# HELP errpulse_events_total Total crash events ingested since project inception.\n")
	fmt.Fprintf(w, "# TYPE errpulse_events_total counter\n")
	fmt.Fprintf(w, "errpulse_events_total %d\n\n", totalEvents)

	fmt.Fprintf(w, "# HELP errpulse_events_24h Total crash events ingested in the last 24 hours.\n")
	fmt.Fprintf(w, "# TYPE errpulse_events_24h gauge\n")
	fmt.Fprintf(w, "errpulse_events_24h %d\n\n", events24h)

	fmt.Fprintf(w, "# HELP errpulse_issues_unresolved Current count of active unresolved issues.\n")
	fmt.Fprintf(w, "# TYPE errpulse_issues_unresolved gauge\n")
	fmt.Fprintf(w, "errpulse_issues_unresolved %d\n\n", unresolved)

	fmt.Fprintf(w, "# HELP errpulse_projects_total Total monitored projects.\n")
	fmt.Fprintf(w, "# TYPE errpulse_projects_total gauge\n")
	fmt.Fprintf(w, "errpulse_projects_total %d\n\n", projects)

	fmt.Fprintf(w, "# HELP errpulse_memory_alloc_bytes Bytes of allocated heap memory.\n")
	fmt.Fprintf(w, "# TYPE errpulse_memory_alloc_bytes gauge\n")
	fmt.Fprintf(w, "errpulse_memory_alloc_bytes %d\n\n", m.Alloc)

	fmt.Fprintf(w, "# HELP errpulse_memory_sys_bytes Total bytes of memory obtained from the OS.\n")
	fmt.Fprintf(w, "# TYPE errpulse_memory_sys_bytes gauge\n")
	fmt.Fprintf(w, "errpulse_memory_sys_bytes %d\n\n", m.Sys)

	fmt.Fprintf(w, "# HELP errpulse_goroutines_count Number of active Go goroutines.\n")
	fmt.Fprintf(w, "# TYPE errpulse_goroutines_count gauge\n")
	fmt.Fprintf(w, "errpulse_goroutines_count %d\n\n", runtime.NumGoroutine())

	fmt.Fprintf(w, "# HELP errpulse_gc_cycles_total Total garbage collection cycles completed.\n")
	fmt.Fprintf(w, "# TYPE errpulse_gc_cycles_total counter\n")
	fmt.Fprintf(w, "errpulse_gc_cycles_total %d\n", m.NumGC)
}
