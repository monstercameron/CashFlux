# CashFlux Cloud — Business Plan

> The CashFlux app is free and local-first. **CashFlux Cloud** is a paid subscription that adds
> multi-device **sync**, encrypted **backup**, the **server-only conveniences** (reminders, ingest,
> feeds), and **AI two ways** (a capped included tier + uncapped BYO-key). This plan covers
> positioning, pricing, unit economics, and go-to-market. Numbers are planning estimates to
> validate, not promises. *(Refined 2026-07-23 for honesty + profitability after the full
> competitive teardown — see `docs/COMPETITIVE_TEARDOWN.md`.)*

## 1. Summary
- **Free:** the full app, on one device, offline, forever (MIT-licensed, owned). This is a
  standing guarantee, not a launch promo: **no existing local feature ever moves behind Cloud.**
  We gate *operations and server-only capabilities*, never core capability (§13).
- **Paid (CashFlux Cloud):** sync + encrypted backup across devices, the server-only bundle
  (§2a), and AI two ways — an **included, capped AI tier** (no key, no setup) and the **BYO-key
  proxy** (uncapped, user pays their provider directly, key stored encrypted off the browser).
- **Why it stays cheap & high-margin:** the dataset is small (KB–low MB; blobs capped), the
  server is a cheap Go binary, and BYOK AI costs us nothing. The included AI tier *does* carry a
  real per-user token cost — it is capped, measured, and priced in honestly (§5) rather than
  assumed away. Prior versions of this plan claimed "AI cost: $0 to us"; that stops being true
  the day the included tier ships, so the economics below no longer pretend otherwise.

## 2. Value proposition
- **Own your data, and still sync it.** Local-first means the data lives on your device; Cloud adds
  convenience without lock-in (export anytime; cancel → keep everything locally).
- **The zero-knowledge AI claim (say it precisely).** Synced data is envelope-encrypted;
  **our servers cannot read your finances — including when you use our AI.** The client
  assembles the minimal context locally and sends it to the model provider through our proxy;
  the proxy meters usage and holds keys, it never decrypts your stored data. Honest fine print
  we always publish next to the claim: the *model provider* does see the context of each
  question (same as PiggySize discloses for Claude); the AI module has a hard off-switch; the
  deterministic engine does all arithmetic, the model only narrates.
- **AI without the key ceremony — or with it.** Included tier: it just works, capped and
  disclosed. BYOK: uncapped, your provider bill, your models. No competitor offers both.
- **Cheaper than the incumbents**, with a privacy/ownership story they can't copy without
  destroying their own revenue.

## 2a. What Cloud actually includes (the bundle — because "just sync" undersells)
Willingness-to-pay for bare sync is the plan's known weak point; the teardown identified the
server-only capabilities users already pay competitors for. Cloud is all of these, sequenced in §11:
1. **Sync + encrypted backup** (multi-device, envelope-encrypted, restore preview).
2. **Bill reminders that reach a closed tab** — email/push before due dates (PS3; PiggySize's
   most-loved cheap feature; fundamentally impossible local-only).
