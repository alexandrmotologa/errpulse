package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alexandrmotologa/errpulse/internal/alerts"
	"github.com/alexandrmotologa/errpulse/internal/ingest"
	"github.com/alexandrmotologa/errpulse/internal/sourcemap"
	"github.com/alexandrmotologa/errpulse/internal/server"
	"github.com/alexandrmotologa/errpulse/internal/storage"
)

const (
	Version   = "1.0.0"
	BannerArt = `
  ______          _____       _          
 |  ____|        |  __ \     | |         
 | |__   _ __ _ _| |__) |   _| |___  ___ 
 |  __| | '__| '__|  ___/ | | | / __|/ _ \
 | |____| |  | |  | |   | |_| | \__ \  __/
 |______|_|  |_|  |_|    \__,_|_|___/\___|
  Sentry-Compatible Crash Daemon (< 30MB RAM)
`
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		cmdServe(os.Args[2:])
	case "seed-demo":
		cmdSeedDemo(os.Args[2:])
	case "version":
		fmt.Printf("ErrPulse v%s\n", Version)
	case "help", "-h", "--help":
		printUsage()
	default:
		// If flags passed directly e.g. errpulse --port 9988
		if strings.HasPrefix(command, "-") {
			cmdServe(os.Args[1:])
		} else {
			fmt.Printf("Unknown command: %s\n\n", command)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Print(BannerArt)
	fmt.Printf("\nVersion: %s\n\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  errpulse serve [flags]       Start the ingestion daemon and web studio")
	fmt.Println("  errpulse seed-demo [flags]   Inject realistic sample errors for evaluation")
	fmt.Println("  errpulse version             Display version information")
	fmt.Println("\nFlags for 'serve':")
	fmt.Println("  --port <number>        HTTP port to listen on (default: 9988)")
	fmt.Println("  --host <string>        Network interface to bind (default: 0.0.0.0)")
	fmt.Println("  --db <path>            SQLite database file path (default: ./errpulse.db)")
	fmt.Println("  --retention-days <int> Event retention in days (default: 30)")
	fmt.Println("  --demo                 Populate realistic sample crashes on boot")
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9988, "HTTP port to listen on")
	host := fs.String("host", "0.0.0.0", "Network interface to bind")
	dbPath := fs.String("db", "./errpulse.db", "SQLite database file path")
	retentionDays := fs.Int("retention-days", 30, "Event retention in days")
	demo := fs.Bool("demo", false, "Pre-seed realistic sample errors on startup")

	_ = fs.Parse(args)

	fmt.Print(BannerArt)
	log.Printf("[ErrPulse] Initializing SQLite database in WAL mode at %s", *dbPath)

	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("[ErrPulse] Database initialization failed: %v", err)
	}
	defer db.Close()

	baseURL := fmt.Sprintf("http://localhost:%d", *port)
	if *host != "0.0.0.0" && *host != "" {
		baseURL = fmt.Sprintf("http://%s:%d", *host, *port)
	}

	// Initialize pipeline subsystems
	resolver := sourcemap.NewResolver(db)
	sseHub := server.NewSSEHub()
	dispatcher := alerts.NewDispatcher(db, baseURL)
	defer dispatcher.Close()

	ingestor := ingest.NewIngestHandler(db, resolver, sseHub, dispatcher)
	apiHandler := server.NewAPIHandler(db, resolver, dispatcher, ingestor, sseHub, baseURL)
	sentryCLIHandler := server.NewSentryCLICompatHandler(db, resolver)
	spaHandler := server.NewSPAHandler()

	// Seed demo crashes if requested or if database has 0 events
	if *demo {
		seedDemoEvents(ingestor, db)
	} else {
		summary, _ := db.GetStatsSummary(1)
		if summary != nil && summary.TotalEvents == 0 {
			seedDemoEvents(ingestor, db)
		}
	}

	// Start background data retention cleaner
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db.StartPruningJob(ctx, 24*time.Hour, *retentionDays)

	// Setup HTTP routing multiplexer
	mux := http.NewServeMux()

	// 1. Sentry Ingestion Endpoints (standard protocol)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/envelope/") || strings.HasSuffix(path, "/envelope") {
			ingestor.HandleEnvelope(w, r)
			return
		}
		if strings.HasSuffix(path, "/store/") || strings.HasSuffix(path, "/store") {
			ingestor.HandleStore(w, r)
			return
		}
		if strings.HasSuffix(path, "/security/") || strings.HasSuffix(path, "/security") {
			ingestor.HandleSecurity(w, r)
			return
		}

		// Sentry CLI release file endpoints (/api/0/...)
		if strings.HasPrefix(path, "/api/0/") {
			sentryCLIHandler.ServeHTTP(w, r)
			return
		}

		// REST API (/api/v1/...)
		if strings.HasPrefix(path, "/api/v1/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	})

	// 2. Real-time Server-Sent Events stream
	mux.Handle("/api/v1/stream", sseHub)

	// 3. Health check & Prometheus metrics endpoints
	mux.Handle("/metrics", server.NewMetricsHandler(db))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"version": Version,
			"time":    time.Now().UTC(),
		})
	})

	// 4. Incident Management Web Studio (Embedded React 19 UI)
	mux.Handle("/", spaHandler)

	addr := fmt.Sprintf("%s:%d", *host, *port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown channel
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[ErrPulse] Incident Studio running at %s", baseURL)
		log.Printf("[ErrPulse] Default Sentry DSN: http://errpulse_default_key@localhost:%d/1", *port)
		log.Printf("[ErrPulse] Daemon listening on %s (Memory target < 30MB RAM)", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[ErrPulse] HTTP server failure: %v", err)
		}
	}()

	<-shutdownChan
	log.Println("[ErrPulse] Gracefully shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ErrPulse] Shutdown warning: %v", err)
	}

	log.Println("[ErrPulse] Daemon stopped.")
}

