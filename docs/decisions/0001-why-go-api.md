# ADR 0001: Go for the API server

**Status:** Accepted
**Date:** 2025-12-01
**Context:** Choosing the primary language for the HTTP API and data orchestration layer.

---

## Decision

Use Go for the API server (`cmd/api`) and all data-fetching orchestration
(Yahoo Finance, FMP, SEC-API, dashboard assembly, caching).

## Context

The project originally used Python (FastAPI + yfinance). That worked for prototyping,
but created problems:

- **Subprocess fragility.** Yahoo data was fetched via `yfinance` in a Python subprocess
  called from Go. Stderr warnings from Python libraries (deprecation notices, SSL
  warnings) corrupted JSON output. Parsing required defensive guards.
- **Deployment complexity.** Running the app required Python 3.x, pip, a virtualenv,
  and matching system-level SSL libraries. None of that is needed with Go.
- **Concurrency.** Python's GIL makes concurrent HTTP calls awkward (asyncio or
  multiprocessing, both with their own footguns). Go's goroutines handle this natively.
- **Cold start.** Python's interpreter + import chain added ~2 s to startup. The Go
  binary starts in ~50 ms.

## Alternatives considered

| Option | Pros | Cons |
|---|---|---|
| **Python (FastAPI)** | Rich finance ecosystem, fast prototyping | GIL, subprocess fragility, deployment weight |
| **Node.js (Express)** | Same language as frontend, large ecosystem | Weaker typing, callback complexity for CPU work |
| **Rust (Axum)** | Maximum performance, memory safety | Slower iteration speed, heavier for HTTP plumbing |
| **Go (net/http)** | Single binary, goroutines, fast builds | Thinner finance ecosystem |

## Consequences

- **Positive:** Single binary deployment, zero runtime dependencies, native concurrency,
  fast cold start, smaller attack surface (fewer transitive dependencies).
- **Negative:** No `pandas`/`yfinance` — had to find or build Go equivalents.
  Resolved by adopting `go-yfinance` (native Go Yahoo client with TLS fingerprint
  spoofing) and direct HTTP calls to FMP.
- **Accepted risk:** Go's error handling is verbose. The codebase is larger than
  the equivalent Python would be. This is an acceptable trade for the deployment
  and reliability benefits.
