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

## 13. Self-hosting & the open-source model

CashFlux (app **and** server) is open source, so anyone can run their own private server for free.
This is deliberate, not a leak in the model — it's the **Actual / Bitwarden / Obsidian-Sync pattern**:
the software is free and self-hostable; the paid product is *managed convenience*.

- **What you pay for is operations, not features:** CashFlux Cloud = zero-ops hosting, backups,
  uptime, OAuth, billing, and "it just works" across devices. Self-hosters trade money for time + a
  server.
- **It strengthens, not weakens, the brand.** Self-hosting is the ultimate proof of the data-ownership
  promise; it builds trust that converts the *non*-technical majority to the paid tier. Most users who
  *could* self-host still won't — the convenience gap is the business.
- **Pricing implication:** self-host caps how high we can price (the free alternative is always there),
  which is exactly why the plan targets a **low, convenience-justified price** (~$35/yr) rather than
  competing on locked-up features. Don't gate core capability behind Cloud — gate *operations*.
- **Support boundary:** self-host is community-supported (docs + issues); paid support is part of Cloud.
- **Risk it mitigates:** lock-in fear and "what if the company dies" — answered by "run it yourself."
- **GTM upside:** the open-source + self-host story is itself marketing in privacy/self-host communities
  (r/selfhosted, HN), feeding the top of the free-app funnel that Cloud monetizes.

Net: self-hosting and the paid Cloud are complementary. Keep the server genuinely runnable (single
binary, simple token auth option, Docker quickstart), and let Cloud win on convenience.

## 14. Turnkey self-host deploy + referral revenue (DigitalOcean)

Make self-hosting *one click* and turn that free path into a second revenue stream — without
compromising the open, no-dark-patterns promise.

### Easy deploy (ship in order)
1. **cloud-init user-data script** (now, no approvals): paste-on-create installs Docker + the CashFlux
   server image (compose) + **Caddy auto-HTTPS** + prints the first-run access token. One paste → a
   running, TLS'd server.
2. **One-command installer** (`curl … | sh`) for any fresh VPS — same recipe, host-agnostic.
3. **DO Marketplace 1-Click image** (later): Packer-built snapshot submitted to the DigitalOcean
   Marketplace ("Create CashFlux Droplet"). More discoverable/trusted; carries the referral; needs
   DO vendor approval. All three reduce to: Docker image + compose + Caddy + printed token (server is
   already a single binary).

### Referral monetization (the flywheel)
- DigitalOcean's **referral program** credits new sign-ups (DO has run ~$200/60-day promos) and pays the
  referrer **account credit (~$25)** once the referee spends a threshold.
- Put **our DO referral link** on the "Deploy your own server" button / install docs / Marketplace
  listing. Self-hosters who create a DO account via it earn us DO credit — which **offsets the cost of
  running CashFlux Cloud itself**.
- **Why it's complementary, not cannibalizing:** the free self-host path is great for brand + top-of-
  funnel; with referral it *also lowers our Cloud infra bill* and monetizes users who'll never buy a
  subscription. Two revenue paths (Cloud subs + DO referral credit) reinforce each other.

### Discipline / caveats
- Referral payout is **account credit, not cash** — it reduces our DO bill (≈ Cloud COGS); model it as
  reduced cost, not a paycheck.
- **Verify current DO terms** (amounts/thresholds change; promos can end). Don't hard-depend on it.
- **Disclose it plainly** ("deploy via this link → you get DO credit, and it supports CashFlux") — honest
  and required by DO's ToS (no misrepresenting referrals).
- Keep an unconditional **plain self-host path** (any host, your own account, no referral) — the free
  promise stays absolute.
- Marketplace 1-Click may carry separate partner economics — check if/when going that route.

### Referral-fraud guardrails
- Treat referral attribution as accounting metadata only. It must never unlock product features, support priority,
  discounts, trial extensions, sync quota, AI quota, or deployment behavior.
- Detect obvious self-referral/farming before counting referral economics: same CashFlux account, same billing
  contact, repeated same-card/customer evidence from Stripe, repeated same OAuth identity, or bursty signups from
  the same operator-controlled campaign source.
- If a referral looks suspicious, exclude it from internal COGS/revenue modeling and keep the user experience
  unchanged; do not punish app access based on referral outcome.
- Keep the non-referral self-host path equally visible so the referral link remains optional and disclosed.
