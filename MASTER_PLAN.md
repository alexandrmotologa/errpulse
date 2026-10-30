# ErrPulse: Master Implementation Plan

Drop-in Sentry-compatible single-binary error and exception tracking daemon under 30MB RAM.

ErrPulse implements the standard Sentry envelope ingestion protocol (compatible with official `@sentry/browser`, `@sentry/node`, and `sentry-sdk` clients), de-minifies production stack traces using source maps, groups exceptions with deterministic fingerprinting, and provides an embedded React 19 incident management studio.

---

## 1. System Architecture

Cloud crash monitoring currently presents two main hurdles:
1. Self-hosting Sentry requires over 15 containers (ClickHouse, Kafka, Snuba, Relay, Redis, PostgreSQL, Memcached) and 16GB of RAM.
2. Sending production error traces and customer context to commercial cloud SaaS risks leaking sensitive database parameters and personal identifiable information.

ErrPulse solves this by providing a single static binary in Go 1.23 with embedded SQLite WAL storage and an embedded React 19 single page application:
- Standard Sentry Protocol Support: Handles `/api/{project_id}/envelope/`, `/api/{project_id}/store/`, and `/api/{project_id}/security/`. Teams swap their DSN string without changing client logging libraries.
- Memory Target Under 30MB RAM: Operates on a minimal memory footprint using SQLite in write-ahead log (WAL) mode with tuned page caches.
- Pure Go VLQ Source Map Decoder: Parses Base64 VLQ mappings directly in Go, matching minified error frames to original TypeScript source lines and embedding surrounding context.
- Deterministic Exception Fingerprinting: Groups raw crash events into deduplicated issues using structural hashing on sanitized exception types and in-app stack frames.
- Multi-Channel Incident Alerts: Sends notifications to Slack, Discord, Telegram, or custom webhooks when an issue is first seen or regresses.
- Embedded Incident Studio: Serves a complete React 19 dashboard directly from the Go binary via `embed.FS` on port 9988.

```
+-----------------------------------------------------------------------------------+
|                                     ERRPULSE                                      |
+-----------------------------------------+-----------------------------------------+
                                          |
        +---------------------------------+---------------------------------+
        |                                                                   |
+-------v-------------------------+                 +-----------------------v-------+
| Sentry Protocol Ingestion Engine|                 | Source Map De-Minifier        |
| * /api/{project_id}/envelope/   |                 | * VLQ Source Map Decoder      |
| * /api/{project_id}/store/      |                 | * Remaps bundle:1:3402 to TS  |
| * gzip / deflate unpack         |                 | * Sentry CLI & API upload     |
+-------+-------------------------+                 +-----------------------+-------+
        |                                                                   |
        +---------------------------------+---------------------------------+
                                          |
                        +-----------------v-----------------+
                        | Fingerprinting & Deduplication    |
                        | * Dynamic Token Normalizer        |
                        | * Vendor Frame Stripping          |
                        | * SHA-256 Structural Fingerprint  |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        | Embedded Storage & Alerting       |
                        | * SQLite WAL (< 30MB RAM)         |
                        | * Hourly Frequency Buckets        |
                        | * Slack / Discord / Telegram      |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        | Incident Management Studio        |
                        | * Real-time SSE Event Stream      |
                        | * De-minified Stack Viewer        |
                        | * Breadcrumb Timeline (Port 9988) |
                        +-----------------------------------+
```

---

## 2. Directory Layout

