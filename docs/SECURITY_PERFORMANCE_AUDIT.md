# Security Architecture & Performance Analysis

**Vibes – S&P 500 Insider Selling Tracker**  
Senior Security Architect + Performance Engineer Review  
*Balanced security × performance recommendations*

---

## Executive Summary

### Top 5 Security Risks

| # | Risk | Location | Impact |
|---|------|----------|--------|
| 1 | No auth on `/api/scan`, `/api/dashboard/refresh` | `cmd/api/main.go`, `api.py` | Unauthorized resource use, API key burn |
| 2 | API keys in URL query params (FMP, EODHD) | `internal/fmp/client.go:31` | Log/proxy leakage |
| 3 | `VIBES_ANOMALY_BIN` unvalidated → arbitrary binary exec | `internal/rustclient/rustclient.go:47` | RCE if env compromised |
| 4 | HTTP client has no timeout | `internal/fmp/client.go:33` | Resource exhaustion, DoS |
| 5 | Rate limiter map grows unbounded | `cmd/api/middleware.go:26` | Memory exhaustion |

### Top 5 Performance Bottlenecks

| # | Bottleneck | Location | Impact |
|---|------------|----------|--------|
| 1 | 1 FMP call per ticker (N+1) | `internal/aggregator/aggregator.go:31` | ~500 calls for full scan |
| 2 | Sequential dashboard fetch (quotes, hist, news) | `internal/dashboard/dashboard.go:98-135` | 2–3 min build |
| 3 | No HTTP connection reuse | `internal/fmp/client.go`, `yahoo/client.go` | Extra TLS handshakes |
| 4 | No debounce on cache refresh | `cmd/api/main.go:88` | Multiple concurrent builds |
| 5 | No response compression | All handlers | Large JSON over wire |

### Quick Wins (<1 day)

- [x] Add HTTP timeout to all Go clients – **DONE** (`internal/httpclient/client.go` 30s, shared client)
- [x] Restrict `VIBES_ANOMALY_BIN` to absolute path under project – **DONE** (`internal/rustclient/rustclient.go`)
- [x] Add 5-min cooldown debounce for cache refresh – **DONE** (`cmd/api/main.go`)
- [x] LRU eviction for rate limiter – **DONE** (`cmd/api/middleware.go`, max 10k entries, 1h TTL eviction)
- [x] Cache file mode 0600 – **DONE** (`internal/cache/cache.go`)
- [x] Optional `ADMIN_API_KEY` for `/api/scan` and `/api/dashboard/refresh` – **DONE** (`config.AdminAPIKey`, `X-Admin-Key` or `Authorization: Bearer`)

---

## 1. System Overview

### 1.1 Trust Boundaries

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         UNTRUSTED (Internet)                                  │
│  User Browser  │  FMP  │  Yahoo  │  EODHD  │  SEC-API  │  Financial Datasets│
└───────┬────────┴───┬───┴────┬─────┴────┬────┴─────┬─────┴──────────┬─────────┘
        │            │       │          │           │                │
        │  HTTP      │       │          │           │  API Keys      │
        ▼            ▼       ▼          ▼           ▼                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  ENTRY POINTS (Trust Boundary)                                               │