func cmdSeedDemo(args []string) {
	fs := flag.NewFlagSet("seed-demo", flag.ExitOnError)
	dbPath := fs.String("db", "./errpulse.db", "SQLite database file path")
	_ = fs.Parse(args)

	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("Failed opening database: %v", err)
	}
	defer db.Close()

	resolver := sourcemap.NewResolver(db)
	sseHub := server.NewSSEHub()
	ingestor := ingest.NewIngestHandler(db, resolver, sseHub, nil)

	seedDemoEvents(ingestor, db)
	log.Println("[ErrPulse] Demo events successfully seeded into database.")
}

type fakeRespWriter struct {
	header http.Header
}

func (f *fakeRespWriter) Header() http.Header         { return f.header }
func (f *fakeRespWriter) Write(b []byte) (int, error) { return len(b), nil }
func (f *fakeRespWriter) WriteHeader(status int)      {}

func seedDemoEvents(ingestor *ingest.IngestHandler, db *storage.DB) {
	log.Println("[ErrPulse] Seeding realistic sample exceptions across frontend, backend, and workers...")

	// Seed release health sessions
	_ = db.RecordSession(1, "webapp@1.4.2", true, "ok", "usr_101")
	_ = db.RecordSession(1, "webapp@1.4.2", true, "ok", "usr_102")
	_ = db.RecordSession(1, "webapp@1.4.2", true, "crashed", "usr_103")
	_ = db.RecordSession(1, "webapp@1.4.2", true, "ok", "usr_104")
	_ = db.RecordSession(1, "webapp@1.4.2", true, "ok", "usr_105")
	_ = db.RecordSession(1, "checkout-service@1.4.2", true, "ok", "srv_node_1")
	_ = db.RecordSession(1, "checkout-service@1.4.2", true, "ok", "srv_node_2")
	_ = db.RecordSession(1, "checkout-service@1.4.2", true, "crashed", "srv_node_3")

	scenarios := []string{
		"react_type_error",
		"go_panic",
		"python_key_error",
	}

	for _, s := range scenarios {
		for i := 0; i < 3; i++ {
			event := server.GenerateSyntheticEvent(s)
			event.EventID = fmt.Sprintf("%d%d", time.Now().UnixNano(), i)
			if len(event.EventID) > 32 {
				event.EventID = event.EventID[:32]
			}
			bodyBytes, err := json.Marshal(event)
			if err != nil {
				continue
			}

			req, _ := http.NewRequest(http.MethodPost, "/api/1/store/", strings.NewReader(string(bodyBytes)))
			req.Header.Set("Content-Type", "application/json")
			w := &fakeRespWriter{header: make(http.Header)}
			ingestor.HandleStore(w, req)
		}
	}
}
