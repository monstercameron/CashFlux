# CashFlux Cloud — UX Layer Design

> The paid tier ("**CashFlux Cloud**") adds multi-device **sync**, encrypted **backup**, and the
> **AI proxy** on top of the free, local-first app. This is the in-app UX for it. See
> [`docs/BACKEND_PLAN.md`](./BACKEND_PLAN.md) (server) and [`docs/CLOUD_BUSINESS_PLAN.md`](./CLOUD_BUSINESS_PLAN.md)
> (monetization). No code — design only. Build it on the existing candidate-C design system
> (FlipPanel, toast, segmented controls, rail) per the CLAUDE.md UI rules.

## Principles
- **Local-first is never gated.** Every existing feature keeps working offline, free, forever.
  Cloud is purely additive and opt-in. Cancelling = sync stops, your data stays on the device.
- **Honest & calm.** No dark patterns. Clear price, "cancel anytime", "export anytime",
  plain-English benefits. Privacy stated up front (your data, your AI key).
- **Low friction, clear status.** One obvious place to sign in; an always-visible, glanceable sync
  state; contextual (not nagging) upgrade prompts.

## Surfaces (where it lives in the app)

1. **Cloud section in Settings** (global settings FlipPanel) — the home for everything Cloud:
   - Signed out: a short value pitch + **Sign in with Google / GitHub** buttons, and a "What is
     CashFlux Cloud?" link.
   - Signed in: plan status (Free / Trial / Active / Past due), **Sync status**, **Manage
     subscription**, **AI key**, **Devices**, **Sign out**, **Export / Delete account**.

2. **Sync status chip** — a small, glanceable indicator by the workspace switcher in the rail (and
   mirrored in the top bar on mobile). States: `Synced ✓` (with "last synced 2m ago" tooltip),
   `Syncing ⟳`, `Offline ☁` (queued changes count), `Error !`, and a subtle `Not signed in` (a cloud
   outline). Click → opens the Cloud settings section. A **"Sync now"** action lives here too.

3. **Upgrade prompt (contextual paywall)** — when a *free, signed-out/free* user taps a Cloud-only
   action (the sync chip, "Add another device", the AI-via-Cloud toggle), show a non-blocking sheet:
   the 3 benefits (sync, backup, AI proxy), the price, **Start 14-day free trial**, and a "Maybe
   later" dismiss. It never blocks a local feature — only the cloud action that triggered it.

4. **Plan / pricing screen** — benefits list, an **Annual / Monthly** segmented toggle (annual shown
   first, "save ~30%"), the price, trial note, and a **Subscribe** CTA → Stripe Checkout (redirect).
   Trust line underneath: *cancel anytime · export anytime · your data is encrypted · powered by
   your own AI key.*

5. **First-run / onboarding moment** — one calm line: *"Your data stays on this device. Add CashFlux
   Cloud anytime to sync and back up across devices."* Dismissible; never a wall.

6. **AI key setup (Cloud)** — for Cloud users, the OpenAI key moves here: enter once, stored
   **encrypted server-side**, shown as `Key set •` (never re-displayed), with "Replace" / "Remove".
   Free users keep the existing client-side key field (unchanged). Copy explains the difference: with
   Cloud, the key leaves the browser and is used via the proxy.

7. **Devices** — list of devices syncing this account (name, last seen); **Revoke** a device.

8. **Billing management** — **Manage subscription** opens the Stripe customer portal (redirect):
   change plan, update card, cancel. In-app we show plan, renewal date, and status only.

9. **Conflict / LWW message** — a quiet toast when a newer version was pulled from the server:
   *"Synced — a newer version from another device replaced your local copy."* (LWW is the chosen
   model; the message keeps it non-surprising.)

## Account & subscription states (define each explicitly)
- **Signed out (free):** local-only; sync chip shows "Not signed in"; Cloud actions show the upgrade
  sheet.
- **Signed in, Free:** account exists but no subscription; sync/proxy disabled; upgrade prompts.
- **Trial (active):** full Cloud; banner "Trial — N days left, then $X/yr"; one-tap subscribe.
- **Active:** full Cloud; quiet synced state; renewal date in settings.
- **Past due (grace):** sync still works for a short grace window; gentle banner "Update payment to
  keep syncing"; link to portal.
