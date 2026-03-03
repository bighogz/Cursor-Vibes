# ADR 0003: Threat model scope and security framework alignment

**Status:** Accepted
**Date:** 2025-12-15
**Context:** Defining the security posture for a portfolio project that handles API keys and financial data but has no real users.

---

## Decision

Align the project to **NIST SP 800-218 (SSDF v1.1)** for development practices
and **NIST SP 800-53r5** for runtime controls. Apply both frameworks proportionally —
implement real controls, but don't pretend the threat model is the same as a
production financial system.

## Context

This is a portfolio project. It has:

- **Real secrets** (API keys for FMP, SEC-API, EODHD) that would cost money if leaked
- **Real external API calls** with real rate limits
- **No real users** beyond the developer and reviewers
- **No PII** (all data is public financial filings and market data)
- **No uptime SLA** (it's not deployed)

The question was: how much security is *appropriate*?

## Why these frameworks

- **SSDF (800-218)** covers the development lifecycle: code review, testing, dependency
  management, build integrity, vulnerability response. These practices cost almost nothing
  to implement and are good habits regardless of project size.
- **800-53** covers runtime controls: input validation, authentication, logging, error
  handling. The subset I implemented (SI-10, AU-12, AC-17, etc.) addresses real risks
  in the code, not theoretical compliance checkboxes.

Together they show that I think about security as a continuous practice, not a
bolt-on audit.

## What's in scope

| Threat | In scope | Rationale |
|---|---|---|
| API key leakage | Yes | Real financial cost if keys are exposed |
| Path traversal | Yes | Static file serving is a real attack surface |
| Timing attacks on auth | Yes | Demonstrates understanding of subtle vulnerabilities |
| Dependency vulnerabilities | Yes | Supply chain risk is real and automated to check |
| DDoS / resource exhaustion | Partially | Rate limiting is implemented; full DDoS mitigation is out of scope |
| Data integrity (insider records) | Yes | Garbage-in-garbage-out for anomaly detection; validate inputs |
| XSS | Yes | React JSX escaping + CSP, even though there's no user-generated content |

## What's out of scope

| Threat | Why excluded |
|---|---|
| Multi-tenant isolation | Single user, no tenancy |
| Data encryption at rest | Cache files contain public market data; no PII |
| Network segmentation | Single binary on localhost; no internal network to segment |
| Incident response team | Solo developer; SECURITY.md defines process for hypothetical team |
| SOC 2 / ISO 27001 | Certification frameworks require organizational context that doesn't exist |

## Consequences

- **Positive:** Security controls are real and testable (unit tests for path traversal,
  timing-safe auth comparison, input validation). CI runs automated scans (SAST, SCA,
  secret scanning). The security posture is documented and auditable.
- **Negative:** Some controls (CODEOWNERS, SECURITY.md disclosure SLAs) are aspirational
  for a solo project. They exist to show process awareness, not because they're
  operationally necessary today.
- **Key insight:** The most valuable security work in this project isn't the framework
  alignment — it's the habits: input validation by default, structured error responses,
  no secrets in code, automated scanning in CI. The frameworks just organize and
  communicate those habits.
