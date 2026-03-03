# ADR 0002: Rust for anomaly detection

**Status:** Accepted
**Date:** 2025-12-01
**Context:** Deciding whether anomaly detection should run in Go or in a separate Rust binary.

---

## Decision

Implement anomaly detection and quarterly trend computation in a Rust CLI binary
(`vibes-anomaly`), invoked by the Go API as a subprocess with JSON over stdin/stdout.
Maintain an identical Go fallback (`internal/aggregator`, `internal/trend`) so the
Rust binary is never a hard dependency.

## Context

The anomaly detection algorithm is straightforward (z-score over historical insider
sell volumes). It does not *require* Rust. The question was whether adding a second
compiled language was worth the complexity.

## Why Rust

1. **Demonstrate a clean language boundary.** The Go → Rust subprocess interface is
   a real-world pattern (similar to how ripgrep integrates with editors, or how
   VS Code integrates language servers). Showing that I can design, implement, and
   test a cross-language boundary is more interesting than showing I can write
   all-Go or all-Rust.

2. **Natural extension point.** If the anomaly detection grew to include streaming
   computation, time-series pattern matching, or larger datasets, Rust's performance
   characteristics would matter. The current architecture makes that migration
   incremental rather than architectural.

3. **Binary safety.** The Go API validates the Rust binary path (`validateBinPath()`
   ensures it resolves under the project root) and passes data via stdin — no shell
   interpolation, no argument injection.

## Alternatives considered

| Option | Pros | Cons |
|---|---|---|
| **All Go** | Simpler build, one toolchain | Doesn't demonstrate cross-language design |
| **All Rust** | Maximum performance | Slower HTTP plumbing iteration, heavy for orchestration |
| **Go + Python subprocess** | Rich math ecosystem | Subprocess fragility (see ADR 0001) |
| **Go + Rust via cgo/FFI** | No subprocess overhead | cgo is fragile, cross-compilation is painful |
| **Go + Rust via gRPC** | Typed interface, streaming support | Heavier infra for a portfolio project |

## Consequences

- **Positive:** Clean subprocess boundary with JSON serialization. Graceful
  degradation (Go fallback). Demonstrates multi-language integration. Rust binary
  is optional — `make build` skips it if Rust isn't installed.
- **Negative:** Two build toolchains (Go + Cargo). CI must install both.
  Build time increases by ~15 s for the Rust compilation.
- **Accepted risk:** The subprocess adds ~50 ms of overhead per invocation
  (process start + JSON serialization). For a batch operation that runs at most
  once per cache refresh, this is negligible.