3. **Email-ingest imports** — forward a bank statement/receipt email to your private
   `@ingest.cashflux` address → lands in the import drafts queue (the Firefly pattern; directly
   attacks the manual-import freshness chore that is our #1 structural weakness).
4. **Feeds:** FX rates auto-refresh, delayed/EOD security quotes, and later Zillow/VinAudit-class
   asset valuations — the "numbers stay current" tier of value.
5. **Included AI tier + BYOK proxy** (§1).
6. Later: **household multi-login** (separate logins, one subscription — the Monarch/PiggySize
   family wedge) as the higher-ARPU plan.
Everything on this list is operations or a third-party feed — nothing is a paywalled app feature.

## 3. Target customer
Privacy-conscious budgeters and prosumers; ex-Mint / YNAB / Monarch users seeking lower cost + data
ownership; the "self-host-curious" who want the benefits without running a server. Households are a
later, higher-ARPU segment.

## 4. Pricing (recommended, validate)
- **Annual-first:** **$39.99 / year** (~$3.33/mo), shown first as the default. (Raised from the
  prior $34.99 to honestly cover the included-AI token cost with headroom; still the cheapest
  hosted option in the roster by 2×.)
- **Monthly:** **$4.99 / month** (anchors annual as the deal).
- **30-day free trial, no card up front** (matches PiggySize's proven pattern; a 14-day
  card-gated trial optimizes conversion optics, not user trust — and trust is the brand).
- **Household plan:** later phase, ~$69–79/yr for separate logins in one workspace.
- Positioning vs market (mid-2026 verified): **YNAB $109/yr, Monarch $99.99/yr (+Plus tier
  above), Copilot ~$95/yr, PiggySize $90/yr ($9/mo, AI included), Tiller $79/yr, Simplifi
  ~$48/yr, Actual = free/self-host.** CashFlux Cloud undercuts every hosted option; PiggySize
  is the price-pressure comp to watch (§9), Simplifi the value-anchor floor.

## 5. Unit economics (honest version)
- **Included AI tier: a real, capped cost.** Cap ~50 assistant messages/day (PiggySize-parity,
  disclosed in-product); small-model default; the deterministic engine does all math so prompts
  stay short. Planning estimate **$0.10–0.50/user/month blended** (most subscribers use far less
  than the cap; heavy users hit it). This is an estimate to *measure from day one* — per-user
  token metering is a launch requirement, not a nice-to-have. Kill-switch: if blended cost
  exceeds ~$1/user/mo, tighten the cap/model before touching price.
- **BYOK AI: $0** to us (user pays their provider; proxy meters only).
- **Storage/bandwidth:** dataset KB–low MB; blobs behind a fair-use cap (1–2 GB) →
  **single-digit cents/user/month**; compute is one cheap Go binary amortized across users.
- **Email/push reminders + ingest:** transactional email at ~$0.001/msg → cents/user/month.
  Feeds (FX/EOD quotes) are free-tier APIs at launch scale; valuation feeds priced when added.
- **Stripe fees:** ~2.9% + $0.30/charge (annual billing minimizes per-charge overhead).
- **Implied gross margin: ~80–85%** at $39.99/yr with the included AI tier, ~90%+ for
  BYOK-only users. (The previous "~90%+ / AI costs us $0" line was true only while AI was
  BYOK-only; this version prices the real bundle.)
- **Fixed costs:** hosting (1 small VM/managed host to start), domain/TLS, monitoring — and
  **dev/support time, which dominates everything else at small scale**. The margin numbers
  above exclude labor; at <1k subscribers this is a margin-positive side project, not an income.
- **Guardrails:** storage fair-use cap; AI message cap; content-addressed blob dedup; overage →
  soft prompt to prune / BYOK, never surprise charges.

## 6. Funnel & assumptions (to instrument, not assume)
Free local users → discover Cloud (calm in-app prompts) → trial → paid. Honest calibration:
- **2–5% trial start** is plausible; the prior **30–50% trial→paid was optimistic** — industry
  norms for no-card consumer trials are **10–25%**. Plan on **15–25%**, celebrate above.
- ~5–8% annual churn assumed; consumer-finance apps often run higher (10–20%) — measure.
- Re-run sanity check at honest rates: 10k active local × 3% trial × 20% convert ≈ **60 paying**
  ≈ $2.4k ARR; 100k active ≈ **$24k ARR**. The lever is unambiguous: **top-of-funnel free-app
  reach**, which is why GTM (§7) and the free tier's completeness matter more than price tuning.
- Corollary worth stating plainly: this business does not pay a salary below ~50–100k active
  local users. It is high-margin convenience revenue on top of an owned product, and the plan
  should not make decisions (pricing, dark patterns, feature-gating) that trade the product's
  trust position for near-term conversion.

## 7. Go-to-market
- **In-app** is the primary channel: the free app is the funnel; calm, contextual Cloud prompts.
- **Communities:** r/ynab, r/personalfinance, r/selfhosted, Hacker News (local-first + privacy angle).
- **Product Hunt / Show HN** launch around the local-first + BYO-AI story.
- **Content:** posts on local-first finance, "own your budget data", BYO-AI — SEO + credibility.
- **Word of mouth** from the data-ownership differentiator.

## 8. Competitive positioning
| Product | Price/yr | Local-first | AI included | BYO AI | Note |
|---|---|---|---|---|---|
| YNAB | $109 | No | No | No (public API though) | Method + brand |
| Monarch | $99.99 (+Plus tier) | No | Yes | No | Aggregation + planning tier |
| Copilot | ~$95 | No | Yes (invisible ML) | No | Apple + web, USD-only |
| PiggySize | $90 ($9/mo) | No (manual-entry, server-stored) | **Yes (Claude, 50 msgs/day free)** | No | The price-pressure comp |
| Tiller | $79 | No (Sheets) | No | No | Spreadsheet |
| Simplifi | ~$48 | No | No | No | The value floor |
| Actual | Free/self-host | Yes | No | No | DIY twin |
| **CashFlux** | **~$40** | **Yes** | **Yes (capped)** | **Yes (uncapped)** | Free app + cheap cloud + data ownership |

Wedge: the only one that is **free + genuinely local + cheap optional cloud + AI both ways** —
and the only one whose paid tier a competitor can't copy without abandoning their own model
(aggregators can't go local; Actual won't go managed-with-AI).

## 9. Risks & mitigations
- **Low willingness to pay for "just sync."** → §2a bundle: reminders, email-ingest, feeds, AI.
  Lead marketing with reminders + "your numbers stay current," not storage.
- **Included-AI cost blowout.** → hard per-user caps, small-model default, greedy clamp on tool
  calls, per-user metering from day one, kill-switch threshold (§5), overflow path = BYOK.
  Never eat unbounded tokens to look generous.
- **PiggySize-style price pressure** ($9/mo with included AI). → we win on capability + locality
  at $40/yr; do not chase their $0-tier AI giveaway — our free tier's answer is BYOK.
- **Optimistic funnel math.** → §6 now uses honest conversion rates; instrument before scaling
  spend; no paid CAC until organic trial→paid is measured.
- **Storage cost from image blobs.** → fair-use cap, dedup (content-addressed), prune prompts.
- **Churn / support load** at consumer scale. → strong self-serve (Stripe portal), good empty/error UX,
  docs.
- **Single-maintainer / hosted-service reliability.** → simple stateless Go + SQLite/blob backups;
  status page; conservative SLAs — and honest ones: no "99.9%" claims a solo operator can't keep.
- **Trust regression risk (the brand-killer).** → standing rules, published: free tier never
  loses features to Cloud; caps and limits disclosed in-product; cancel → everything keeps
  working locally; no dark patterns (no countdown timers, no fake discounts, no card-gated
  trials); the AI off-switch is global and honored.
- **Compliance:** payments via Stripe (PCI handled); publish privacy policy + terms; honor data
  export/delete (GDPR/CCPA). Keep these ready before charging.

## 10. Metrics to track
MAU (local), Cloud sign-ups, trial starts, **trial→paid**, MRR/ARR, churn, ARPU, **gross margin**,
storage cost/user, CAC (mostly $0 organic to start), LTV.

## 11. Sequencing (ties to the backend rollout)
The backend ships in phases (auth+sync → blobs → proxy/feeds). Monetization timing, revised so
every beat adds a server-only value users can name:
1. **Launch paid at the sync milestone** (auth + snapshot sync + Stripe billing + 30-day trial).
   Headline: multi-device sync + encrypted backup.
2. **Bill reminders (email/push)** — the first "my closed tab still works for me" beat; small
   build, outsized retention. Ship close behind launch.
3. **Add blob store** (artifact sync) — improves the product, no pricing change.
4. **AI proxy: BYOK first** (key off the browser; $0 token cost), then the **included capped
   tier** once per-user metering is proven — that's the marquee marketing beat ("AI that can't
   read your data"), priced-in per §5.
5. **Email-ingest imports + FX/quote feeds** — the "numbers stay current" beat.
6. **Household plan** (separate logins) for ARPU expansion.

## 12. Open decisions
- Ratify $39.99/$4.99 vs the prior $34.99/$3.99 (decision input: measured included-AI cost/user).
- Included-AI cap shape: msgs/day vs token budget/month; which small model; clamp policy.
- Storage fair-use cap (and overage behavior — prompt-to-prune only, never surprise charges).
- Refund policy; annual-only vs monthly availability at launch.
- Entity/tax setup for taking payments (Stripe account, jurisdiction).
- When (if ever) a Plus-style tier (retirement engine PS1, business books PS2, multi-login PS4)
  splits out above Cloud — Monarch Plus validates the slot; do not open it before those exist.

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
