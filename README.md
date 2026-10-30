# ErrPulse

Single-binary Sentry-compatible error and exception tracking daemon with embedded SQLite storage and a React 19 incident management studio.

ErrPulse accepts crash reports directly from standard Sentry SDKs (`@sentry/browser`, `@sentry/node`, `sentry-sdk`, `sentry-go`) over the official Sentry envelope ingestion protocol. It de-minifies production stack traces using Base64 VLQ source maps, groups duplicate exceptions using deterministic SHA-256 fingerprinting, stores events in SQLite WAL mode with a memory footprint under 30MB RAM, and dispatches alerts to Slack, Discord, and Telegram.

---

## Key Features

- **Drop-in Sentry Protocol Compatibility**: Implements `/api/{project_id}/envelope/`, `/api/{project_id}/store/`, and `/api/{project_id}/security/`. Point any existing Sentry SDK to your ErrPulse instance by changing only the DSN.
- **Sub-30MB RAM Footprint**: Runs as a single compiled Go binary. Uses SQLite in write-ahead log (WAL) mode with tuned cache sizes, running comfortably on entry-level virtual machines.
- **Pure Go VLQ Source Map Decoder**: Translates minified stack frames (`bundle.min.js:1:3402`) back into original TypeScript or JavaScript files with line numbers and five lines of surrounding source context from embedded `sourcesContent`.
- **Deterministic Fingerprinting**: Strips dynamic tokens (hex memory addresses, UUIDs, timestamps) and excludes third-party vendor frames (`node_modules`, standard library wrappers) to group related crashes into unified issues.
- **Regression Detection**: Flags issues when an error marked as resolved reappears in an incoming release.
- **Multi-Channel Alert Dispatcher**: Sends notifications with stack previews to Slack Block Kit, Discord Embeds, Telegram bots, or custom HTTP webhooks.
- **Real-Time Web Studio**: Embedded React 19 single page application served directly from the Go binary on port 9988, featuring real-time Server-Sent Events (SSE), breadcrumbs timelines, and release artifact management.
- **Sentry CLI Compatibility**: Supports direct artifact uploads via `sentry-cli releases files <version> upload-sourcemaps` for CI/CD pipelines.

---

## Quickstart

### 1. Download or Build

Build the static binary from source:

```bash
# Build frontend and compile single binary
cd web && npm install && npm run build && cd ..
go build -o errpulse ./cmd/errpulse
```

### 2. Start the Server

```bash
./errpulse serve --port 9988 --db ./errpulse.db
```

The incident management studio will be accessible at `http://localhost:9988`.

To launch with synthetic demo crashes pre-loaded for evaluation:

```bash
./errpulse serve --port 9988 --db ./errpulse.db --demo
```

---

## Client SDK Integration

Because ErrPulse implements Sentry envelope ingestion, no custom client libraries are required. Use the standard Sentry SDK for your language or framework.

The default project DSN format is:
```
http://errpulse_default_key@localhost:9988/1
```

### Browser (JavaScript / TypeScript)

```typescript
import * as Sentry from "@sentry/browser";

Sentry.init({
  dsn: "http://errpulse_default_key@localhost:9988/1",
  release: "webapp@1.0.0",
  environment: "production",
});
```

### Node.js / Express / Next.js

```typescript
import * as Sentry from "@sentry/node";

Sentry.init({
  dsn: "http://errpulse_default_key@localhost:9988/1",
  release: "backend@1.0.0",
  environment: "production",
});
```

### Python

```python
import sentry_sdk

sentry_sdk.init(
    dsn="http://errpulse_default_key@localhost:9988/1",
    release="api@2.4.0",
    environment="production",
)
```

### Go

```go
package main

import (
	"time"
	"github.com/getsentry/sentry-go"
)

func main() {
	sentry.Init(sentry.ClientOptions{
		Dsn: "http://errpulse_default_key@localhost:9988/1",
		Release: "service@1.2.0",
		Environment: "production",
	})
	defer sentry.Flush(2 * time.Second)
}
```

---

## Source Maps & De-Minification

ErrPulse includes a built-in Base64 VLQ source map parser written in pure Go. When a crash occurs in minified production code, ErrPulse inspects stored `.map` files for the corresponding release version and resolves each frame back to its original file and line number.

### Uploading Source Maps via Web Studio
1. Open the ErrPulse dashboard at `http://localhost:9988`.
2. Navigate to **Releases**.
3. Select or enter your release tag (e.g. `webapp@1.0.0`).
4. Drag and drop your `.map` and `.js` build output files.

### Uploading via Sentry CLI
ErrPulse implements the Sentry release artifacts API endpoint:

```bash
export SENTRY_URL=http://localhost:9988
export SENTRY_AUTH_TOKEN=errpulse_default_key
export SENTRY_ORG=default
export SENTRY_PROJECT=1

sentry-cli releases new webapp@1.0.0
sentry-cli releases files webapp@1.0.0 upload-sourcemaps ./dist --url-prefix "~/"
sentry-cli releases finalize webapp@1.0.0
```

---

## Alert Destinations

Configure webhook destinations in the Web Studio under **Settings > Alerts** or through the REST API.

- **Slack**: Formatted using Slack Block Kit blocks, displaying the exception title, culprit function, release tag, environment badge, and a direct link to the issue in ErrPulse.
- **Discord**: Uses Discord Embed cards with color-coded status bars (crimson for new issues, amber for regressions).
- **Telegram**: Dispatches messages to your Telegram channel or group using a bot token and chat ID.
- **Custom Webhook**: Sends a standard JSON payload on every trigger for integration with custom runbooks.

---

## Configuration Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--port` | `9988` | HTTP listening port for ingestion API and web studio |
| `--host` | `0.0.0.0` | Network binding interface |
| `--db` | `./errpulse.db` | File path for SQLite database |
| `--retention-days`| `30` | Automatic deletion threshold for resolved and raw events |
| `--demo` | `false` | Seeds realistic sample issues across multiple platforms |

---

## Architecture Overview

```
Sentry Client SDK
       │
       ▼ (HTTP POST /api/{project_id}/envelope/)
┌──────────────────────────────────────────────┐
│ Ingestion Engine                             │
│ 1. Validate DSN public key                   │
│ 2. Decompress body (gzip / deflate / zstd)   │
│ 3. Extract event payload and breadcrumbs     │
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│ VLQ Source Map Resolver                      │
│ 1. Match release and frame filename          │
│ 2. Binary search VLQ mapping table           │
│ 3. Extract TS source lines from sourcesContent│
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│ Fingerprinting & Normalization               │
│ 1. Scrub tokens: UUIDs, memory hex, counters │
│ 2. Strip non-app frames (node_modules, etc.) │
│ 3. Compute SHA-256 fingerprint               │
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│ SQLite WAL Engine & Alert Dispatcher         │
│ 1. Upsert issue and write event row          │
│ 2. Trigger Slack/Discord/Telegram if new/reg │
│ 3. Broadcast update over Server-Sent Events  │
└──────────────────────────────────────────────┘
```

Detailed protocol specifications and algorithmic details are available in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## License

MIT License. See [LICENSE](LICENSE) for details.
