<!-- SPDX-License-Identifier: MIT -->
# Security review — data leaving the device on AI calls

**Scope of this review:** every code path where CashFlux transmits user financial
data off the device as part of an AI feature (Insights Q&A, "explain", affordability
answers, and document vision import). It documents *what* is sent (scope) and *how*
it is minimised (redaction / data-minimisation), and records residual risks and
recommendations. Reviewed against `internal/aicontext`, `internal/aiprompt`,
`internal/ai` (transport), and `internal/insights`.

Last reviewed: 2026-06-24. Re-run this review whenever the context builder, the
tier model, or the transport changes.

## 1. Trust model

- **Bring-your-own-key, client-side.** AI calls are made directly from the wasm
  client to the configured base URL (`internal/ai/transport.go` — the *only* place
  that touches the network) using the user's own API key (`ai.DefaultBaseURL` =
  `https://api.openai.com/v1`, overridable for a self-host proxy). There is no
  CashFlux-operated intermediary in the default path: the data goes to the user's
  own OpenAI account under their own terms.
- **Opt-in.** No AI call happens until the user supplies a key in Settings and
  invokes an AI feature. Local budgeting/planning/reports never call out.
- **Key storage.** The key lives in local prefs (optionally remembered) or, for the
  cloud tier, is uploaded once to the user's own server (`SetKey`, removable via the
  AI-key Remove flow + `DeleteAIKey`). The key is never embedded in screenshots,
  logs, or the context block.

## 2. Scope — what is sent

The context builder (`internal/aicontext`) is the choke point: it assembles a
**bounded, structured summary** of finances for the model's system prompt. The
governing rule (documented in the package): *inject a summary, never the raw
ledger* — bounded by top-N / recent-N and gated by an opt-in privacy tier; the
agent pulls further detail only via explicit tools (C82).

Data is gated by a **privacy `Tier`** (least → most revealing; the default is the
most conservative):

| Tier | Adds to the payload |
|------|---------------------|
| `TierAggregates` (default) | Net worth, period income/expense, account **count** only |
| `TierFormulas` | + the user's enabled KPIs, evaluated to display values |
| `TierBreakdowns` | + accounts (name/type/balance), budgets, goals, top categories/payees |
| `TierTransactions` | + recent transactions (date, description, amount, category) |

Every list is **capped**: `TopN` (default 5) per breakdown, `RecentN` (default 10)
recent transactions. Money and dates arrive **pre-formatted** as display strings, so
the builder stays pure (no `appstate`/`money`/`syscall/js`) and unit-tests natively.

## 3. Redaction / data-minimisation controls (present today)

1. **Conservative default tier.** Out of the box only aggregates leave the device —
   no account names, payees, or transactions.
2. **Opt-in escalation.** Higher tiers (B17/C45 privacy shift) are a deliberate,
   user-initiated trade of privacy for answer quality.
3. **Bounded volume.** Top-N / recent-N caps prevent bulk exfiltration of the ledger
   even at the most revealing tier.
4. **Summary, not raw rows.** The full transaction store is never serialised; the
   model must request specifics through tools, which keeps egress observable.
5. **No secrets in context.** The API key, server token, and CSRF value are never
   placed in the prompt block.

## 4. Residual risks & recommendations

- **R1 — Verbatim descriptions/payees at `TierBreakdowns`+.** Transaction
  descriptions and payee labels are user-entered free text and can contain PII
  (merchant names, person names, memos). At `TierTransactions` they are sent
  verbatim. *Recommendation:* offer an optional payee/description scrub (hash or
  truncate) for users who opt into transaction-level sharing; surface the tier and a
  one-line "what this shares" note at the point of opt-in.
- **R2 — Third-party retention.** Once data reaches OpenAI (or a user proxy) it is
  governed by that provider's retention/training terms, outside CashFlux's control.
  *Mitigation:* the BYO-key model means the user already controls that account and
  its data-use settings; document this in the AI settings copy.
- **R3 — Vision import.** Document/image import (`internal/ai/vision.go`) sends image
  bytes for extraction. *Recommendation:* warn before first upload that the image
  leaves the device, and dedupe/review the extracted rows locally (already done) so
  nothing is auto-committed.

## 5. Verdict

The AI egress path is **minimised by design**: a single network choke point, an
opt-in conservative-default tier model, hard volume caps, and a summary-not-raw-row
rule. No secrets are included in prompts. The open items are incremental hardening
(optional free-text scrub at the transaction tier, explicit point-of-opt-in
disclosure, vision warning) rather than structural fixes — there is no path today
that ships the raw ledger or the user's keys off-device.
