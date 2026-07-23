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

## 5a. Services & cost matrix (verified 2026-07-23; re-verify before signing anything)

Every external component the business touches, with real market prices. ⚠ = promo pricing that
renews higher · (≈) = from general knowledge, not re-verified this week.

### Hosting (VPS) — top 10, min tier + one step up

| Provider | Minimum | Step up | Notes |
|---|---|---|---|
| **Hetzner** | €3.35–3.79 — CX22 2vCPU/4GB/40GB (ARM CAX11 €3.29 no-IPv4) | ≈€6.80 — CX32 4vCPU/8GB | Best price-perf; 20 TB traffic incl.; EU + limited US |
| **DigitalOcean** | $4 — 512MB/1vCPU/10GB | $6 — 1GB/1vCPU/25GB | Per-second billing since Jan 2026; §14 referral program lives here |
| **Vultr** | $2.50 — 512MB (often IPv6-only) | $6 — 1GB/25GB/2TB | Benchmarks beat Lightsail at half price |
| **Linode (Akamai)** | $5 — Nanode 1GB/25GB | ≈$12 — 2GB/50GB | Solid, boring |
| **AWS Lightsail** | ≈$5 — 512MB/20GB | ≈$7 — 1GB/40GB | Weakest per-dollar of the majors |
| **Contabo** | €6.99 — 4vCPU/8GB/100GB NVMe | ≈€12 — 6vCPU/16GB | Density king; sustained CPU lags spec |
| **Hostinger** | ⚠ ~$4.99 — KVM1 1vCPU/4GB | ⚠ ~$6.99 — KVM2 2vCPU/8GB | Renewal jump — read the term |
| **IONOS** | $2 — VPS XS 1vCPU/1GB/10GB, unmetered | ≈$5–6 — 2vCPU/2GB | Cheapest legit non-promo entry |
| **OVHcloud** | ≈$4–5 — 1vCPU/2GB | ≈$7–11 | Unmetered bandwidth, spartan panel |
| **Scaleway** | ≈€1–2 — Stardust (stock-limited) | ≈€7 — DEV1-S 2vCPU/2GB | EU-only in practice |

Decision rule: the server is one Go binary + SQLite + blobs — any row runs it. Differentiators
are **egress** (blob sync is the only real traffic), snapshots, and reputation. Cost-optimal:
Hetzner CX22. Strategy-coherent: **DO $6** while the §14 referral/Marketplace flywheel stands —
the $2–3/mo delta is noise next to Stripe fees. (Honorable mention RackNerd $22.99/yr promos:
fine for throwaways, not for paying users.)

### Domains & TLS
| Item | Cost | Notes |
|---|---|---|
| .com registration | ~$10.44/yr (Cloudflare at-cost) · ~$11 (Porkbun) · ~$13 (Namecheap) | Avoid ⚠ $1-first-year registrars with $20+ renewals |
| TLS | $0 | Caddy/Let's Encrypt |
| Ingest subdomain (mail) | $0 extra | MX records on the same domain |

### Object storage (blobs: receipts/artifacts/backups)
| Provider | Storage | Egress | Notes |
|---|---|---|---|
| **Backblaze B2** | **$6/TB/mo** ($0.006/GB) | Free up to 3× stored/mo, then $0.01/GB; free via Cloudflare (Bandwidth Alliance) | Cheapest credible; pairs with Cloudflare CDN |
| **Cloudflare R2** | $0.015/GB ($15/TB) | **$0 always** | Egress-proof; watch buried ops-request costs |
| **DO Spaces** | $5/mo base (250GB + 1TB egress incl.) | then $0.01/GB | Simplest if already on DO |
| Wasabi | $7.99/TB/mo | free | ⚠ minimum-commit + 90-day retention rules |
| AWS S3 | $23/TB/mo + $0.09/GB egress | — | No reason at our scale |