- **Canceled / lapsed:** **graceful downgrade to local** — sync/proxy stop, *all data remains on the
  device and exportable*; chip returns to "Not signed in" style with a "Resubscribe" affordance. No
  data is deleted or held hostage.

## Key flows
- **Subscribe:** Cloud settings → Pricing → Stripe Checkout → return → trial/active; first sync runs;
  chip → Synced.
- **Add a 2nd device:** install/open app on device 2 → Sign in → pull latest → Synced. (Surfaces the
  value moment immediately.)
- **Cancel:** Manage subscription (portal) → cancel → at period end, downgrade-to-local toast +
  settings reflect Free; data untouched.
- **Delete account:** explicit, double-confirmed; removes server data + blobs; local data stays.

## States, a11y, copy
- Cover empty / loading / offline / error for every Cloud surface (sign-in failure, payment failure,
  sync error with retry). Offline always keeps the app usable.
- Keyboard-reachable and labelled (the existing focus-ring + ARIA conventions); the sync chip is a
  real button with an accessible name reflecting state.
- Plain, friendly English everywhere; money and dates honor the user's existing format preferences.

## Open UX questions
- Exact placement of the sync chip (rail vs top bar) at each breakpoint.
- Trial length copy + whether trial requires a card up front (recommend: no card → higher trial start
  rate; or card-required → higher trial→paid; a business-plan decision).
- How prominent the first-run Cloud mention should be (A/B later).

## Server choice (self-hosting is first-class)

CashFlux is open source, so the sync/proxy server is a binary anyone can run. The **Server** is a
first-class, top-of-section choice in **Settings → Cloud** — before sign-in — with two options:

- **CashFlux Cloud (hosted)** — the managed, paid service (default). Subscription, OAuth sign-in,
  zero ops. Everything in the rest of this doc applies.
- **Self-hosted server** — point the app at *your own* server URL. No subscription, no paywall, no
  billing surfaces; you run the binary, you own the data end to end. This is the free escape valve and
  the trust anchor for the local-first promise.

### What changes by server choice
- **Server URL field** (self-hosted): the app stores a base URL and uses it for the gRPC bridge (`wss`)
  and the HTTP OAuth/blob endpoints. A **Test connection** button verifies reachability + version
  compatibility before saving.
- **No billing for self-host:** the Pricing screen, trial banner, "Manage subscription", and storage
  fair-use cap are all hidden when a custom server is selected. Entitlement is "always on" — the
  operator decides limits.
- **Auth is simpler for self-host:** OAuth requires the operator to register Google/GitHub apps, which
  is friction for a solo self-hoster. So self-host supports a lighter **single-user / access-token**
  mode (a token printed by the server on first run, pasted into Settings) *in addition to* optional
  OAuth. Hosted Cloud uses OAuth. The Settings UI shows the auth method the chosen server advertises.
- **AI key:** same per-user encrypted BYO model, just stored on *your* server.
- **Switching servers** is explicit and safe: changing the server URL signs you out of the old one and
  re-points sync; local data is never touched. A clear note: "Your local data stays on this device;
  switching only changes where it syncs."

### UX surfaces affected
- **Settings → Cloud** leads with a **segmented "Server: Cloud / Self-hosted"** control; the rest of
  the section (sign-in, sync, AI key, devices) renders beneath it, adapted to the choice.
- **Sync status chip** is identical for both; the tooltip names the server ("Synced to my-server.tld").
- **Onboarding** mentions both paths once: "Sync with CashFlux Cloud, or run your own server — your
  choice."
- **Trust/privacy copy** highlights self-host as the strongest privacy option.

### Open questions
- Default self-host auth: token-only (simplest) vs optional OAuth — surface what the server supports.
- Version/compat handshake: how the client warns on server too-old/too-new (reuse a schema/version ping).
- Whether to ship a one-command self-host quickstart (Docker) linked from Settings.
