# Design: Tradeoffs, Constraints, Non-Goals

The *why* behind the architecture. Read this before the code.

---

## Guiding constraints

1. **Portfolio artifact, not a startup.** This repo demonstrates engineering judgment,
   not production traffic. The choices matter more than the output.

2. **Real data, real APIs, real limits.** The system talks to live financial APIs (Yahoo Finance,
   FMP) with real rate limits and failure modes. This forces the code to handle retries,
   fallbacks, caching, and free-tier budget management — not mock them away.

3. **Three languages on purpose.** Go, Rust, and TypeScript aren't here for resume padding.
   Each covers a distinct concern: Go for HTTP plumbing and orchestration, Rust for
   compute-heavy anomaly detection, TypeScript/React for a keyboard-first data-dense UI.
   The boundaries between them are the interesting part.

4. **Single-developer repo.** No team. Governance artifacts (CODEOWNERS, SECURITY.md,
   `docs/security/threat-model.md`) are present because the habits transfer.
   Overhead is near-zero.

---

## Key tradeoffs

### Go API instead of Python/FastAPI

| Factor | Go | Python |
|---|---|---|
| Startup time | ~50 ms (single binary) | ~2 s (interpreter + deps) |
| Concurrency | Goroutines, no GIL | asyncio or multiprocessing |
| Deployment | One static binary, zero runtime deps | virtualenv, pip, Python version management |
| Ecosystem for finance | Thinner (no pandas) | Rich (yfinance, pandas) |
| Security surface | Smaller binary, fewer transitive deps | Larger dependency tree |

**Decision**: Go wins on deployment simplicity and concurrency. `go-yfinance` and direct
HTTP calls to FMP/SEC close the ecosystem gap.

See also: [docs/decisions/0001-why-go-api.md](decisions/0001-why-go-api.md)

### Rust binary for anomaly detection

Anomaly detection is z-score math over arrays. It could run in Go. Rust was chosen to:

- Enforce a clean subprocess boundary (JSON over stdin/stdout, no shared memory)
- Provide an extension point for heavier compute (streaming anomalies, WASM)

The Go fallback (`internal/aggregator`) runs the identical algorithm, so the Rust binary
is never a hard dependency.

See also: [docs/decisions/0002-why-rust-binary.md](decisions/0002-why-rust-binary.md)

### File-based cache instead of Redis

- **No infrastructure dependency.** `data/dashboard_cache.json` is a single file.
  No Redis server, no Docker, no connection string.
- **Appropriate for scale.** The dataset is ~500 companies. JSON serialization
  takes microseconds. Adding Redis would be resume-driven engineering.
- **Tradeoff accepted:** No distributed invalidation, no atomic writes.
  For a single-binary deployment, this is fine.

### React SPA instead of server-rendered HTML

The dashboard is data-dense (500 rows, sparklines, drawers, keyboard navigation).
Server-rendered HTML would require either heavy HTMX/Alpine plumbing or constant
round-trips for state changes. React lets the frontend hold state locally
(selected stock, drawer open, sector filter) while the API stays stateless.

The SPA is compiled to static files and served by the same Go binary — no
separate frontend server in production.

---

## Non-goals

Deliberate omissions and why.

| Non-goal | Why not |
|---|---|
| Host it publicly | Cost, maintenance burden. `make demo` proves the system locally. |
| Real-time WebSockets | Data sources update at most daily. WebSockets add complexity without benefit. |
| Database (Postgres, SQLite) | Dataset fits in memory and on disk as JSON. No query complexity justifies the overhead. |
| User authentication | One user: the reviewer running `make demo`. |
| Kubernetes / Docker Compose | Single binary. See [docs/ops-lite.md](ops-lite.md). |
| 100% test coverage | Diminishing returns. Tests target security-critical paths: input validation, path traversal, auth timing, rate limiting. |
| Mobile-responsive layout | Dense financial data table, designed for laptop screens. Responsiveness would compromise density. |

---

## What I'd change with more time

1. **Streaming anomaly pipeline.** Replace batch scanning with an event-driven pipeline
   (Go channels feeding Rust via gRPC or NATS).

2. **Property-based testing.** Fuzz the JSON parsing boundaries between Go and Rust.
   Property tests would catch edge cases in symbol normalization and numeric parsing
   that example-based tests miss.

3. **OpenTelemetry traces.** Distributed traces would reveal where latency hides
   in the Yahoo/FMP API call chains.

4. **WASM anomaly module.** Compile the Rust anomaly code to WASM for client-side
   "what-if" analysis. The subprocess boundary already isolates the compute.
