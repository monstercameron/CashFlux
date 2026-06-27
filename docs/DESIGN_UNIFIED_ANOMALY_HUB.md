# Design — Unified Anomaly Hub (R25)

**Status:** research / spec delivered. The `/smart` hub already serves as the *de facto* unified
intelligence surface (severity-ranked, deduped, "why shown"); this doc maps the currently-fragmented
anomaly detectors + surfaces and specifies the model that makes the hub canonical for anomalies.
Remaining wiring is tracked as follow-on C-items.

**Goal:** CashFlux detects "something looks off" in several independent places. A user should see each
anomaly **once**, ranked by how much it matters, with one place to review them — not the same spike
echoed as a dashboard chip, a notification, a Smart card, and a reports highlight with different
wording and no shared dismissal. This realizes the style-spec's §8.6 mandate: "Smart cards must become
a decision layer, not repeated content furniture" — "group repeated rule hits", "share dismissal
state", "explain why shown".

---

## Current state — where anomalies are detected (fragmented)

| Detector | Code | What it flags |
|---|---|---|
| Balance anomaly (SMART-A1) | `internal/smartengine/accounts.go` (`a1BalanceAnomaly`) | An account whose current-month change is ≥`anomalyFactor`× (3×) its trailing-3-month mean (ignoring sub-$100 baselines). |
| Spending anomalies / highlights | `internal/insights/insights.go`, smartengine spending engines | Category spend "up 32% (+$120)" vs the prior period; honest empty-month wording. |
| Large transaction | `internal/notify/` (`EventLargeTransaction`, `defaultLargeTxnMinor`) | A single transaction over a threshold. |
| "Needs attention" | `internal/attention/attention.go` | Stale balances, over-budget, overdue bills — surfaced on the dashboard. |

## Current state — where they surface (also fragmented)

`/smart` hub (severity-ranked, deduped, capped — and since R38 a decision layer), the dashboard
"Needs attention" strip, the **notification center** (now severity-sorted — R38/§8.6), the
**Reports** anomaly highlights, and the **Insights** page. The same underlying event (e.g. a 3×
spending spike) can appear in several of these with independent wording, severity, and dismissal.

---

## The problem

1. **Duplication.** One real anomaly → multiple cards/notifications/chips, each its own rule, no shared
   identity or dismissal. Violates §8.6 "group repeated rule hits / share dismissal state".
2. **Inconsistent severity + wording.** Each detector ranks and phrases independently; the dashboard
   chip, the notification, and the Smart card may disagree on how urgent the same thing is.
3. **No single review surface for "what's unusual".** The `/smart` hub is close, but it mixes
   anomalies with routine recommendations; there is no anomaly-scoped view or a shared anomaly model.

---

## Design — the unified model

**One anomaly type, one ranker, many derived surfaces.**

1. **`Anomaly` as a first-class shape** (extend `smart.Insight`, do not fork it): `{ID (stable, by
   entity+kind+period), Kind (balance|spend-spike|large-txn|stale|over-budget|overdue),
   Severity, EntityRef, Period, Evidence, WhyShown, Action, Dismissed}`. Stable ID is the dedup key
   across every surface.
2. **One detection pass** in `smartengine` that emits `Anomaly`s (fold A1 + spending-spike + large-txn
   + attention signals into the same engine pass, `smart.SortInsights` for ranking). The attention
   engine and the large-txn notify rule become *consumers* of this pass, not independent detectors.
3. **`/smart` is the canonical hub.** It already ranks, dedups, caps, and explains; add an **"Unusual"
   filter/tab** scoped to `Kind != routine` so a user can answer "what's off this month?" in one place.
4. **Derived surfaces read from the same set.** The dashboard "Needs attention" strip = the top-N
   anomalies by severity; the notification center = anomalies above a notify threshold; Reports
   highlights = anomalies scoped to the viewed period. All share the **same ID and dismissal** — dismiss
   once, gone everywhere (§8.6).
5. **Grouping.** When one rule fires across many entities (e.g. 6 budgets over), collapse to a group
   row with drill-down (the same §8.6 rule the notification-center TODO already tracks).

---

## Recommendations / follow-on (C-items, not this design)

1. Introduce the `Anomaly` stable-ID + dedup key; make the dashboard "Needs attention" strip and the
   large-txn notification derive from the smartengine anomaly pass (shared identity + dismissal).
2. Add the "Unusual" filter to the `/smart` hub (already tabbed Insights/Manage — add a scope toggle).
3. Apply §8.6 grouping to repeated same-rule anomalies (shared with the notifications collapse-threshold
   TODO).

## Acceptance / done-condition

- [x] The fragmented anomaly detectors + surfaces are inventoried (tables above).
- [x] A unified model (one `Anomaly` shape, one detection pass, `/smart` as canonical hub, derived
      surfaces sharing ID + dismissal) is specified and grounded in the real code.
- [ ] (Follow-on) Implementation: shared ID/dismissal + "Unusual" hub filter + grouping — tracked as
      C-items, not part of this design.

The `/smart` hub + the R38 decision-layer + the R38/§8.6 severity-sorted notification center already
deliver the *presentation* half (ranked, deduped-within-surface, explained); this design closes the
loop by making the **detection + identity + dismissal** shared, so an anomaly is one thing the user
reviews once.
