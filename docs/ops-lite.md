# Ops Lite: How I'd Deploy This

> **This application is not deployed.** This document describes how I *would*
> think about running it if it were a real service. The goal is to demonstrate
> operational reasoning, not operational infrastructure.

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

- **Single process.** Fewer moving parts = fewer failure modes. The Go binary handles
  routing, middleware, API logic, static file serving, and subprocess management.
- **Reverse proxy in front.** TLS termination and edge-level rate limiting belong
  outside the application. The app trusts `X-Forwarded-For` from the proxy.
- **Local filesystem for cache.** Appropriate for single-node. If horizontal scaling
  were needed, I'd move the cache to Redis or S3 — but that's not the current scale.

---

## Secrets management

| Method | When to use | Risk |
|---|---|---|
| **1Password CLI** (`op run --env-file=.env.tpl`) | Local dev, CI/CD | Secrets never touch disk. Best option. |
| **Platform secret store** (AWS SSM, GCP Secret Manager) | Cloud deployment | Secrets injected into environment at boot. Rotation supported. |
| **`.env` file** | Quick local testing only | Plaintext on disk. Gitignored, but a risk if the machine is compromised. |

I would **not** bake secrets into Docker images, CI artifacts, or config files checked into git.

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

I'd configure the load balancer to remove the instance from rotation if `/api/health`
returns non-200 for 3 consecutive checks (30 s interval).

---

## What I'd do for high availability

Not needed at current scale, but here's how I'd think about it:

1. **Stateless API.** The Go binary is already stateless — all mutable state is
   in the file cache. Move the cache to a shared store (Redis) and you can run
   N instances behind a load balancer.

2. **Cache warming.** On deploy, the new instance's first request triggers a
   cold cache build (~30 s). I'd add a readiness probe that waits for the
   cache to populate before accepting traffic.

3. **Blue-green deploys.** Single binary makes this trivial: build, verify health,
   swap traffic. Rollback = point back to the old binary.

4. **No database migrations.** The app has no database. Deploy is "copy binary + restart."
   This eliminates an entire class of deployment risk.

---

## Backup and disaster recovery

| Asset | Strategy | RPO | RTO |
|---|---|---|---|
| Source code | GitHub (already there) | 0 (every push) | Minutes (git clone) |
| Cache files | Ephemeral — rebuilt on startup | N/A | ~30 s (cache rebuild) |
| API keys | 1Password vault | N/A (managed externally) | Minutes (re-inject) |

There's no user data to back up. The cache is derived from public financial APIs
and can be reconstructed at any time. This dramatically simplifies DR.

---

## Cost estimate (if deployed)

| Resource | Spec | Monthly cost |
|---|---|---|
| VPS (Hetzner / DigitalOcean) | 2 vCPU, 4 GB RAM | ~$12 |
| Domain + TLS (Let's Encrypt) | Auto-renewed | $0 |
| FMP API (free tier) | 250 calls/day | $0 |
| Total | | **~$12/month** |

The app is CPU-light (most time is spent waiting on upstream HTTP responses)
and memory-light (~50 MB RSS). A $12 VPS would be more than sufficient.

---

## Why I'm not deploying it

1. **Cost vs. signal.** Paying $12/month to host a portfolio project doesn't prove
   anything that `make demo` doesn't already prove.
2. **Maintenance burden.** A deployed service needs uptime monitoring, certificate
   renewal, dependency updates, and incident response. For a portfolio artifact,
   the code *is* the deliverable.
3. **Data freshness.** The free-tier API limits mean the data would be stale
   within hours. A live demo with stale data is worse than a local demo with
   fresh data.
