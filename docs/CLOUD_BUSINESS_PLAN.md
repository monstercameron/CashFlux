# CashFlux Cloud — Business Plan

> The CashFlux app is free and local-first. **CashFlux Cloud** is a paid subscription that adds
> multi-device **sync**, encrypted **backup**, and the **AI proxy**. This plan covers positioning,
> pricing, unit economics, and go-to-market. Numbers are planning estimates to validate, not promises.

## 1. Summary
- **Free:** the full app, on one device, offline, forever (MIT-licensed, owned).
- **Paid (CashFlux Cloud):** sync + backup across devices + AI proxy (BYO key, stored encrypted,
  off the browser). Annual-first subscription with a free trial.
- **Why it can be cheap & high-margin:** AI tokens are **bring-your-own-key** — the user pays OpenAI
  directly, so we carry *no* per-token cost. Our only variable cost is storage/bandwidth/compute for
  small datasets + image blobs. That makes a low price sustainable.

## 2. Value proposition
- **Own your data, and still sync it.** Local-first means the data lives on your device; Cloud adds
  convenience without lock-in (export anytime; cancel → keep everything locally).
- **Secure AI without the cost.** Your key never sits in the browser; the proxy meters and rate-limits;
  you still pay only your own OpenAI usage.
- **Cheaper than the incumbents**, with a privacy/ownership story they don't have.

## 3. Target customer
Privacy-conscious budgeters and prosumers; ex-Mint / YNAB / Monarch users seeking lower cost + data
ownership; the "self-host-curious" who want the benefits without running a server. Households are a
later, higher-ARPU segment.

## 4. Pricing (recommended, validate)
- **Annual-first:** **$34.99 / year** (~$2.92/mo), shown first as the default.
- **Monthly:** **$3.99 / month** (anchors annual as the deal).
- **14-day free trial** (recommend **no card up front** to maximize trial starts; revisit if
  trial→paid is weak).
- **Household plan:** later phase, ~$59–69/yr for shared workspaces / multiple accounts.
- Positioning vs market: **YNAB $109/yr, Monarch $99/yr, Copilot $95/yr, Tiller $79/yr, Actual = free/
  self-host.** CashFlux Cloud undercuts all hosted options while keeping the app itself free.

## 5. Unit economics
- **AI cost: $0** to us (BYO key).
- **Per-user variable cost (estimate):** synced dataset is small (KB–low MB after artifacts move to
  blobs); image/dataset blobs are the main driver. Assume a fair-use storage cap (e.g. 1–2 GB blobs)
  → object storage + bandwidth on the order of **single-digit cents/user/month**; compute is a cheap
  Go binary amortized across users.
- **Stripe fees:** ~2.9% + $0.30/charge (annual billing minimizes per-charge overhead).
- **Implied gross margin: ~90%+** at $35/yr after storage + fees, for typical usage.
- **Fixed costs:** hosting (1 small VM/managed host to start), domain/TLS, monitoring, and dev/support
  time (the real cost at small scale).
- **Guardrail:** a storage fair-use cap protects margin from heavy image users; overage → soft prompt
  to prune or (later) a higher tier.

## 6. Funnel & assumptions (to instrument, not assume)
Free local users → discover Cloud (in-app prompts) → trial → paid. Planning assumptions to test:
- 2–5% of active local users start a trial; 30–50% trial→paid; ~5–8% annual churn.
- Example sanity check: 10k active local users × 3% trial × 40% convert ≈ **120 paying** → ~$4.2k ARR;
  at 100k users ≈ **$42k ARR**. Modest but high-margin; the lever is top-of-funnel (free app reach).

## 7. Go-to-market
- **In-app** is the primary channel: the free app is the funnel; calm, contextual Cloud prompts.
- **Communities:** r/ynab, r/personalfinance, r/selfhosted, Hacker News (local-first + privacy angle).
- **Product Hunt / Show HN** launch around the local-first + BYO-AI story.
- **Content:** posts on local-first finance, "own your budget data", BYO-AI — SEO + credibility.
- **Word of mouth** from the data-ownership differentiator.

## 8. Competitive positioning
| Product | Price/yr | Local-first | BYO AI | Note |
|---|---|---|---|---|
| YNAB | $109 | No | No | Method + brand |
| Monarch | $99 | No | No | Aggregation-heavy |
| Copilot | $95 | No | No | iOS-centric |
| Tiller | $79 | No (Sheets) | No | Spreadsheet |
| Actual | Free/self-host | Yes | No | DIY |
| **CashFlux** | **~$35** | **Yes** | **Yes** | Free app + cheap cloud + data ownership |

Wedge: the only one that is **free + local-first + cheap optional cloud + BYO AI**.

## 9. Risks & mitigations
- **Low willingness to pay for "just sync."** → Bundle AI proxy + backup; lead with multi-device + the
  privacy story; keep price low.
- **Storage cost from image blobs.** → fair-use cap, dedup (content-addressed), prune prompts.
- **Churn / support load** at consumer scale. → strong self-serve (Stripe portal), good empty/error UX,
  docs.
- **Single-maintainer / hosted-service reliability.** → simple stateless Go + SQLite/blob backups;
  status page; conservative SLAs.
- **Compliance:** payments via Stripe (PCI handled); publish privacy policy + terms; honor data
  export/delete (GDPR/CCPA). Keep these ready before charging.

## 10. Metrics to track
MAU (local), Cloud sign-ups, trial starts, **trial→paid**, MRR/ARR, churn, ARPU, **gross margin**,
storage cost/user, CAC (mostly $0 organic to start), LTV.

## 11. Sequencing (ties to the backend rollout)
The backend ships in phases (auth+sync → blobs → AI proxy). Recommended monetization timing:
1. **Launch paid at the sync milestone** (auth + snapshot sync + Stripe billing + trial). Headline:
   multi-device sync + backup.
2. **Add blob store** (artifact sync) — improves the product, no pricing change.
3. **Add AI proxy** as a marquee Cloud upgrade (key off the browser) — marketing beat, still $0 token
   cost to us.
4. **Household plan** later for ARPU expansion.

## 12. Open decisions
- Final price points + trial length + card-up-front?
- Storage fair-use cap (and overage behavior)?
- Refund policy; annual-only vs monthly availability at launch.
- Entity/tax setup for taking payments (Stripe account, jurisdiction).
