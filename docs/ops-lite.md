# Ops Lite: How I'd Deploy This

> **Not deployed.** This document describes how the application *would* run
> as a real service.

---

## Deployment model

Single Go binary (`bin/api`) serving both the JSON API and the pre-built React SPA.
No sidecar containers, no separate frontend server, no application runtime to manage.

```
Internet
   │
   ▼
Reverse Proxy (Caddy / nginx / cloud LB)
   │  TLS termination
   │  rate limiting (complements built-in)
   │  access logs
   ▼
./bin/api  (:8000)
   │  serves /api/* + static React assets
   │  reads secrets from environment (1Password CLI or platform secret store)
   ▼
data/   (local filesystem — cache files, auto-created)
```

### Why this shape

- **Single process.** Fewer moving parts, fewer failure modes. The Go binary handles
  routing, middleware, API logic, static files, and subprocess management.
- **Reverse proxy in front.** TLS termination and edge-level rate limiting belong
  outside the application.
- **Local filesystem for cache.** Appropriate for single-node. Horizontal scaling
  would move the cache to Redis or S3.

---

## Secrets management

| Method | When to use | Risk |
|---|---|---|
| **1Password CLI** (`op run --env-file=.env.tpl`) | Local dev, CI/CD | Secrets never touch disk. Best option. |
| **Platform secret store** (AWS SSM, GCP Secret Manager) | Cloud deployment | Secrets injected into environment at boot. Rotation supported. |
| **`.env` file** | Quick local testing only | Plaintext on disk. Gitignored, but a risk if the machine is compromised. |

Secrets never belong in Docker images, CI artifacts, or version-controlled config.

---

## What I'd monitor

| Signal | Tool | Why |
|---|---|---|
| Request latency (p50, p95, p99) | Prometheus + Grafana (or Datadog) | Detect API degradation before users notice |
| Error rate by endpoint | Same | Catch upstream API failures (Yahoo, FMP rate limits) |
| Cache hit ratio | Custom metric from `internal/cache` | Low ratio = something is bypassing cache or TTL is wrong |
| Rust subprocess duration | Custom metric from `internal/rustclient` | Detect anomaly computation slowdowns |
| Dependency vuln count | `govulncheck` / `osv-scanner` in CI | Trend over time; alert on new critical/high |

### Health check chain

```
Load balancer → GET /api/health → {"status":"ok","version":"1.0.0","commit":"abc1234"}
                                   ↑ returns version + commit for deploy verification
```

The load balancer removes the instance from rotation after 3 consecutive non-200 checks
(30 s interval).

---

## What I'd do for high availability

Not needed at current scale, but straightforward to add:

1. **Stateless API.** All mutable state lives in the file cache. Move the cache to
   Redis and run N instances behind a load balancer.

2. **Cache warming.** A readiness probe waits for the cache to populate before
   accepting traffic (~30 s cold build).

3. **Blue-green deploys.** Build, verify health, swap traffic. Rollback = point back
   to the old binary.

4. **No database migrations.** Deploy is "copy binary + restart."

---

## Backup and disaster recovery

| Asset | Strategy | RPO | RTO |
|---|---|---|---|
| Source code | GitHub (already there) | 0 (every push) | Minutes (git clone) |
| Cache files | Ephemeral — rebuilt on startup | N/A | ~30 s (cache rebuild) |
| API keys | 1Password vault | N/A (managed externally) | Minutes (re-inject) |

No user data to back up. The cache derives from public APIs and rebuilds on demand.

---

## Cost estimate (if deployed)

| Resource | Spec | Monthly cost |
|---|---|---|
| VPS (Hetzner / DigitalOcean) | 2 vCPU, 4 GB RAM | ~$12 |
| Domain + TLS (Let's Encrypt) | Auto-renewed | $0 |
| FMP API (free tier) | 250 calls/day | $0 |
| Total | | **~$12/month** |

The app is CPU-light (mostly waiting on upstream HTTP) and memory-light (~50 MB RSS).
A $12 VPS suffices.

---

## Why I'm not deploying it

1. **Cost vs. signal.** `make demo` proves everything a live deployment would.
2. **Maintenance burden.** A deployed service needs uptime monitoring, certificate
   renewal, dependency updates, and incident response. The code is the deliverable.
3. **Data freshness.** Free-tier API limits leave the data stale within hours. A local
   demo with fresh data beats a live demo with stale data.
