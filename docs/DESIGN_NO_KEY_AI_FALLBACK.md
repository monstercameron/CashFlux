# Design — No-Key AI Fallback (R24)

**Status:** research / spec delivered. Implementation gaps are called out per feature and tracked as
follow-on C-items; the strategy below is the durable design.

**Goal:** CashFlux is local-first and bring-your-own-key. Every feature that *can* use the OpenAI
key (vision extraction, AI Q&A, AI explanations) must have a usable, clearly-surfaced **no-key
path**, so a user who never enters a key is never blocked from the underlying job. This systematizes
the style-spec rules the app already follows in places:

- **§8.9** — "AI-key-gated features must expose non-AI alternatives first when possible."
- **§9.4** — "Never bury the safe/manual path below AI-key-gated paths."
- **§3.5 Explainable Trust** — deterministic results "show their work"; the no-key path is not a
  degraded mystery, it's the auditable default.

The principle: **AI is an opt-in *accelerator*, never the only door.** The free, on-device,
deterministic engines are the product; the key buys speed/convenience on top.

---

## Feature-by-feature fallback matrix

| AI-gated feature (needs key) | Where | No-key alternative (on-device, deterministic) | Status |
|---|---|---|---|
| **Insights / recommendations / Q&A** | `insights.go`, `smart_strip.go`, `/smart` | The **Free SMART engines** — the deterministic engine files in `internal/smartengine` (accounts, bills, budgets, goals, planning, subscriptions, todos, transactions, allocate) producing ~30 severity-ranked `smart.Insight`s with "why shown" + actions. Default-on, no key. | ✅ Works. AI Q&A is additive. |
| **Auto-categorization of transactions** | import + quick-add | **Rules engine** (`internal/rules`) + learned suggestions (`internal/rulesuggest`): keyword/field/amount conditions, applied on every add path (manual, CSV, bulk "Apply rules"). Fully deterministic. | ✅ Works. AI suggestion is additive. |
| **Receipt / bank-statement extraction** | `documents.go` (vision + text) | **CSV / OFX import** (`ImportTransactionsCSV`, OFX parser) with the map-columns wizard, **paste-statement parse** (`statement.ParseAny`, no AI), and **manual quick-add**. Documents now leads with the no-key CSV card (R55/§8.9). | ⚠️ Partial — see Gap 1 (no local OCR for image-only receipts). |
| **Allocate "explain this ranking"** | `allocate.go` | The **deterministic per-criterion breakdown** is always rendered (the allocator is pure, explainable Go — `internal/allocate`). The AI text is a paraphrase of numbers already on screen. | ✅ Works. AI explanation is additive. |
| **"Explain my month" / spending anomalies** | `insights.go` | Deterministic anomaly + delta highlights from `smartengine`/`reports` (e.g. "up 32% (+$120)"), plus the Reports page's full breakdown. | ✅ Works. AI narrative is additive. |

**Rule of thumb:** if an AI surface produces a *number, category, or ranking*, a deterministic engine
already computes it on-device — the AI only rewords or extracts. The single place AI does work the
on-device code cannot is **reading pixels** (image-only receipts) — that is the one real gap.

---

## Gaps and recommendations

1. **Gap (real): image-only receipts have no on-device extraction.** Vision is the only AI feature
   without a deterministic equivalent. Recommendation: ship a **local OCR fallback** (R10) — a WASM
   OCR (e.g. Tesseract-wasm) feeding the existing `extract.Row` review pipeline — so a key-less user
   can still photo-import. Until then, the Documents page already (a) leads with CSV (R55/§8.9) and
   (b) labels the image card "Needs your OpenAI key in Settings", so the path is honest, not dead.

2. **Surfacing rule (do everywhere): label, don't block.** Any AI-gated control must (a) render even
   without a key, (b) state the key requirement inline, and (c) sit *after* its no-key sibling.
   `documents.go` now does this; audit the other AI surfaces (`insights.go`, `allocate.go`,
   `smart_strip.go` AI run-controls) to confirm the no-key alternative is presented first and the
   gated control reads as "optional accelerator", never a wall. (Follow-on C-items per surface.)

3. **Default-on free intelligence.** The Free SMART engines must be enabled by default (C254) so a
   no-key user gets recommendations immediately — the no-key path should feel *complete*, not empty,
   on first run.

4. **Cost honesty (when a key IS set).** Keep AI calls click-before-run (no silent spend) and show
   the key-storage/privacy note (done: `settings.aiKeyTrust`). Out of scope for "no-key" but part of
   the same trust contract.

---

## Acceptance / done-condition

- [x] Every AI-gated feature has a documented, on-device, no-key path (matrix above).
- [x] The one genuine gap (image OCR) is isolated and routed to R10, with an honest interim UX
      (CSV-first + labeled gated control) already shipped.
- [ ] (Follow-on) Per-surface audit confirming each AI control is presented *after* its no-key
      sibling and labeled — tracked as C-items, not part of this design.

This design is the reference for keeping CashFlux fully usable — and explainable — with zero API keys.
