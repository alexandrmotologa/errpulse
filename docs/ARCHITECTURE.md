# ErrPulse Architecture and Protocol Specification

This document details the internal design, protocols, and data models powering ErrPulse.

---

## 1. Sentry Envelope Ingestion Protocol

Sentry SDKs send crash data encoded in the Sentry Envelope format, a streaming container protocol composed of newline-delimited JSON and binary chunks.

### 1.1 Ingestion Endpoints
- `POST /api/{project_id}/envelope/`: Modern envelope ingestion used by `@sentry/browser` v7+, `@sentry/node` v7+, `sentry-sdk` Python v1.0+, and `sentry-go`.
- `POST /api/{project_id}/store/`: Legacy single-event ingestion endpoint used by older SDKs.
- `POST /api/{project_id}/security/`: CSP (Content Security Policy) and Expect-CT violation reports.

### 1.2 Authentication
Authentication credentials may arrive in two formats:
1. `X-Sentry-Auth` Header:
   ```
   X-Sentry-Auth: Sentry sentry_version=7, sentry_client=sentry.javascript.browser/8.26.0, sentry_key=errpulse_default_key, sentry_secret=
   ```
2. Query Parameters (used when preflight CORS requests avoid custom headers):
   ```
   POST /api/1/envelope/?sentry_key=errpulse_default_key&sentry_version=7
   ```

ErrPulse validates the `sentry_key` against the target project before processing payload bytes.

### 1.3 Envelope Wire Format
An envelope consists of:
- **Envelope Header** (one line JSON, terminated by `\n`)
  ```json
  {"event_id":"9ec69572404f4357b988f57297e68406","sent_at":"2026-10-30T17:42:00Z","dsn":"http://errpulse_default_key@localhost:9988/1"}
  ```
- **Item Header** (one line JSON, terminated by `\n`)
  ```json
  {"type":"event","content_type":"application/json","length":1420}
  ```
- **Item Payload** (binary or text bytes matching `length`, followed by optional `\n`)
  ```json
  {
    "event_id": "9ec69572404f4357b988f57297e68406",
    "timestamp": "2026-10-30T17:41:59Z",
    "platform": "javascript",
    "level": "error",
    "environment": "production",
    "release": "webapp@1.2.0",
    "exception": {
      "values": [
        {
          "type": "TypeError",
          "value": "Cannot read properties of undefined (reading 'calculateTotal')",
          "stacktrace": {
            "frames": [
              {
                "filename": "http://localhost:3000/assets/index-D7h28v.js",
                "lineno": 1,
                "colno": 3402,
                "function": "t.checkout"
              }
            ]
          }
        }
      ]
    },
    "breadcrumbs": [
      {
        "type": "default",
        "category": "ui.click",
        "message": "button#submit-order",
        "timestamp": 1793382118
      }
    ]
  }
  ```

ErrPulse decompresses incoming payloads transparently if `Content-Encoding: gzip` or `Content-Encoding: deflate` is present.

---

## 2. Base64 VLQ Source Map Decoder

Modern web frontends minify code into single-line bundles. To make error traces actionable, ErrPulse parses Source Map v3 files directly in Go using a Base64 VLQ (Variable-Length Quantity) decoder.

### 2.1 VLQ Decoding Mathematics
Source Map v3 encodes integer coordinates using Base64 characters where each character represents 6 bits:
- **Bit 5 (Continuation bit)**: If set (1), another character follows in this integer sequence.
- **Bits 1-4 (Payload)**: Data bits shifted into position.
- **Bit 0 (Sign bit, in the first character only)**: If set (1), the resulting integer is negative.

Each mapping line is separated by a semicolon `;` (corresponding to each line of generated code). Inside each line, segments are separated by commas `,`.

Each segment contains 1, 4, or 5 fields:
1. `generated_col` (relative to previous segment in this generated line)
2. `sources_index` (relative to previous segment)
3. `original_line` (relative to previous segment)
4. `original_col` (relative to previous segment)
5. `names_index` (optional, relative to previous segment)

