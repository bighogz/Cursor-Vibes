# Design: Tradeoffs, Constraints, Non-Goals

This document captures the *why* behind the architecture. If you only have a few minutes,
read this before the code.

---

## Guiding constraints

1. **Portfolio artifact, not a startup.** This repo exists to demonstrate engineering judgment,
   not to serve production traffic. Every choice was made to show *thinking*, not just *shipping*.

2. **Real data, real APIs, real limits.** The system talks to live financial APIs (Yahoo Finance,
   FMP) with real rate limits and real failure modes. This is intentional — it forces the code
   to handle the messy parts (retries, fallbacks, caching, free-tier budget management) rather
   than mocking them away.

3. **Three languages on purpose.** Go, Rust, and TypeScript aren't here for resume padding.
   Each covers a distinct concern: Go for HTTP plumbing and orchestration, Rust for
   compute-heavy anomaly detection, TypeScript/React for a keyboard-first data-dense UI.
   The boundaries between them are the interesting part.

4. **Single-developer repo.** There's no team. The governance artifacts (CODEOWNERS, SECURITY.md,
   `docs/security/threat-model.md`) exist to show that I think about *process* even when I'm the only one
   affected. The overhead is near-zero; the signal is high.

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

**Decision**: Go wins on deployment simplicity and concurrency. The finance ecosystem gap is
closed by `go-yfinance` (native Go Yahoo client) and direct HTTP calls to FMP/SEC.

See also: [docs/decisions/0001-why-go-api.md](decisions/0001-why-go-api.md)

### Rust binary for anomaly detection

Anomaly detection is z-score math over arrays. It could run in Go. Rust was chosen to:

- Demonstrate a clean subprocess boundary (JSON over stdin/stdout, no shared memory)
- Show that I can write safe systems code and integrate it from another language
- Provide a natural extension point for heavier compute (e.g., streaming anomalies)

The Go fallback (`internal/aggregator`) runs the identical algorithm, so the Rust binary
is never a hard dependency. This was a deliberate design choice — graceful degradation.

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

These are things I deliberately chose *not* to do, and why.

| Non-goal | Why not |
|---|---|
| Host it publicly | Cost, maintenance burden, and the point is the *code*, not a live demo. See `make demo` for local proof. |
| Real-time WebSocket streaming | The data sources update at most daily (insider filings). WebSockets would add complexity with no user-visible benefit. |
| Database (Postgres, SQLite) | The dataset fits in memory and on disk as JSON. A database adds operational burden with no query complexity to justify it. |
| User authentication | There's one user (the reviewer running `make demo`). Auth would be boilerplate, not judgment. |
| Kubernetes / Docker Compose | Single binary. See [docs/ops-lite.md](ops-lite.md) for how I'd think about deployment. |
| 100% test coverage | Diminishing returns. Tests focus on security-critical paths: input validation, path traversal, auth timing, rate limiting, config loading. |
| Mobile-responsive layout | The UI is a dense financial data table. It's designed for a laptop screen. Making it responsive would compromise the data density that makes it useful. |

---

## What I'd change with more time

1. **Streaming anomaly pipeline.** Replace the batch "scan all tickers" model with
   an event-driven pipeline (e.g., Go channels feeding Rust via gRPC or NATS).
   Better for real-time use cases.

2. **Property-based testing.** Fuzz the JSON parsing boundaries between Go and Rust.
   The current tests are example-based; property tests would catch edge cases in
   symbol normalization and numeric parsing.

3. **OpenTelemetry traces.** The structured request logging is fine for debugging,
   but distributed traces would make it easier to see where latency hides
   (especially in the Yahoo/FMP API call chains).

4. **WASM anomaly module.** Compile the Rust anomaly code to WASM and run it in
   the browser for client-side "what-if" analysis. The subprocess boundary was
   designed to make this migration straightforward.