At our numbers (KB–MB datasets, 1–2 GB blob cap): even 1,000 subscribers × worst-case 2 GB =
2 TB ≈ **$12–30/mo total**. Storage is genuinely not a cost risk; pick B2+Cloudflare or Spaces.

### Email (transactional: reminders, ingest, receipts, auth)
| Provider | Cost | Notes |
|---|---|---|
| **Amazon SES** | **$0.10 per 1,000** | Cheapest; more setup (reputation, DKIM) |
| Resend | Free 3k/mo → $20/mo (50k) | Nicest DX; free tier covers launch |
| Postmark | $15/mo per 10k | Best deliverability reputation |
| Brevo | Free 300/day | Fallback tier |

Reminder math: 500 subscribers × ~20 bill-reminder emails/mo = 10k emails ≈ **$1 (SES) to
$15/mo**. Ingest inbound: SES inbound or Cloudflare Email Routing ($0) → webhook.

### SMS — priced, and recommended AGAINST for v1
| Item | Cost |
|---|---|
| Twilio outbound SMS (US) | ~$0.008/segment |
| A2P 10DLC brand registration | $4.50 one-time |
| A2P campaign fee | $1.50–10/mo |
| Carrier surcharges | +$0.003–0.005/msg (T-Mobile raised fees Jan 2026; 10k msgs/mo ≈ $30–50/mo in surcharges alone) |

Verdict: SMS adds registration bureaucracy + per-message costs for a channel email+push already
covers. **Skip in v1**; revisit only if users ask and price it as an add-on, never bundled.

### Payments
| Processor | Rate | Notes |
|---|---|---|
| **Stripe** | 2.9% + $0.30 · +0.5% Stripe Tax · +1.5% international cards | ≈**$1.46–1.66 per $39.99 annual charge** (3.7–4.2%) |
| **PayPal** | 2.99% + $0.49 standard · **3.49% + $0.49 Checkout/subscriptions** · +1.5% intl | ≈$1.89 per $39.99 (4.7%) — keep as secondary for the PayPal-only cohort; already integrated in the portal |

Annual-first billing is what keeps the fixed $0.30–0.49 per-charge fee tolerable; on a $4.99
monthly it's 9–13% of revenue — another honest argument for pushing annual.

### Compliance & business registration (Florida)
| Item | Cost | Notes |
|---|---|---|
| FL LLC formation (Sunbiz) | **$125 one-time** ($100 filing + $25 registered-agent designation) | Self as registered agent = $0/yr (address becomes public record; a service is $50–150/yr) |
| FL LLC annual report | **$138.75/yr** | Due May 1; late = $400 penalty — calendar it |
| EIN | $0 | Direct from IRS; ignore paid "services" |
| Privacy policy + ToS | $0–500 | Templates fine at launch; lawyer review before scale. **Must include:** liability cap, arbitration clause, not-financial-advice disclaimer — these do more work than any insurance policy |
| PCI | $0 | Stripe/PayPal carry it (SAQ-A) |

### Insurance (not legally required — buy at first charge)
| Item | Cost (2026) | Notes |
|---|---|---|
| **Tech E&O + cyber (bundled)** | ~$50–130/mo (~$600–1,500/yr; SaaS avg $1,516/yr, solo/low-rev quotes lower) | The one policy that fits: covers breach response AND "your app's numbers misled me" claims. Hiscox / Next / Vouch / Embroker |
| GL / BOP / workers' comp | skip | No premises, no foot traffic; FL workers' comp starts at 4+ employees |

Timing: **skip pre-revenue** (LLC + ToS clauses are the real shield); buy the bundle **the same
month charging starts** — that's when we simultaneously hold a customer database and become
worth suing. Quote-form leverage: state "no funds custody, no bank credentials, E2E-encrypted
storage — server cannot read financial data" or insurers will price us like fintech. Treat
quotes above ~$150/mo as a shop-harder signal.