### 2.2 Segment Resolution and Context Extraction
When a minified stack frame arrives with `(lineno, colno)`:
1. ErrPulse locates the indexed lines in the source map.
2. Performs a binary search across the line segments to identify the segment whose `generated_col <= frame.colno`.
3. Computes the absolute target: `sources[sources_index]` at line `original_line` and column `original_col`.
4. If the source map includes `sourcesContent`, ErrPulse splits the stored original code by line and extracts:
   - `pre_context`: 5 lines preceding the crash.
   - `context_line`: The exact line of source code that failed.
   - `post_context`: 5 lines following the crash.

This reconstructs the developer's original TypeScript code directly inside the ErrPulse Web Studio without needing local source access.

---

## 3. Deterministic Exception Fingerprinting

High-volume production applications can throw thousands of identical crashes within minutes. ErrPulse clusters these into single trackable issues.

### 3.1 Frame Classification
Frames are classified into `in_app` and `vendor`:
- Paths containing `/node_modules/`, `/vendor/`, `/usr/lib/`, `/internal/runtime/`, or bundler prelude code (`webpack/bootstrap`, `vite/client`) are marked `in_app = false`.
- Repository code is marked `in_app = true`.

### 3.2 Token Normalization
Raw exception messages frequently contain dynamic data that would otherwise fragment identical issues into separate groups:
- Hexadecimal addresses: `0x7ffee12a4b80` -> `<HEX>`
- UUIDs: `4f29a008-6202-475a-bc81-540eeea03e62` -> `<UUID>`
- Unix timestamps and ISO dates: `2026-10-30T17:42:00Z` -> `<TIME>`
- Numeric identifiers: `/users/982142/orders` -> `/users/<NUM>/orders`

### 3.3 Fingerprint Hash Computation
1. Collect the normalized `ExceptionType` (e.g. `TypeError`, `SQLIntegrityConstraintViolationException`).
2. Extract the top 3 `in_app` stack frames (function name and stripped relative filename). If no in-app frames are present, use the top 3 available frames.
3. Concatenate these items with null byte separators.
4. Calculate `SHA-256` and truncate to a 32-character hexadecimal string.

```
Fingerprint = SHA256(ExceptionType + "\x00" + Frame1 + "\x00" + Frame2 + "\x00" + Frame3)
```

If an event payload provides an explicit `fingerprint` array from the client, that array is used directly instead of the automatic algorithm.

---

## 4. SQLite WAL Storage Optimization

ErrPulse stores data in an embedded SQLite database using tuned configuration settings to maintain a RAM footprint below 30MB while sustaining high write concurrency.

### 4.1 Connection Pragmas
```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA cache_size = -2000;
PRAGMA foreign_keys = ON;
PRAGMA temp_store = MEMORY;
```
- `journal_mode = WAL`: Allows concurrent readers while writes occur on a separate append-only write-ahead log.
- `cache_size = -2000`: Limits page cache memory usage to approximately 2MB per database handle.
- `synchronous = NORMAL`: Safely eliminates costly fsync operations on every write in WAL mode without risking corruption.

### 4.2 Relational Schema
- `projects`: `id`, `name`, `slug`, `public_key`, `secret_key`, `created_at`
- `issues`: `id`, `project_id`, `fingerprint`, `title`, `culprit`, `type`, `status`, `first_seen_at`, `last_seen_at`, `event_count`, `user_count`
- `events`: `id`, `issue_id`, `project_id`, `timestamp`, `environment`, `release`, `platform`, `message`, `user_json`, `tags_json`, `breadcrumbs_json`, `stacktrace_json`, `created_at`
- `hourly_stats`: `project_id`, `issue_id`, `hour_bucket`, `count`
- `release_artifacts`: `id`, `project_id`, `release`, `filename`, `content_type`, `data`, `created_at`
- `alert_configs`: `id`, `project_id`, `channel`, `target_url`, `enabled`, `notify_on_new`, `notify_on_regression`
- `alert_logs`: `id`, `project_id`, `issue_id`, `channel`, `sent_at`, `status`, `response`

### 4.3 Retention Pruner
A background worker runs every 24 hours to delete raw event rows older than the configured retention period (default: 30 days). Aggregated issues and hourly sparkline metrics remain intact for historical comparisons.

---

## 5. Alerting & Incident Lifecycle

### 5.1 Trigger Conditions
- **NEW_ISSUE**: A crash event generates a fingerprint never previously observed in the project database.
- **REGRESSION**: An incoming event matches a fingerprint whose current status is `RESOLVED`. The issue status is automatically restored to `UNRESOLVED` and marked as a regression.

