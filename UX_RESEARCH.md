# UX Research & Product Design — S&P 500 Insider Tracker

**Product:** S&P 500 Insider Selling Tracker — website that tracks insider selling across all 503 S&P 500 companies, organized by sector, with stock price, quarterly trend, recent news, and top insider sellers per company.

**Target user:** Retail investors, analysts, and finance professionals who monitor insider activity as a signal.

**Primary goal:** User visits the site and immediately sees all relevant information — no loading, no manual refresh. Data is pre-populated and refreshes every 24 hours (aligned with quarterly insider filing cadence).

---

## 1. Top 5 User Goals & Critical Paths

| Goal | Critical path |
|------|---------------|
| **1. Scan for notable insider selling** | Land on dashboard → see sectors → find company row → read top insider sellers |
| **2. Compare insider activity across sectors** | Land → scroll sectors → compare companies within and across industries |
| **3. Check a specific company** | Land → use sector grouping or (future) search → find company → see price, trend, news, insiders |
| **4. Understand context (price, news)** | Land → read company row → price + Q trend + news + insiders in one view |
| **5. Trust data freshness** | Land → see "Last updated X ago" → know data is current without acting |

---

## 2. Heuristic Evaluation

| Heuristic | Score | Notes |
|-----------|-------|------|
| **Clarity** | 4/5 | Clear table headers and labels. "Q Trend" could be "3M % change." News/insider cells can be dense. |
| **Hierarchy** | 4/5 | Sector → company is clear. Hero could be less prominent; content is primary. |
| **Navigation** | 3/5 | Two links (Dashboard, Anomaly Scan). No in-page nav (e.g. jump to sector). No search. |
| **Forms** | N/A | No forms on main dashboard (improved). Anomaly Scan has parameters. |
| **Feedback** | 3/5 | Loading state exists. Error states are generic. No success feedback. "Last updated" helps. |
| **Accessibility** | 3/5 | Tables need scope/headers. Color-only cues (up/down). No skip links. |
| **Performance perception** | 4/5 | Pre-loaded data feels instant. Brief "Loading…" on first paint is acceptable. |

---

## 3. Top 10 UX Improvements (Impact × Effort)

| # | Improvement | Impact | Effort | Rationale |
|---|-------------|--------|--------|-----------|
| 1 | **Pre-populate dashboard, remove Load CTA** | High | Low | Eliminates primary friction; aligns with user goal of "see it already loaded" |
| 2 | **Add "Last updated" timestamp** | High | Low | Builds trust; explains data cadence without user action |
| 3 | **Sector jump links / anchor nav** | Medium | Low | Reduces scroll for users targeting a sector |
| 4 | **Improve empty state copy** | Medium | Low | "Data is being prepared" → friendlier, sets expectation |
| 5 | **Mobile: collapsible sector cards** | High | Medium | Tables don’t fit mobile; accordion preserves hierarchy |
| 6 | **Search / filter by ticker or company** | High | Medium | Critical for "find specific company" goal |
| 7 | **ARIA labels, table scope, skip link** | Medium | Low | WCAG 2.1 AA baseline |
| 8 | **Sticky sector headers on scroll** | Low | Medium | Nice-to-have for long lists |
| 9 | **Error recovery CTA** | Medium | Low | "Retry" or "Check status" instead of plain error text |
| 10 | **Reduce color-only dependency** | Medium | Low | Add ▲/▼ or "+"/"-" with color for trend |

---

## 4. Microcopy Improvements

| Context | Current | Recommended |
|---------|---------|-------------|
| **Primary CTA (removed)** | "Load Dashboard" | N/A — removed |
| **Hero subtext** | "All 503 companies…" | "All 503 companies by sector. Stock price, quarterly trend, news, and top insider sellers. Updates daily." |
| **Loading** | "Loading dashboard…" | "Loading…" (shorter; less cognitive load) |
| **Error (generic)** | "Could not load dashboard. Please try again." | "We couldn’t load the dashboard. It may be updating — try again in a minute." |
| **Empty state** | "No data yet. Data is prepared on first server start — check back shortly." | "Dashboard is being built. This usually takes 2–3 minutes on first run. Check back shortly." |
| **Last updated** | "Updated X ago" | "Data as of X" (or "Updated X ago") — both acceptable |
| **Table: Q Trend** | "Q Trend" | "3M %" or "90d change" — clearer for non-experts |
| **Table: Top Insider Sellers** | "Top Insider Sellers" | "Top sellers (shares)" |

---

## 5. Mobile-First Layout Recommendation

**Information architecture:**
- Primary: Sectors (collapsible list)
- Secondary: Companies within each sector (cards or simplified rows)
- Tertiary: Company detail (price, trend, news, insiders)

**Key screen sections (mobile):**
1. **Header** — Logo/title, "Last updated"
2. **Sector accordion** — Tap sector → expand to show companies
3. **Company card** — Symbol, name, price, trend badge; tap for news + insiders
4. **Footer** — Disclaimer, links (Anomaly Scan, etc.)

**Breakpoints:**
- &lt; 768px: Accordion + cards
- 768px–1024px: Stacked tables, horizontal scroll if needed
- &gt; 1024px: Full tables as today

---

## 6. Accessibility Checklist (WCAG-Oriented)

| Requirement | Status | Change |
|-------------|--------|--------|
| 1.1.1 Non-text content | Partial | Add `alt` for any icons; decoratives `aria-hidden` |
| 1.3.1 Info and relationships | Partial | Tables: `scope="col"`, `<th id>`, `<td headers>` |
| 1.4.1 Use of color | Partial | Add ▲/▼ or text with color for trends |
| 2.1.1 Keyboard | Partial | Ensure all interactive elements focusable |
| 2.4.1 Bypass blocks | Fail | Add skip link: "Skip to main content" |
| 2.4.4 Link purpose | OK | Links have descriptive text |
| 3.2.1 On focus | OK | No unexpected changes |
| 4.1.2 Name, role, value | Partial | Buttons/links have accessible names; tables need review |

**UI component changes:**
- Tables: `scope="col"` on `<th>`, `id`/`headers` for complex tables
- Links: `rel="noopener"` (already present) for external
- Skip link: `<a href="#main" class="skip-link">Skip to main content</a>`
- Loading spinner: `aria-live="polite"` region
- Error/empty: `role="alert"` or `aria-live="assertive"` when shown

---

## 7. Instrumentation Plan

**Events to track:**
| Event | When | Properties |
|-------|------|------------|
| `page_view` | Dashboard load | sector_count, company_count |
| `sector_expand` | User expands sector (mobile) | sector_name |
| `company_row_view` | Company row enters viewport | symbol, sector (optional) |
| `news_click` | User clicks news link | symbol, url |
| `anomaly_scan_click` | User navigates to Anomaly Scan | — |
| `error_shown` | Error/empty state displayed | error_type |

**Funnels:**
1. **Engagement:** page_view → sector_expand → company_row_view
2. **Digging deeper:** company_row_view → news_click
3. **Cross-feature:** page_view → anomaly_scan_click

**Success metrics:**
- **Primary:** % of sessions with company_row_view (engagement)
- **Secondary:** Avg. time on page, scroll depth
- **Operational:** Cache hit rate, last_updated age at page load

**Implementation:** Small team — start with `page_view` and `error_shown`; add scroll/expand events when analytics (e.g. Plausible, PostHog) is in place.