```
errpulse/
├── cmd/
│   └── errpulse/
│       └── main.go               # CLI entrypoint, flag parsing, daemon launcher
├── internal/
│   ├── alerts/
│   │   ├── dispatcher.go         # Alert queue worker and regression detector
│   │   ├── discord.go            # Discord webhook embed formatter
│   │   ├── slack.go              # Slack Block Kit JSON builder
│   │   ├── telegram.go           # Telegram HTML/Markdown bot broadcaster
│   │   ├── throttle.go           # Alert rate-limiter and spike protection
│   │   └── webhook.go            # Generic HTTP JSON webhook sender
│   ├── grouping/
│   │   ├── cleaner.go            # In-app frame classifier (filters vendor paths)
│   │   ├── fingerprint.go        # SHA-256 issue grouping algorithm
│   │   └── normalizer.go         # Sanitizes hex, UUIDs, numbers, and timestamps
│   ├── ingest/
│   │   ├── auth.go               # DSN and X-Sentry-Auth header parser
│   │   ├── envelope.go           # Multi-part Sentry envelope stream parser
│   │   ├── handler.go            # Ingestion HTTP endpoints (/envelope, /store)
│   │   ├── sanitizer.go          # PII and credential data scrubber
│   │   └── model.go              # Event, Exception, Frame, Breadcrumb schemas
│   ├── models/
│   │   └── sentry.go             # Protocol type definitions (decoupled)
│   ├── server/
│   │   ├── api.go                # REST API for issues, projects, and alerts
│   │   ├── metrics.go            # Prometheus exposition endpoint (/metrics)
│   │   ├── sentry_cli_compat.go  # Sentry CLI release file upload compatibility
│   │   ├── sse.go                # Server-Sent Events real-time stream hub
│   │   └── static.go             # Embedded React 19 UI handler (embed.FS)
│   ├── sourcemap/
│   │   ├── resolver.go           # Stack trace remapper with source lines
│   │   ├── store.go              # Release artifact and .map file database
│   │   └── vlq.go                # Pure Go Base64 VLQ source map v3 decoder
│   └── storage/
│       ├── migrations.go         # SQLite schema initialization
│       ├── pruner.go             # Background data retention cleaner
│       ├── repo_alerts.go        # Notification settings repository
│       ├── repo_artifacts.go     # Release artifact and .map storage
│       ├── repo_events.go        # Raw event storage and occurrence retrieval
│       ├── repo_issues.go        # Issue aggregation, search DSL, and stats
│       ├── repo_projects.go      # Project credentials and DSN management
│       ├── repo_sessions.go      # Session tracking and Crash-Free Rate calculations
│       └── sqlite.go             # SQLite connection pool with WAL pragma
├── web/
│   ├── index.html
│   ├── package.json
│   ├── postcss.config.js
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── src/
│       ├── App.tsx               # Main application container and layout
│       ├── index.css             # Tailwind base and theme tokens
│       ├── main.tsx              # React 19 root bootstrap
│       ├── components/
│       │   ├── AlertSettings.tsx     # Webhook destination manager
│       │   ├── BreadcrumbView.tsx    # Chronological pre-crash user trail
│       │   ├── DsnHelper.tsx         # Copy-paste setup snippets for SDKs
│       │   ├── EventSimulator.tsx    # One-click synthetic error injector
│       │   ├── IssueDetails.tsx      # Full issue inspection and occurrence navigation
│       │   ├── IssueTable.tsx        # Filterable issue list with sparklines
│       │   ├── ReleaseManager.tsx    # Source map manager and release health cards
│       │   ├── Sparkline.tsx         # SVG 24-hour activity sparkline
│       │   └── StackTraceViewer.tsx  # Interactive de-minified source viewer
│       ├── hooks/
│       │   ├── useErrPulseStream.ts  # SSE real-time connection hook
│       │   └── useIssues.ts          # Issue querying and mutation state
│       ├── utils/
│       │   └── sound.ts              # Web Audio API tactile incident chimes
│       └── types/
│           └── index.ts              # TypeScript domain interfaces
├── docs/
│   └── ARCHITECTURE.md           # Ingestion protocol and VLQ design details
├── go.mod
├── go.sum
└── README.md
```

---

## 3. Detailed Implementation Tasks

### Task 1: Repository Setup and Portfolio Synchronization
- Initialize Git repository in `B:\workgit\errpulse`.
- Create GitHub public repository `alexandrmotologa/errpulse`.
- Set repository description and strictly couple 12 targeted topics covering primary domain, protocols, patterns, and stack.
- Register ErrPulse in `B:\workgit\mtlg-site\projects.json` and `B:\workgit\mtlglabs-space\projects.json` with status `dev`.
- Rebuild portfolio sites with `sync.js`, verify with `compare_projects.js`, and commit/push.