### Taxes (Florida + federal) — the good news column
| Item | Rate | Notes |
|---|---|---|
| **FL sales tax on SaaS** | **$0 — electronically delivered SaaS is NOT taxable in Florida** | Physical-media software is; we ship none. Verified Jul 2026; re-check annually |
| FL personal income tax | **0%** | — |
| FL corporate income tax | 5.5% — **avoided** by LLC pass-through | Don't elect C-corp without a reason |
| Federal self-employment | 15.3% on net profit + ordinary income tax | Quarterly estimates once profit is real |
| **Other states' sales tax** | Varies — SaaS *is* taxable in NY/TX/PA/WA etc. | Economic nexus ≈ $100k/state; **Stripe Tax (0.5%) monitors + files-ready**; a non-issue until revenue is real, a solved one after |

### Reference hosting stacks
Two named stacks — lowest-cost "prove-it" (~$6–7/mo) and optimal "launch" (~$21–25/mo) — are
specified in full detail in **§15** at the end of this document, including the side-by-side
component table, rationales, trade-offs, and the switch rule (first charge = move to Stack B).

### Bottom line — the actual launch budget
| Phase | Monthly | Annual-ish |
|---|---|---|
| **Today (pre-launch)** | VPS $4–6 + domain amortized ≈ **$5–7/mo** | ~$75/yr |
| **Launch (0–500 subs)** | VPS $6 + Spaces/B2 $5–6 + SES ~$1–5 + monitoring $0 ≈ **$12–17/mo** + $138.75/yr FL + payment fees ~4% of revenue | ~$300–350/yr fixed |
| **1k subscribers ($40k ARR)** | infra ≈ $30–60/mo + included-AI tokens (§5, measure) + fees ~4% | fixed costs ≈ **2–3% of revenue** — margin lives or dies on AI tokens + support time, nothing else |

Standing rule for this matrix: prices above were verified 2026-07-23 (or marked ≈/⚠);
**re-verify any line before committing to it**, and update this section when a real bill
replaces an estimate — measured beats quoted.

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

## 15. Reference hosting stacks — full detail

> Pick one of these two, don't improvise a third. Both run the identical software — one Go
> binary + Caddy auto-HTTPS + SQLite + an S3-compatible blob interface — so moving A→B is an
> rsync + DNS flip, not a migration project. Prices verified 2026-07-23.

### 15.1 Side-by-side component table

| Component | **Stack A — "Prove-it" (lowest cost)** | A cost/mo | **Stack B — "Launch" (optimal)** | B cost/mo | Why the choice differs |
|---|---|---|---|---|---|
| **VPS** | Hetzner **CPX11**, Ashburn US-east — 2 vCPU AMD / 2 GB RAM / 40 GB NVMe / 20 TB traffic | ~$5.00 | DigitalOcean **Basic $12** — 1 vCPU / 2 GB RAM / 50 GB NVMe / 2 TB transfer | $12.00 | A buys the best raw price-perf on the market; B buys the provider where the §14 referral/Marketplace flywheel pays and resize-in-place upgrades |
| **Blob storage** (receipts/artifacts) | On the VPS's 40 GB disk — covers first ~100+ users at realistic usage | $0 | **DO Spaces** — 250 GB storage + 1 TB egress included, S3-compatible | $5.00 | A defers the object-store decision until real usage data exists; B separates blobs from the box so droplet resize/rebuild never touches user data |
| **Server backups** | Nightly encrypted snapshot pushed off-box to **Backblaze B2** ($0.006/GB) | ~$0.10–0.50 | **DO automated backups** (20% of droplet price) **+ the same nightly B2 push** (offsite, different provider) | $2.40 + ~$0.50 | A gets off-box durability for pennies; B adds provider-level restore *and* keeps the cross-provider copy — the server-side half of the "privacy ≠ durability" promise (FB6) |
| **Email — outbound** (reminders, receipts, auth) | **Amazon SES** — $0.10 per 1,000 | ~$0–1 | **SES** (same); upgrade path: Postmark $15/mo *only if* deliverability measurably hurts | ~$1–5 | Volume, not vendor, is the difference; the Postmark trigger is a measured bounce/spam rate, not vibes |
| **Email — inbound** (ingest address) | **Cloudflare Email Routing** → webhook | $0 | Same | $0 | Free and provider-neutral both sides |
| **Domain** | Cloudflare Registrar at-cost .com (~$10.44/yr) | ~$0.87 | Same | ~$0.87 | No reason to differ |
| **DNS / CDN / DDoS front** | Cloudflare free tier, proxied | $0 | Same | $0 | Also gives blob-egress relief via Bandwidth Alliance if B2 serves anything |
| **App/wasm delivery** | **Serve `main.wasm.gz` + static shell from the Cloudflare edge cache**, not the VPS | $0 | Same | $0 | The wasm bundle is the single heaviest asset every visitor downloads — edge-caching it removes the bulk of origin egress AND fixes global first-paint latency (feeds FB1 boot perception). **Non-negotiable detail:** cache with immutable, content-hashed filenames (`main.<hash>.wasm.gz`), never TTL-cached mutable names — the known stale-`wasm.gz`-runs-old-code landmine becomes a CDN-wide incident otherwise. Keep pre-compressed gzip semantics correct (Content-Encoding) or let Cloudflare re-encode brotli |
| **Monitoring** | UptimeRobot + healthchecks.io free tiers | $0 | DO monitoring (included) + healthchecks.io | $0 | — |
| **Status page** | none (beta — no uptime promises made) | $0 | **Instatus** free tier | $0 | A deliberately makes no promises; B starts making them the day money moves |
| **TLS** | Caddy / Let's Encrypt | $0 | Same | $0 | — |
| **TOTAL** | | **~$6–7/mo** | | **~$21–25/mo** | |