│  • cmd/api (Go):  GET /, /static/*, /api/dashboard, /api/scan, /api/health    │
│  • api.py (FastAPI): same routes                                              │
│  • cmd/scan (Go CLI)                                                          │
│  • main.py (Python CLI)                                                      │
└───────┬─────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  INTERNAL (Trusted)                                                          │
│  • config: .env → FMP_API_KEY, EODHD_API_KEY, SEC_API_KEY, FMP_FREE_TIER     │
│  • aggregator: FMP + others → dedupe → anomaly                               │
│  • rustclient: spawn vibes-anomaly, stdin/stdout JSON                        │
│  • dashboard: S&P 500 → quotes/hist/news/insiders → sector grouping          │
│  • cache: data/dashboard_cache.json (24h TTL)                                │
└───────┬─────────────────────────────────────────────────────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  STORAGE & SECRETS                                                           │
│  • .env (API keys) – gitignored                                              │
│  • data/dashboard_cache.json – world-readable (0644)                          │
│  • static/* – served as-is                                                   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 Data Flow

```
User → GET /api/dashboard
         → cache.Read() [HIT] → return JSON
         → [MISS] go refreshCache() → dashboard.Build()
              → sp500.Load() [CSV from GitHub]
              → FMP/Yahoo quotes (batch)
              → FMP/Yahoo hist + news (per-ticker, sequential)
              → aggregator.AggregateInsiderSells() [N+1 FMP calls]
              → cache.Write()

User → POST /api/scan?limit=25&baseline_days=365&...
         → rateLimitScan (5s per IP)
         → fmp.GetSP500Tickers()
         → aggregator.AggregateInsiderSells(tickers)
         → rustclient.ComputeAnomalySignals() OR aggregator.ComputeAnomalySignals()
         → return JSON
```

### 1.3 Key Assets

| Asset | Sensitivity | Storage |
|-------|-------------|---------|
| FMP_API_KEY | High | .env |
| EODHD_API_KEY | High | .env |
| SEC_API_KEY | High | .env |
| Dashboard cache | Low | data/dashboard_cache.json |
| Insider records | Low | In-memory, not persisted |

### 1.4 Dependencies

| Stack | File | Pinned? | Notable |
|-------|------|---------|---------|
| Go | go.mod | yes (indirect) | godotenv |
| Python | requirements.txt | min versions | requests, yfinance, fastapi |
| Rust | rust-core/Cargo.toml | Cargo.lock | serde, chrono |

---

## 2. Threat Model (Top 10)

| # | Threat | Type | Likelihood | Impact | Current Exposure | Exploit Sketch | Mitigation |
|---|--------|------|------------|--------|------------------|----------------|------------|
| 1 | Unauthorized scan/refresh | AuthZ | High | High | No auth | `curl -X POST /api/scan` burns API quota | API key or JWT for admin endpoints |
| 2 | API key leakage | Info disclosure | Medium | High | Key in URL | Proxy log, referrer, browser history | Use Authorization header |
| 3 | Arbitrary binary execution | Spoofing | Low | Critical | VIBES_ANOMALY_BIN | Attacker sets env to /tmp/evil | Validate path: absolute, under project |
| 4 | DoS via scan | Availability | High | Medium | No auth, rate limit per IP only | Multiple IPs, exhaust FMP quota | Auth + tighter rate limit |
| 5 | Path traversal | Tampering | Low | Medium | Partial | `..%2f`, `....//` variants | Use filepath.Clean, deny outside base |
| 6 | SSRF via ticker | Tampering | Low | Medium | Ticker in FMP URL | Ticker `@evil.com` if API allows | Whitelist ticker format |
| 7 | Error info leakage | Info disclosure | Medium | Low | stderr in rustclient error | Panic stack, paths in errors | Sanitize before returning |
| 8 | X-Forwarded-For spoofing | Spoofing | Medium | Low | Trusted for rate limit | Set X-Forwarded-For to bypass | Use rightmost IP if behind proxy |
| 9 | Dependency vuln | Supply chain | Medium | Medium | No scanning | CVE in requests, etc. | Dependabot, `pip audit`, `cargo audit` |
| 10 | Cache file read | Info disclosure | Low | Low | 0644 | Local user reads cache | 0600 permissions |

---

## 3. Security Architecture Review

### 3.1 Authentication / Authorization

| Where | Status | Gap |
|-------|--------|-----|
| `/`, `/static/` | None | Public by design – OK |
| `/api/dashboard` | None | Read-only cached data – OK |
| `/api/health` | None | Health check – OK |
| `/api/scan` | None | **Gap**: Expensive, burns API keys |
| `/api/dashboard/refresh` | None | **Gap**: Triggers full rebuild |
| `/api/dashboard/meta` | None | Low risk |

**Prioritized fix**: Add optional `ADMIN_API_KEY`; require `Authorization: Bearer <key>` or `X-API-Key` on `/api/scan` and `/api/dashboard/refresh`. Default: no key = reject if `ADMIN_API_KEY` set.

### 3.2 Secrets Management

| Issue | Location | Fix |
|-------|----------|-----|
| Keys in URL | `internal/fmp/client.go:31` | FMP supports `Authorization: Bearer` – switch |
| .env plaintext | config | Acceptable for local; consider vault in prod |
| No rotation | N/A | Document manual rotation |
| Log risk | No sensitive logging found | Add guideline: never log keys |

### 3.3 Input Validation & Injection

| Risk | Location | Status |
|------|----------|--------|
| SQL/NoSQL | N/A | No DB |
| Shell | `internal/rustclient/rustclient.go:107` | `exec.Command(binPath)` – no shell, but binPath from env |
| SSRF | `internal/fmp/client.go`, yahoo | Fixed URLs; ticker in params only |
| Path traversal | `cmd/api/main.go:63-71`, `middleware.go:64` | Has `..` check + `safeStaticPath` |

**Fix for rustclient**: Reject `VIBES_ANOMALY_BIN` if not `filepath.IsAbs` or not under project root.

### 3.4 Cryptography

| Area | Status |
|------|--------|
| TLS | Not enforced in code; assume LB terminates |
| Randomness | N/A for crypto |
| Key storage | .env – standard for this setup |

### 3.5 Dependencies

| Stack | Tool | Command |
|-------|------|---------|
| Go | go list -m all | Audit manually |
| Python | pip-audit, Safety | `pip install pip-audit && pip-audit` |
| Rust | cargo-audit | `cargo install cargo-audit && cargo audit` |

### 3.6 Logging / Telemetry

| Issue | Status |
|-------|--------|
| PII in logs | No logging of user data |
| Token leakage | No tokens logged |
| Log injection | N/A – no structured logging yet |

### 3.7 Error Handling

| Location | Issue |
|----------|-------|
| `internal/rustclient/rustclient.go:115` | Returns stderr to caller – may leak paths |
| `internal/fmp/client.go` | Returns `_error` map – internal shape |
| `cmd/api` | No stack traces to client – good |

### 3.8 Sandbox / Isolation

| Component | Isolation |
|-----------|-----------|
| Go API | Single process, no sandbox |
| Rust binary | Subprocess, receives JSON only |
| Python | Same |

### 3.9 Secure Defaults

| Setting | Current | Recommended |
|---------|---------|-------------|
| FMP_FREE_TIER | false | true for new installs |
| Scan limit | 25 (free) / 0 (paid) | Default 25 |
| Rate limit | 5s per IP | Keep; add auth for scan |

---

## 4. Performance Architecture Review

### 4.1 Hot Paths

| Path | Component | CPU | I/O | Network |
|------|-----------|-----|-----|---------|
| GET /api/dashboard | cache read | Low | File read | None |
| Cache miss | dashboard.Build | Low | File write | **High** (FMP/Yahoo) |
| POST /api/scan | aggregator + anomaly | Medium | None | **High** (FMP) |

### 4.2 Bottlenecks

- **N+1**: `aggregator.AggregateInsiderSells` – 1 FMP call per ticker (`internal/aggregator/aggregator.go:32-41`)
- **Sequential waits**: Dashboard hist/news loops with `time.Sleep(80ms)` (`internal/dashboard/dashboard.go:98-135`)
- **No connection reuse**: `http.Get` per request; default client has no timeout
- **Single-thread refresh**: `refreshMu` serializes all cache builds

### 4.3 Concurrency Model

| Language | Model |
|----------|-------|
| Go | Goroutines for startup refresh; handlers blocking |
| Python | FastAPI async, but clients sync; ThreadPoolExecutor for scan |
| Rust | Single-threaded binary |

### 4.4 Caching

| Cache | TTL | Invalidation |
|-------|-----|--------------|
| dashboard_cache.json | 24h | Time-based; refresh on stale read |
| No cache for scan | N/A | Each scan hits APIs |

### 4.5 Profiling Plan

| Language | Tool | Command |
|----------|------|---------|
| Go | pprof | `go test -cpuprofile=cpu.prof`; `go tool pprof -http=:6060 cpu.prof` |
| Go | trace | `import _ "net/http/pprof"`; `curl localhost:6060/debug/pprof/trace?seconds=5` |
| Python | py-spy | `py-spy record -o profile.svg -- python api.py` |
| Python | cProfile | `python -m cProfile -o out.prof main.py` |
| Rust | criterion | Add `[[bench]]` in Cargo.toml |
| Rust | flamegraph | `cargo flamegraph --bin vibes-anomaly` |

---

## 5. Balanced Recommendations

### No-Regrets (Security + Performance)

| # | Change | Security | Perf | Effort | Files |
|---|--------|----------|------|--------|-------|
| 1 | HTTP timeout 30s on all clients | DoS resistance | Prevents hangs | Low | fmp/client.go, yahoo/client.go |
| 2 | Reuse `http.Client` with transport pool | N/A | Fewer connections | Low | fmp, yahoo, sp500 |
| 3 | Restrict `VIBES_ANOMALY_BIN` to abs path under project | RCE prevention | N/A | Low | rustclient/rustclient.go |
| 4 | Cache file 0600 | Local exposure | N/A | Trivial | cache/cache.go |
| 5 | Debounce cache refresh 5 min | Reduces thundering herd | Fewer redundant builds | Low | cmd/api/main.go |

### Tradeoffs

| # | Change | Security | Perf | Note |
|---|--------|----------|------|------|
| 6 | Auth on /api/scan | + | Slight latency for key check | Acceptable |
| 7 | FMP key in header | + | None | FMP must support it |
| 8 | Parallel dashboard fetch | N/A | +++ | More concurrent API calls; stay within rate limits |
| 9 | Gzip responses | N/A | + bandwidth | Small CPU cost |
| 10 | Rate limiter LRU | Memory safety | Tiny alloc overhead | Worth it |

### Prioritized Roadmap

| Phase | Changes | Owner | Timeline |
|-------|---------|-------|----------|
| **Quick wins** | Timeouts, client reuse, VIBES_ANOMALY_BIN validation, 0600 cache, debounce | Dev | 1 day |
| **Week 1** | Optional ADMIN_API_KEY for scan/refresh | Dev | 2 days |
| **Week 2** | FMP key in header; rate limiter LRU | Dev | 1 day |
| **Week 3** | Parallel dashboard hist/news (bounded concurrency) | Dev | 2 days |
| **Ongoing** | Dependency scanning in CI; pprof in dev | Dev | 1 day setup |

---

## 6. Language-Specific Best Practices

### Python

| Area | Current | Recommendation |
|------|---------|----------------|
| Secrets | dotenv | OK; never `print(api_key)` |
| HTTP | requests | Add `timeout=30`, `verify=True`; check proxy config |
| Validation | Minimal | Pydantic for /api/scan body if added |
| Subprocess | N/A | If used: `subprocess.run(..., shell=False)` |
| Perf | Sequential | Consider `asyncio` + `aiohttp` for parallel fetch |
| Profiling | N/A | `py-spy record`, `cProfile` |

### Go

| Area | Current | Recommendation |
|------|---------|----------------|
| Context | Not used | Add `context.WithTimeout` for HTTP, scan |
| http.Client | Default | `&http.Client{Timeout: 30*time.Second}`; reuse |
| TLS | Default | OK; ensure `InsecureSkipVerify: false` |
| SQL | N/A | N/A |
| Input | Clamped | Good; add explicit validation errors |
| Perf | Blocking | pprof; watch goroutine leaks |

### Rust

| Area | Current | Recommendation |
|------|---------|----------------|
| Unsafe | None | Good |
| Crates | serde, chrono | `cargo audit` |
| Secrets | N/A | If added: consider `zeroize` |
| Parsing | serde | Good; strict date parsing |
| Perf | Single-threaded | criterion for benchmarks |

---

## 7. Tests and Verification

### Security

| Test | Tool | CI command |
|------|------|------------|
| Dep vulns (Go) | govulncheck | `go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...` |
| Dep vulns (Python) | pip-audit | `pip-audit` |
| Dep vulns (Rust) | cargo-audit | `cargo audit` |
| SAST (Go) | gosec | `gosec ./...` |
| Fuzzing | go test -fuzz | Fuzz aggregator input parsing |

### Performance

| Test | Tool | Target |
|------|------|--------|
| Load | wrk, k6 | 100 req/s on /api/dashboard |
| Benchmark | Go testing | aggregator, cache |
| Benchmark | criterion | Rust anomaly |

### Example CI Additions

```yaml
# .github/workflows/ci.yml
- name: Security scan (Go)
  run: go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
- name: Security scan (Python)
  run: pip install pip-audit && pip-audit
- name: Security scan (Rust)
  run: cargo install cargo-audit && cargo audit
```

---

## 8. Implementation Checklist

### Quick Wins (<1 day)

- [ ] Add 30s timeout to `http.Get` in fmp, yahoo, sp500
- [ ] Create shared `http.Client` with timeout, reuse
- [ ] Validate `VIBES_ANOMALY_BIN`: must be abs path, under project
- [ ] Change cache write to `0600`
- [ ] Add `lastRefresh` time; skip refresh if <5 min ago

### Strategic Refactors

- [ ] Optional `ADMIN_API_KEY` for /api/scan and /api/dashboard/refresh
- [ ] Move FMP auth to header (if API supports)
- [ ] Rate limiter: replace map with LRU (e.g. `github.com/hashicorp/golang-lru`)
- [ ] Dashboard: parallel fetch hist/news with `errgroup` (limit concurrency)
- [ ] Add `Content-Encoding: gzip` for JSON responses

---

*Document generated from security + performance audit. Last updated: 2025-02.*