### Task 2: Go Backend Foundations and SQLite WAL Storage
- Initialize Go module `github.com/alexandrmotologa/errpulse`.
- Implement `internal/storage/sqlite.go`:
  - Set pragmas: `journal_mode=WAL`, `synchronous=NORMAL`, `busy_timeout=5000`, `cache_size=-2000`.
  - Schema migrations for `projects`, `issues`, `events`, `hourly_stats`, `release_artifacts`, `alert_configs`, and `alert_logs`.
- Implement repositories for project CRUD, issue upsert, event queries, and retention cleanup.
- Add unit tests verifying schema creation, concurrent WAL writes, and issue aggregation.

### Task 3: Sentry Protocol Ingestion Engine
- Implement `internal/ingest/auth.go`:
  - Support `X-Sentry-Auth` headers and query string params (`sentry_key`, `sentry_version`).
  - Validate project ID and public key against database.
- Implement `internal/ingest/envelope.go`:
  - Read multi-part newline-delimited envelope stream.
  - Automatically decompress gzip, zlib, and deflate bodies.
  - Parse `event`, `session`, and `client_report` items.
- Implement `internal/ingest/handler.go`:
  - Support `/api/{project_id}/envelope/`, `/api/{project_id}/store/`, and `/api/{project_id}/security/`.
  - Return Sentry-compliant HTTP 200 JSON with event ID.

### Task 4: Pure Go VLQ Source Map Decoder and Stack Remapper
- Implement `internal/sourcemap/vlq.go`:
  - Base64 VLQ decoder algorithm.
  - Parse Source Map v3 JSON format (`mappings`, `sources`, `sourcesContent`, `names`).
  - Binary search algorithm on segment entries to find original source file, line, and column.
- Implement `internal/sourcemap/resolver.go`:
  - Extract frames from incoming stack traces.
  - Lookup corresponding `.map` artifact for matching release and filename.
  - Remap minified locations and extract 5 lines before and after from embedded `sourcesContent`.

### Task 5: Fingerprinting, Deduplication, and Regression Detection
- Implement `internal/grouping/normalizer.go`:
  - Scrub dynamic variables: UUIDs, memory addresses, timestamps, hexadecimal tokens, and query strings.
- Implement `internal/grouping/cleaner.go`:
  - Mark vendor frames as non-app (`node_modules`, standard libraries, bundler wrappers).
- Implement `internal/grouping/fingerprint.go`:
  - Compute SHA-256 hash using exception type and top 3 in-app frames.
  - Detect regression if an issue marked `RESOLVED` reappears.

### Task 6: Multi-Channel Alert Dispatcher
- Implement `internal/alerts/dispatcher.go`:
  - Background worker consuming alert jobs.
  - Trigger conditions: `NEW_ISSUE` and `REGRESSION`.
- Implement Slack Block Kit builder in `internal/alerts/slack.go`.
- Implement Discord Embed builder in `internal/alerts/discord.go`.
- Implement Telegram HTML/Markdown bot sender in `internal/alerts/telegram.go`.
- Implement generic HTTP JSON webhook caller in `internal/alerts/webhook.go`.

### Task 7: Sentry CLI Compatibility and SSE Real-Time Streaming
- Implement Sentry CLI release upload endpoint `/api/0/organizations/{org}/releases/{version}/files/`.
- Implement `/api/v1/stream` endpoint with Server-Sent Events broadcasting incoming events and issue count updates.
- Implement REST API endpoints for issue filtering, status transitions, project keys, and alert configurations.

### Task 8: React 19 Incident Management Studio
- Scaffold frontend in `web/` with Vite, React 19, TypeScript, and Tailwind CSS.
- Build components:
  - IssueTable with search, status filters, environment chips, and SVG sparklines.
  - StackTraceViewer with expandable source code contexts, syntax styling, and in-app toggles.
  - BreadcrumbView displaying user click trails, console logs, and HTTP network requests.
  - ReleaseManager supporting drag-and-drop `.map` file uploads.
  - AlertSettings with immediate live webhook test dispatch.
  - DsnHelper with copy-to-clipboard code snippets for React, Node.js, Python, and Go.
  - EventSimulator for one-click synthetic error generation.