### 15.2 Capacity, risk, and economics detail

| Dimension | Stack A | Stack B |
|---|---|---|
| Realistic capacity | Beta cohort → low hundreds of users | ~1,000–2,000 subscribers before first resize |
| Single point of failure | **Yes — one box.** Tolerable *only* with nightly off-box backups and zero uptime promises | Box still single, but blobs externalized + two backup layers; restore = new droplet + rsync + DNS |
| Latency (US users) | Fixed by choosing Ashburn (the reason CPX11, not the cheaper EU-only CX22) | US regions native |
| Egress ceiling | 20 TB included (a non-thought) | 2 TB droplet + 1 TB Spaces (fine; Cloudflare front absorbs static) |
| Referral coherence (§14) | **Forfeited** while A runs | Aligned — we host where we send self-hosters |
| Upgrade path | Move to B (rsync + DNS) | Resize droplet in place; Spaces scales linearly |
| Cost as % of revenue | Pre-revenue by definition | **7 subscribers cover the stack**; ~0.7% of revenue at 1k subs |
| Rejected alternative | IONOS $2 (1 GB RAM): saving $3/mo to lose all headroom for backup jobs and traffic spikes is false economy | Bigger droplet up front: paying for headroom before measurement contradicts §5's measure-first rule |

### 15.3 Rationales (the reasoning, not just the rows)

- **Stack A exists to make the funnel experiment nearly free.** The entire stack costs less per
  month than one subscriber pays per year, so running the beta for six months is a ~$40
  question. Everything deferred (object store, status page, provider backups) is deferred
  *because no promise requiring it has been made yet*.
- **Stack B is "optimal" as in: the cost floor consistent with the strategy** — not the
  cheapest possible number. One provider for compute + blobs + backups keeps the solo-operator
  ops surface tiny; hosting on DO while pointing self-hosters at DO referrals is coherent
  rather than hypocritical; 2 GB and external blobs buy headroom for import bursts and the AI
  proxy without touching architecture.
- **The switch rule is a calendar trigger, not a judgment call:** run A through beta; move to B
  **the month billing turns on** — deliberately the same trigger as buying the §5a insurance
  bundle, so "the month money starts moving" is one checklist (Stack B + insurance + status
  page + published SLA posture), not three separate decisions.