### 5.2 Destination Formats
- **Slack**: Sends interactive Block Kit payloads with title, culprit, stack frame context, release badge, and a button linking back to the issue view.
- **Discord**: Dispatches embed objects with red/orange sidebars, culprit codeblocks, and environment metadata.
- **Telegram**: Posts messages formatted in Telegram HTML, escaping special characters and embedding code blocks for stack traces.
- **Generic Webhook**: Sends raw JSON containing the full issue summary, event details, and cause.

---

## 6. Server-Sent Events (SSE) Stream

ErrPulse provides a live SSE stream at `GET /api/v1/stream?project_id={project_id}`.

### Event Channels
- `event: new_event`: Emitted when an envelope arrives and is persisted.
- `event: issue_update`: Emitted when issue counters increment or issue status changes.
- `event: ping`: Keepalive heartbeat every 15 seconds.

The embedded React 19 interface connects to this stream upon launch, updating issue counters and sparkline charts without browser reloads.

---

## 7. PII Data Sanitization Pipeline

To prevent leaking sensitive database parameters, payment details, or credentials, incoming payloads pass through an automated sanitizer before being written to SQLite or forwarded to alert webhooks.

### 7.1 Key-Based Redaction
Any key in `tags`, `extra`, `user`, or request body matching known credential names is replaced with `[REDACTED]`:
- `password`, `token`, `secret`, `api_key`, `access_token`, `auth`, `cookie`, `credit_card`

### 7.2 Pattern-Based Scanning
String values undergo regular expression checks:
- **JWT Tokens**: Values matching `eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+` are replaced with `[REDACTED_JWT]`.
- **Payment Cards**: 13 to 19 digit strings formatted with digits, spaces, or hyphens matching the standard Luhn pattern are replaced with `[REDACTED_CREDIT_CARD]`.
- **Authorization Headers**: HTTP headers formatted as `Bearer <token>` or `Basic <token>` are masked.

---

## 8. Session Health and Crash-Free Analytics

ErrPulse consumes Sentry session items (`{"type": "session"}`) from client SDK envelopes to track release adoption and reliability over time.

### 8.1 Schema and Rollups
- `release_sessions`: Stores aggregated totals for `total_sessions`, `crashed_sessions`, `total_users`, and `crashed_users` partitioned by `(project_id, release)`.
- `session_distinct_users`: Tracks unique user identifiers per release to accurately measure user crash impact without double-counting.

### 8.2 Reliability Formulas
$$\text{Crash-Free Sessions } \% = \frac{\text{Total Sessions} - \text{Crashed Sessions}}{\text{Total Sessions}} \times 100$$
$$\text{Crash-Free Users } \% = \frac{\text{Total Users} - \text{Crashed Users}}{\text{Total Users}} \times 100$$

These percentages are computed dynamically and exposed via `GET /api/v1/releases/health`.

---

## 9. Search Syntax DSL Engine

The issue repository parses user queries into structured SQL filtering tokens:

- `is:unresolved`, `is:resolved`, `is:ignored`: Filters the `issues.status` column.
- `env:<name>`: Joins the `events` table on `events.environment = <name>`.
- `release:<tag>`: Performs a `LIKE` match on `issues.last_release`.
- `type:<error_type>`: Matches `issues.type`.
- Free text: Matches against `issues.title`, `issues.culprit`, or `issues.fingerprint`.

---

## 10. Alert Throttling and Spike Protection

To defend downstream notification channels from being overwhelmed during cascading service outages, ErrPulse includes an in-memory alert throttler:

1. **Cooldown Window**: Each issue maintains a sliding cooldown timer (default: 5 minutes). Duplicate crash events within this window are suppressed.
2. **Spike Detection**: When an issue receives more than 20 events within a 60-second sliding window, ErrPulse flags the event as an abnormal crash spike and broadcasts a high-priority alert banner `[CRITICAL SPIKE ALERT]`.

---

## 11. Prometheus Metrics Exposition

Operational telemetry is served at `GET /metrics` in standard Prometheus text format:
- `errpulse_uptime_seconds`: Process lifetime gauge.
- `errpulse_events_total`: Monotonic counter of all ingested error events.
- `errpulse_active_issues`: Gauge of open issues requiring triage.
- `errpulse_db_bytes`: Gauge measuring SQLite database disk footprint.