- Connect to SSE stream for live updates.
- Embed production Vite build assets into Go binary using `embed.FS`.

### Task 9: CLI Entrypoint, Data Retention, and Integration Verification
- Implement `cmd/errpulse/main.go` supporting:
  - `serve`: Run daemon on `--port 9988` with `--db ./errpulse.db` and `--retention-days 30`.
  - `seed-demo`: Populate database with realistic sample errors for instant inspection.
- End-to-end integration tests:
  - Ingest real Sentry envelopes via HTTP.
  - Verify VLQ source map de-minification on sample minified bundle.
  - Check issue deduplication across 100 duplicate events.
  - Verify webhook notification payload formatting.
  - Check memory footprint under load (< 30MB RAM).

---

## 4. Architectural Enhancements Included

1. **PII and Sensitive Data Sanitizer (`internal/ingest/sanitizer.go`)**:
   - Automated redaction of passwords, tokens, API keys, JWTs, credit card numbers, and authorization headers from event payloads and extra context before SQLite writes.

2. **Session and Release Health Tracking (`internal/storage/repo_sessions.go`)**:
   - Ingests Sentry session envelope items (`{"type": "session"}`) and tracks session runs, crashes, and unique users per release.
   - Computes real-time Crash-Free Session Rate % and Crash-Free User Rate % exposed via `GET /api/v1/releases/health`.

3. **Search Syntax DSL and Multi-Environment Triage (`internal/storage/repo_issues.go`)**:
   - Parses search tokens: `is:unresolved`, `is:resolved`, `is:ignored`, `env:<name>`, `release:<tag>`, `type:<error_type>`, plus free-text matching.
   - Front-end dropdown filter for multi-environment isolation (`all`, `production`, `staging`, `development`).

4. **Historical Occurrence Browser (`internal/storage/repo_events.go`)**:
   - Allows users to step through every occurrence of an issue (`Event 1 of 42`) with previous/next controls, surfacing temporal shifts in breadcrumbs, user accounts, and request headers.

5. **1-Click GitHub Issue Creator (`web/src/components/IssueDetails.tsx`)**:
   - Generates pre-populated markdown bug reports containing issue titles, culprit files, system tags, and formatted stack traces with a single click.

6. **Alert Throttling and Crash Spike Protection (`internal/alerts/throttle.go`)**:
   - Rate-limits incident notifications to prevent alert storms (configurable cooldown window).
   - Flags abnormal crash frequency spikes (>20 events per minute) with critical priority banners.

7. **Prometheus Metrics Exposition (`internal/server/metrics.go`)**:
   - Built-in `/metrics` endpoint exporting ingestion counters, active issue counts, database storage size in bytes, and uptime gauges.

8. **Tactile Web Audio Alerts (`web/src/utils/sound.ts`)**:
   - Optional procedural audio chimes synthesized directly in the browser via the Web Audio API for incoming crashes and regressions without external media files.

9. **Real-Time SSE Stream (`internal/server/sse.go`)**:
   - Live updates push directly to the browser when new errors arrive without manual refresh.

10. **Sentry CLI Compatibility (`internal/server/sentry_cli_compat.go`)**:
    - Standard CI/CD release workflows (`sentry-cli releases files upload-sourcemaps`) work directly with ErrPulse.

11. **Embedded Sources Support (`internal/sourcemap/vlq.go`)**:
    - Source maps with embedded `sourcesContent` render original source code without requiring local file access.

12. **Automatic Data Retention Pruner (`internal/storage/pruner.go`)**:
    - Daily background routine deletes raw events older than the configured threshold, keeping SQLite compact.

13. **In-Studio Error Simulator (`internal/server/api.go`)**:
    - Allows immediate testing of the full pipeline (envelope intake, stack trace decoding, and alerting) from the web UI.
