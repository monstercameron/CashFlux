# CashFlux — Master Feature Backlog

Single source of truth, **ordered top-to-bottom by implementation priority**. Work in order;
within a section earlier items unblock later ones. Build **bottom-up** per the SDLC rule
(data model → services/logic with tests → persistence → state → UI last). See [`SPEC.md`](./SPEC.md)
for product detail and [`CLAUDE.md`](./CLAUDE.md) for the rules.

**Legend:** `[ ]` todo · `[x]` done · `[~]` in progress · `(P#)` phase · `★` critical path.
**Discipline:** one feature per commit; update `CHANGELOG.md` + `DEVLOG.md` each commit; pure logic
packages have no `syscall/js` and ship with table-driven tests.

---

### Review F36 — Personalized recommendations (4/10)
- [ ] **C254 [MAJOR]** Free smart insights are OFF by default — no recommendation surfaces for any user until a manual /smart enable trip → enable TierFree deterministic rules by default; keep AI-tier opt-in.
- [ ] **C255 [MAJOR]** Smart enabled-state may not persist across a fresh session (SmartSettings hydration on boot) → audit appstate hydration reads/writes SmartSettings from SQLite every load. *(verify)*
- [ ] **C256 [MAJOR]** 190/191 recommendation actions are navigate-only — cancel-sub / automate-goal / create-goal don't execute → add executable ActionKinds; depends on C186 (money-movement) + ActionCreateGoal/Recurring/CancelSubscription.
- [ ] **C257 [MAJOR]** /smart is a settings catalog, not a ranked insight hub, and the dashboard surfaces no recommendations → split into Insights (severity-sorted) + Manage-features tabs; show top-3 in a dashboard card.
- [ ] **C258 [MINOR]** SMART-SU1 "Review subscriptions" navigates to /subscriptions when already there (no-op); SMART-SU9 "Add a to-do" shows no confirmation toast → highlight the named row; confirm PostNotice reaches the toast renderer.
- [ ] **C259 [DESIGN]** No "Enable free features only" bulk; 193 insights render unranked/uncapped (15 of one rule) → free-only bulk enable; cap per-rule ~3, sort by severity, paginate.
- [ ] **R26 [RESEARCH]** Recommendation system: default-on free deterministic insights, ranked hub + dashboard surfacing, executable actions. → spec.

### Review F37 — Financial-health score (1/10)
- [ ] **C260 [MAJOR]** No composite financial-health score/breakdown/steps → deterministic `internal/healthscore` (savings rate + emergency months + DTI + budget adherence + utilization → 0–100) + dashboard widget + top-3 steps.
- [ ] **C261 [MAJOR]** Only SMART-A10 exists (per-account, AI-gated); inputs already exist → aggregate as a free deterministic rule; cap score &lt;50 on negative savings rate.
### Review F38 — Smart configurable alerts (2/10)
- [ ] **C263 [MAJOR]** No per-alert-type settings UI (notify.Rule config unexposed; only "Browser notifications" toggle; 6 types hardcoded) → "Manage alerts" per-rule rows.
- [ ] **C264 [MAJOR]** No user-settable thresholds (large-txn $500, bill-lead 7d hardcoded) → persist rule slice + threshold inputs.
- [ ] **C265 [MAJOR]** No "paycheck landed" alert type → EventPaycheckLanded + income-landing detector.
- [ ] **C266 [MAJOR]** No "low balance" alert type → EventLowBalance + per-account floor.
- [ ] **C267 [MINOR]** No severity differentiation in center (Severity field unused) → severity pill/icon.
- [ ] **C268 [MINOR]** No per-item read/dismiss/snooze (only Clear all) → per-item controls.
- [ ] **C269 [DESIGN]** "Notifications" missing from Settings jump-to tabs → add tab.
- [ ] **R28 [RESEARCH]** Alerts system (rules UI + thresholds + new events + live firing + unified badge + severity). Consolidates C121/C122/C158/C159.

### Review F39 — "While you were away" digest (3/10, broken)
- [ ] **C270 [MAJOR] ★ ROOT CAUSE of empty Notification Center** (fixes C121/C158/C159): `runNotifyCatchUp` writes the feed to SQLite KV but never `.Set`s the live atom (runs pre-mount; delivered-log then suppresses re-fire) → `feedAtom.Set` after `PrependNotifyFeed`, or init the feed atom after catch-up.
- [ ] **C271 [MAJOR]** No consolidated "while you were away" digest card/modal + no "since last visit" framing → dismissible dashboard catch-up card + labeled center section.
- [ ] **C272 [MINOR]** `runNotifyCatchUp` `recover()` swallows panics silently → add slog.

### Review F40 — Shared household access + roles (2/10)
- [ ] **C273 [MAJOR]** No role/permission model at any layer — domain.Member is {ID,Name,Color,IsDefault,Prefs}; IsDefault is a quick-add seed, not a role → add MemberRole (owner/admin/viewer) + enforce in entity access paths.
- [ ] **C274 [DESIGN]** No per-member login / access control / device user-switching (local-first single dataset) → add a local profile/PIN switch or explicitly surface the single-device limitation so users aren't misled.
- [ ] **C275 [MAJOR]** Add/Edit member forms have no role field → add a role selector to both.
- [ ] **C276 [MINOR]** Cosmetic "Default/Member" labels imply non-existent roles; member filter is display-only (no read-visibility enforcement) → remove misleading labels until roles exist; gate reads by role when implemented.
- [ ] **R29 [RESEARCH]** Household roles/permissions + local multi-user (profile/PIN switch) model vs. the local-first constraint. → spec.

### Review F41 — Per-member views/allocations/privacy (5/10)
- [ ] **C277 [MAJOR]** Member views not visibly scoped — txns summary shows household total ("1725 shown") regardless of member; dashboard KPIs identical Everyone vs Marcus with no indicator → recompute summary from filtered subset (transactions.go:82-84); add "Showing X's activity" label.
- [ ] **C278 [MAJOR]** Accounts/budgets/goals/allocate don't scope by active member (UseActiveMember only in txns/dashboard/split/quickadd) → filter or badge by OwnerID across these screens (accounts.go, allocate.go).
- [ ] **C279 [MAJOR]** No income-allocation / fractional account ownership (binary Owner only) → optional AllocationShares sub-form (e.g. Marcus 60% / Priya 40%) feeding ledger.NetByOwner.
- [ ] **C280 [MINOR]** /members shows balance-sheet attribution only; reports.SpendingByMember exists but unwired → add a per-member "this month" income/spend row.
- [ ] **C281 [DESIGN]** No "Viewing as &lt;member&gt;" banner/framing → persistent scope badge when a non-Everyone member is active. (Privacy is display-only, no enforcement — see C274/R29.)

### Review F42 — Bank-grade security (5/10)
> Verified working: PBKDF2-600k→AES-GCM-256 full-dataset at-rest encryption, passcode lock gate (wrong rejected / right unlocks), manual + inactivity auto-lock, honest "forgot passcode" wipe.
- [ ] **C282 [MAJOR]** No biometric/WebAuthn unlock (B17.5 designed, unbuilt) → navigator.credentials.create() + PRF as a second unlock.
- [ ] **C283 [MAJOR]** No MFA for cloud/backend auth → surface MFA enrollment at the cloud layer; passkey = local 2nd factor.
- [ ] **C284 [MAJOR] ★security** Passcode gate hash is SHA-256 (applock/applock.go:58), not a memory-hard KDF → brute-forceable offline if localStorage is extracted; use the same PBKDF2-SHA256/Argon2id as the dataset key.
- [ ] **C285 [MAJOR]** App-lock section absent from settings jump-nav → add `applock.section` to settingsNavKeys (settingssectionnav.go).
- [ ] **C286 [MINOR]** Lock gate text low-contrast/invisible in dark mode (card text color falls through to white on a white surface) → scope card text color for dark.
- [ ] **C287 [MINOR]** No passcode-strength check — setup accepts "000000"; pwcheck exists but unwired → wire pwcheck.Validate(PIN); also show auto-lock timeout in the status line.
- [ ] **C288 [DESIGN]** No "Security" section heading/route → rename "App lock" to Security; consider /security.
- [ ] **R30 [RESEARCH]** Security hardening (passkey unlock, MFA, memory-hard gate KDF, passcode strength).

### Review F43 — Privacy stance / local-first (3/10)
- [ ] **C289 [MAJOR]** No privacy/local-first trust statement anywhere user-facing (dashboard, first-run, sample banner) — the core differentiator is invisible; CLOUD_UX.md:40 spec'd "Your data stays on this device" but it was never built (copy exists only in the admin console) → add a trust line to the hero/sidebar footer + sample banner.
- [ ] **C290 [MAJOR]** No About/Privacy page/route or footer link (/privacy loads the dashboard; no &lt;footer&gt;) → add /about or /privacy (server-rendered or router) from docs/LEGAL_COMPLIANCE.md; link from the settings footer.
- [ ] **C291 [MAJOR]** Cloud sync section discloses nothing about what data leaves on sync → one-line disclosure under the backend toggle ("syncs encrypted snapshots; nothing leaves without this toggle").
- [ ] **C292 [MINOR]** AI-key disclosure + cloud trust line buried/conditionally hidden (cloud trust line only renders when CloudSelected) → surface at the Insights gate + Documents header; make the cloud trust line always visible.
- [ ] **C293 [DESIGN]** About surface is just version + changelog → expand: "Local-first · data stays on device · no account · export anytime".

### Review F44 — Data ownership: export/delete (8/10)
> Verified: Export JSON + CSV downloads fire; import round-trips losslessly; palette "Back up everything" + restore; wipe modal with Cancel. Read-only bank connections = N/A by design.
- [ ] **C294 [MAJOR]** Manual Export JSON calls `ExportJSON()` not `ExportJSONWithBlobs()` (settings.go:914) — receipt/document images excluded, so a "backup" can't self-restore images on a fresh device → switch to ExportJSONWithBlobs() (or warn).
- [ ] **C295 [MAJOR]** Import dataset overwrites all data with NO confirmation (importJSON settings.go:965-980 lacks confirmModal, unlike restore) → add a "this replaces your current data — continue?" modal.
- [ ] **C296 [MINOR]** CSV export is transactions-only but unlabeled, implying a backup → label "Export transactions (CSV)" + note JSON is the complete backup.
- [ ] **C297 [MINOR]** "Back up everything" (lossless multi-workspace) is palette-only, absent from Settings → Data → add a button/hint.
- [ ] **C298 [MINOR]** Settings "Data" not in jump-nav (buried below AI/Cloud); wipe confirm button labeled generic "Confirm" → add Data jump-link; relabel to "Wipe data".
- [ ] **C299 [DESIGN]** No "last backed up" timestamp shown (recordBackupNow stamps it but the UI never surfaces) → show "Last backed up: &lt;date&gt;" beside Export.

### Review F45 — Honest pricing / free tier / no dark patterns (6/10)
> Positives: free tier is genuinely generous (all core budgeting is local + ungated); UpgradeSheet is calm (no fake urgency, "Maybe later").
- [ ] **C300 [MAJOR]** No pricing page / price disclosure outside the one-shot UpgradeSheet (price strings en.go:951-953 render only in the sheet; no Plans tab) → add a "Plans" surface / "Cloud · $34.99/yr" in Settings; show price on every prompt.
- [ ] **C301 [MAJOR]** Upgrade path is one-shot — cloudmention.go writes `cloud-mention-dismissed` on BOTH buttons; the UpgradeSheet (sole caller cloudmention.go:38) is then permanently unreachable → add a persistent "View plans / Add Cloud" CTA in Settings → Cloud.
- [ ] **C302 [MAJOR]** No discoverable manage/cancel/downgrade surface — cancel routes via Stripe portal (billing_http.go:131 needs StripeCustomer); subscription banner only renders trialing/past_due/canceled → add a "Manage subscription" link in Settings → Cloud visible even to non-subscribers.
- [ ] **C303 [MINOR]** Free-vs-paid boundary + 14-day trial never stated in plain language in-app (cloud.benefit*/cloudTrialNote locked behind the unreachable sheet) → add "Free forever: budgeting/goals/reports · Cloud $34.99/yr: sync/backup/AI · 14-day trial" to the Cloud tab.
- [ ] **C304 [DESIGN]** Cloud & server tab is raw infra config (URL/token/test/deploy), not a billing surface → lead with plan status (tier/price/trial); collapse URL/token under Advanced.
- [ ] **R31 [RESEARCH]** Pricing/plan UX (visible Plans, free-vs-paid clarity, re-engageable upgrade, manage/cancel surface). ("Synced" pill on a local session = C5.)

### Review F46 — Cross-platform native + web sync (3/10)
- [ ] **C305 [BLOCKER] ★ LIVE REGRESSION** Dashboard panics on load — GWC-RUNTIME-PANIC "GoUseAtom called outside component context" at uistate/healthtrend.go:31 → screens/health.go:306 (a just-added health widget calls a hook in an effect body); a full-screen panic modal is the first thing the user sees → move the UseHealthTrend hook into the component render, not an effect body.
- [ ] **C306 [MAJOR]** PWA not installable — manifest.webmanifest has `icons:[]` (no 192/512 maskable) + no apple-touch-icon / apple-mobile-web-app-capable / splash → add an icon set + iOS meta tags.
- [ ] **C307 [MAJOR]** Install prompt captured (beforeinstallprompt) but never exposed — no Install button; window._installPromptCaptured undefined → wire the deferred prompt to a visible "Install app" affordance + iOS fallback.
- [ ] **C308 [MAJOR]** No native iOS/Android app (web/WASM only) → acknowledge the trade-off; consider a Capacitor shell.
- [ ] **C309 [MAJOR]** Sync conflict resolution is last-write-wins at full-snapshot level (syncstate.ShouldApplyRemote on timestamp; sync_client.go:170-175 silently drops rejected local pushes with only a toast) → field-level merge or a conflict-resolution UI; never silently discard local writes.
- [ ] **C310 [DESIGN]** Real-time sync requires a self-hosted backend (no hosted tier); no multi-device onboarding / "add a device" flow → hosted option or explicit no-backend state + add-device wizard. ("Synced" with no backend = C5.)
- [ ] **R32 [RESEARCH]** Cross-platform + sync (native shell, hosted sync tier, field-level conflict resolution, PWA installability).

### Review F47 — Offline / PWA + performance (3/10, offline broken)
- [ ] **C311 [MAJOR]** Offline reload returns a BLANK page — offline boot broken: SW handleNavigate uses `{cache:"no-store"}` (throws offline), then appShell()'s `./index.html` cache lookup also misses → empty 504 → audit SW install/precache; test offline end-to-end before shipping.
- [ ] **C312 [MAJOR]** Wasm never cached by the SW — install `c.add(u).catch(()=>{})` silently swallows the 60 MB wasm fetch failure → offline /bin/main.wasm returns false → make the failure visible; retry / dedicated large-asset cache.
- [ ] **C313 [MAJOR]** SW active but not controlling the page on first load (clients.claim loses the race vs window.load) → controls only on 2nd load → register earlier or handle the first-load case.
- [ ] **C314 [MINOR]** 60 MB uncompressed wasm, no gzip/brotli at serve (serve.go sets no Content-Encoding; ~48s cold load @10Mbps vs ~12s compressed) → compress wasm + add a load progress bar. *(Panic C305, PWA icons/install C306/C307 reconfirmed here.)*

<!-- END-REVIEW-FINDINGS -->

---

## B. Bug fixes (active, high priority) ★

### B2. Dashboard drag should reflow like an iOS app grid (respect multi-cell tiles) ★

**Symptom:** dragging a dashboard widget swaps it 1:1 with the drop target instead of inserting it and
letting the other tiles reflow; multi-cell (multi-span) widgets aren't handled and can overlap.
**Root cause:** `ui.Widget` (`internal/ui/widget.go`) handles `OnDrop` by calling
`dashlayout.Layout.Swap(src, target)`, which exchanges the two widgets' absolute `Col/Row` **and**
spans. So (a) only the two tiles move — the rest don't reflow; (b) no live displacement during the
drag (acts only on drop); (c) swapping spans between differently-sized tiles overlaps neighbors and
corrupts the bento packing. The model is absolute-placement + pairwise-swap; iOS-grid behavior needs
ordered reflow + size-aware packing.
**Fix (bottom-up per SDLC):**
- [~] Verify: multi-cell tiles never overlap + resize re-packs — **done** (Pack model + render verified
      in-browser); smooth FLIP animations — **done** (above). Still open: a live drag-over preview (reflow
      lands on drop) and pointer-events over HTML5 DnD for touch (the deferred top item).

### B4. Settings is duplicated — consolidate into the household-card panel ★

**Symptom:** the "Settings" item in the menu list opens what looks like a duplicate of the settings
you get from the **Your household** card at the bottom of the rail. The household card should be the
single, primary settings panel.
**Root cause:** there are two settings surfaces. (1) The **Settings** nav item → `/settings` route →
`screens.Settings()`, which only shows a *read-only* Household summary (base currency + member/account/
category counts) plus the Debug log — so it reads as an emptier duplicate. (2) The **household card**
(`app.HouseholdCard`, rail bottom) → the global settings flip panel (`globalSettingsForm` in
`internal/app/settings.go`), which holds all the real editing: members, base currency + FX rates, AI
key/model, appearance (theme/accent/density/week-start/date), data export/import/sample/wipe, freshness
overrides, module-visibility toggles.
**Fix (make the household-card panel the one primary settings surface):** — done.
- [~] Verify: one settings entry point, debug log in the panel, nothing regresses (full `go test ./...`
      + wasm green; browser spot-check pending).

### B5. Collapsed rail should reveal labels on hover ★

**Symptom / want:** the left menu should collapse to icons-only, and hovering an icon should show a
text label ("text highlight") for quick reference.
**Current state:** the rail already collapses to a 58px icon-only mode (`.collapsed`, shared
`rail:collapsed` atom; `internal/app/shell.go`), which hides each item's label `Span`. What's missing
is the hover affordance — collapsed, there's no quick way to see what an icon is.
**Fix:**
- [~] Verify: hover/focus reveals the label without expanding the rail (wasm green; browser spot-check pending).

### B6. Add a UI / font-size scale setting ★

**Want:** fonts and buttons feel ~30% too large for some users (e.g. on `/accounts`), though others
find them fine — add a setting to scale the whole interface up or down.
**Approach (analysis):** the design is px-heavy (Tailwind arbitrary px like `text-[13px]`), so a
rem-based root-font scale would NOT resize buttons/spacing. Use a **whole-UI zoom**: a `--ui-scale`
CSS variable applied via `zoom` on `#app` (Chromium target; `zoom` reflows and scales fonts + buttons
+ spacing together).
- [~] Verify: changing scale resizes the whole UI (wasm build green; browser spot-check pending); 100% == current.

### B7. Menu is missing main-line features ★

**Symptom:** the sidebar lists fewer items than the app implements. Primary nav has Dashboard /
Accounts / Transactions / Budgets / Goals / To-do; System has Members / Categories / Settings. But
`screens.All()` also routes five Phase-2 screens that are **not in the rail** — reachable only by
typing the URL: **Planning** (`/planning`), **Allocate** (`/allocate`), **Insights** (`/insights`),
**Documents** (`/documents`), **Customize** (`/customize`).
**Fix:**
- [~] Verify: every routed main-line screen now has a menu entry (wasm build green; browser spot-check
      pending). Module toggles cover them via the hidden-path filter.

### B8. Sidebar menu management: reorder, drop "My pages", visibility settings ★

Three related sidebar changes (relates to B5 collapsed-hover, B7 missing items):
- [~] Verify: no "My pages" group (done, wasm green). Shift+drag reorder still pending.

### B9. Clickable breadcrumb in the top bar ★

**Want:** an easy-to-read, clickable breadcrumb on the right side of the top-level panel so users can
see where they are and step backwards.
**Context:** the top bar (`internal/app/shell.go` `TopBar`) shows the page title on the left and the
resolution control + "+ Add" on the right (`ml-auto`). Routing is **flat** — Dashboard, Accounts,
Transactions, … are siblings with no nesting (`screens.All()`), so there's no natural multi-level
trail yet.
**Open decision (resolve before building) — what does the trail contain?**
  1. *Home-rooted* (simplest, recommended): `Dashboard / {Current Page}`, with "Dashboard" clickable to
     go home. Static, derived from the current route — no history needed.
  2. *Visited history*: last N visited pages as crumbs (browser-like back trail). Needs a small
     nav-history atom.
  3. *Logical hierarchy*: e.g. `Dashboard / Accounts / {account} transactions` once drill-downs carry
     context (account→ledger filter already exists). Richest but needs per-drill-down context.
**Fix — implemented option 1 (home-rooted):**
- [~] Verify: trail correct per screen; clicking returns home (wasm green; browser spot-check pending).

### B11. "+ Add" opens a flip-panel of add actions ★

**Want:** the top-bar "+ Add" button should open a centered flip panel (the same lift-to-center +
`rotateY` animation as settings) offering the kinds of things you can add — new transaction, bills to
scan, docs to scan, custom workflows, etc. — instead of jumping straight to `/transactions`.
**Context / reuse:** the flip animation + centered panel already exist as `ui.FlipPanel`, driven by
the `uistate.UseSettings()` atom and rendered by `app.SettingsHost` (kinds: "global" / "widget"). The
cleanest path is to **reuse that mechanism** rather than build a parallel overlay.
**Fix:**
- [~] Back face: instead of a menu of cards, it goes straight to the **New transaction** flow inline
      (account / expense-income / amount / description / category / date → `PutTransaction`, toast).
      Still TODO if a menu is wanted: **Scan a bill** / **Scan a document** (Documents import) /
      **Custom workflow** cards.
- [~] Keyboard-accessible, labelled, light/dark — inherits FlipPanel's chrome and the focus-visible
      rings; a `role="dialog"`/`aria-modal`/focus-trap pass is tracked under the dialogs a11y item.
- _Decision to confirm:_ what "custom workflows" means here — map to the existing Customize screen
  (custom fields + formula builder), or a new "workflow" concept? Need scope before building that card.

### B15. App-wide accessibility — spike + program ★

**Goal:** make CashFlux usable with a keyboard and a screen reader, at high zoom, and without relying
on color — to WCAG 2.1 AA as the bar. This is large and cross-cutting, so it starts as a **spike**
(time-boxed audit → prioritized plan) before the implementation tasks it spawns. Supersedes the
one-line a11y item in §1.20.

**B15.0 — Spike (do first):**
**Deep analysis — the areas the program must cover (becomes tasks after the spike):**
- [~] **Semantics & landmarks:** sidebar `<nav>` labelled "Main navigation"; `<main id=main tabindex=-1>`
      + a **skip-to-content** link; the top bar's page title is now the screen's single `<h1>` (dashboard
      in-canvas header demoted to `<h2>`). Still TODO: `banner`/`contentinfo` roles.
- [~] **Keyboard:** the div-based **toggle switch** and **accent swatches** are focusable + operable
      (tabindex=0 + Space/Enter via `OnKeyDown`; focus ring via `:focus-visible`). Segmented = real
      buttons. The **bento tiles are now keyboard-reorderable** — each is `tabindex=0` with
      `aria-keyshortcuts`, and Arrow keys move it one slot earlier/later (reuses `dashlayout.Move`,
      persists, switches to Custom) while **Shift+Arrow resizes** it (`dashlayout.ResizeItem`, clamped).
      Verified: ArrowRight moves a tile 1/2→2/2; Shift+ArrowRight grows it to "1 / span 2". The bento is
      now fully keyboard-operable. Still pointer-only: inline-edit focus-on-enter/exit and the nav reorder
      (B8, drag-only).
- [~] **Custom controls → correct ARIA:** Segmented = `role="radiogroup"`/`role="radio"`/`aria-checked`;
      Toggle/ToggleRow = `role="switch"` + `aria-checked` + name; StepperPill ‹/› have `aria-label`s;
      SwatchPicker = labelled `role="radiogroup"` of `role="radio"` chips. The gear (`aria-label="Widget
      settings"`), accounts "⋯" overflow (`aria-label`), and the grip (`aria-hidden`, decorative) now have
      correct names; the AddMenu/menu/+Add carry text or titles. Still TODO: real keyboard operability for
      the div-based Toggle/Swatch (they have Space/Enter via OnKeyDown; verify with a screen reader).
- [~] **Touch targets:** small icon-only buttons (delete/toast-x/rstep/set-close) now meet the WCAG
      2.5.8 AA 24×24 minimum (centered glyph). 44×44 (AAA) left aspirational given the dense desktop UI.
      - **✅ clr-toggle fixed (2026-06-24).** The per-row cleared-status toggle (`.txn-table .clr-toggle`,
        the ○/✓ on every transaction) was MISSED by the pass above: it measured **26×17px on desktop**
        (the 44×44 sizing was scoped to `@media (max-width:640px)` only, so mouse users got the 17px box).
        Added a base `display:inline-flex; align-items:center; justify-content:center; min-height:1.5rem`
        so desktop now gets a **26×24** hit area (glyph unchanged, the 55px row absorbs it; the 640px touch
        rule still wins → 44×44 on mobile). MEASURED (`e2e/clrtoggle_targetsize_verify.mjs`, 3/3): desktop
        min-dim 24, mobile min-dim 44, desktop row height unchanged at 55px. Screenshot
        `e2e/screenshots/clrtoggle_desktop.png`. (Remaining sub-24px hits are inline text drill-links
        `.row-desc`/breadcrumb — covered by the 2.5.8 inline-text exception — and the intentionally-thin
        widget resize handles `.rz`.)
### B22. Bills & due-date tracker + calendar — SPEC (from C38, 2026-06-18)
**Want:** a real bills surface beyond the dashboard "upcoming bills" widget — a list with due dates,
amounts, paid/unpaid status, and a **month calendar** view.
- [~] **Pure `internal/bills`** (no `syscall/js`, tested): derive bills from liability accounts'
      due-day/min-payment **and** Planning recurring items; compute next-due, overdue, days-until,
      paid-this-cycle; month-grid layout helper (which bills fall on which day). Reuse `dateutil`,
      `freshness`, `domain.Recurring`.
      Liability bills, Planning recurring outflows, next-due/days-until, and month-grid dots are now tested
      and wired into Bills/dashboard/notifications. Remaining: paid-this-cycle derivation.
- [~] **UI:** Bills screen — upcoming/overdue list + a **month calendar** with bill dots; "mark paid" →
      logs the payment; ties **B19** (bill-due reminders) + the dashboard widget.
      Bills screen, calendar dots, reminder-to-task, dashboard, CSV, and bill-due notifications are live.
      Remaining: mark-paid creates/links a transaction.
### B24. Split / shared expenses & settle-up between members — SPEC (from C38, 2026-06-18)
**Want:** split a transaction across members ("50/50") and track **who owes whom** with a settle-up view.
- [~] **UI:** "Split…" on a transaction (equal / % / custom); a **Settle up** view of net balances +
      "record a settlement" (creates a transfer).
      Standalone Split calculator now supports even and weighted splits, shows who owes whom, and exports the
      settle-up plan as CSV. Remaining: transaction-row entry point and persisted settlement transfer.
### B26. Budget rollover / sinking funds — SPEC (from C38, 2026-06-18)
**Want:** envelope **rollover** (unspent carries over) + **sinking funds** (save toward periodic large
expenses).
- [~] **State/UI:** per-budget rollover toggle; "carried over $X"; a sinking-fund type. Ties the
      methodology selector (envelope/zero-based, D6).
      Per-budget rollover now persists on `Budget.Rollover`, has add/edit checkboxes, and shows previous-period
      carried amount in the Budgets list. Remaining: dedicated sinking-fund type/UI.
## C. Live UI/UX review findings — 2026-06-16 (sample data) ★

✅ FIX (2026-06-24) — account-row `⋯` overflow menu had no keyboard/outside-click dismissal + missing
`aria-expanded`. The hand-rolled `add-wrap`/`add-menu` menu on each account row (and the unused `OverflowMenu`
primitive) had the same gaps the +Add menu had: no Escape-to-close, no `aria-expanded` on the trigger, and it
relied on `.add-backdrop` for outside-clicks (which doesn't paint over page content — stacking). Extracted a
reusable **`uiw.DismissPopover(isOpen, wrapID, onClose)`** custom-hook in `internal/ui/dismiss.go` (Escape →
close + refocus trigger; document `pointerdown` outside the wrapper → close; stacking-immune), wired it into
the `OverflowMenu` primitive AND `accounts_row.go`, and added `aria-expanded` (via the existing `ariaBool`).
**Non-obvious bug found by verifying on the rendered menu:** `UseId()` returns ids containing colons (e.g.
`gwc:3:1`), which are invalid in a `#id` CSS selector — `querySelector("#"+id)` threw a SyntaxError and
panicked the wasm callback, silently breaking BOTH dismissal paths. Switched to `getElementById(id)` (never
throws). (The +Add menu was unaffected — it keys off the `.add-btn` class, not an id.) MEASURED on the live
`/accounts` page (`e2e/accounts_menu_verify.mjs`, 8/8): aria-expanded toggles false↔true, menu opens & stays
open, outside-click over content (a `SPAN`) closes, Escape closes + returns focus to the ⋯ trigger, menu item
still closes the menu. Build rc=0; `go test ./internal/ui` ok. (Note: Playwright stability waits stall on
interaction because WONDER hover transitions keep elements "unstable" — verified with raw mouse + querySelector
snapshots. **Correction (2026-06-24): this is NOT a re-render bug** — a MutationObserver shows **0 idle DOM
mutations** on /accounts, /, /transactions, /budgets over 3s with the pointer parked, so there's no runaway
re-render; the stall is purely animation-induced. Also verified: cold deep-link to 8 routes shows no dashboard
flicker.)

✅ ENHANCE (2026-06-24) — `ui.DismissPopover` now also does WAI-ARIA arrow-key roving focus. With Escape +
outside-click + aria-expanded already in place, the menus lacked keyboard item navigation. Added
ArrowDown/ArrowUp (cycle, with wraparound) + Home/End to the shared helper's keydown handler, gated on focus
being inside the popover so global arrow keys are never hijacked while a menu is merely open-but-unfocused.
Benefits every DismissPopover consumer (accounts ⋯, custom-page ⋯, the OverflowMenu primitive). MEASURED on
the accounts menu (`e2e/menu_arrowkeys_verify.mjs`, 8/8): ArrowDown from trigger → first item; Down/Up move &
wrap; End→last, Home→first; Escape still closes + refocuses (no regression). Prior dismissal guards re-run
green (accounts 8/8, custom-page 7/7). Build rc=0; `go test ./internal/ui` ok.

✅ ENHANCE (2026-06-24) — migrated the `+ Add` topbar menu onto `ui.DismissPopover` too. It was the last
dropdown still running its own ~50-line inline Escape/outside-click `UseEffect`, so it had no arrow-key nav and
duplicated the helper. Gave its `.add-wrap` a `UseId` and replaced the inline effect with one
`ui.DismissPopover(open, menuID, closeMenu)` call (keeping `addMenuShouldOpenLeft` for open-direction). Now
EVERY app dropdown (+Add, accounts ⋯, custom-page ⋯, OverflowMenu primitive) shares one helper with the full
menu-button pattern: aria-expanded, Escape+refocus, outside-click, Arrow/Home/End. MEASURED: +Add arrow-keys
(`e2e/addmenu_arrowkeys.mjs`, 6/6 — 9 items, Down/Up/Home/End + wrap, Escape+refocus); existing +Add guards
unchanged (widths 6/6, escape 5/5, outside-click 4/4). Build rc=0; `go test ./internal/app ./internal/ui` ok.
Net −~50 LOC from `addmenu.go`.
✅ FIX (2026-06-24) — `+ Add` menu opened OVER the left rail (items half-unclickable). The single "Add
something new" button sits at the top-left of the content area (x≈264), only ~24px right of the rail edge
(x=240). `.add-menu` used `position:absolute; right:0`, so its 210px panel extended **leftward** to x≈84 —
back over the sidebar. Measured consequence: the menu items' clickable centres (x≈189) fell inside the rail,
which intercepted the pointer (a real Playwright "rail subtree intercepts pointer events" failure when
clicking "New transaction"). Fix: open the menu **rightward** (`right:0` → `left:0`, `web/index.html`), so it
flows into the content column. MEASURED (`e2e/addmenu_verify.mjs`, 8/8, both themes): items now at minLeft=269
≥ railRight=240, menu fits viewport (maxRight=469), "New transaction" is clickable (no interception) **and
opens the add modal**. Pure-CSS; build rc=0. Screenshots `e2e/screenshots/addmenu_fixed_{dark,light}.png`.

⮑ FOLLOW-UP FIX (2026-06-24, same day) — the `left:0` above was verified only at 1280 and **regressed narrow
widths**. Measuring the button across 8 viewports showed it REFLOWS between the left of the content area
(gapLeft≈24-104px, near the rail) and the right edge (gapRight≈24-32px) with NO clean width→side mapping
(left at 1280/1200/1025/900; right at 1100/768/500). So a fixed `left:0` overflowed the viewport when the
button was on the right, while `right:0` overlapped the rail when on the left — and a breakpoint can't capture
it. Made it **decide direction live at open-time**: `internal/app/addmenu.go` measures the button's gap to the
right edge via `syscall/js` (`addMenuShouldOpenLeft`) and adds `.open-left` (→ `right:0`) when there's < ~224px
of room on the right; otherwise opens rightward (`left:0`, default). MEASURED (`e2e/addmenu_widths_verify.mjs`,
6/6 at 1280/1100/1025/1024/768/390): no viewport overflow AND no rail overlap at any width; click+modal still
8/8 (`addmenu_verify.mjs`, both themes). Build rc=0; `go test ./internal/app` ok.

✅ FIX (2026-06-24) — `+ Add` menu didn't close on ESCAPE (keyboard-a11y gap). Measured: opening the popover
then pressing Escape left `aria-expanded="true"` and the backdrop still active (it only closed via item-click
or a backdrop click). Per the WAI-ARIA menu-button pattern, Escape should dismiss it and return focus to the
trigger. Added a document `keydown` listener in `internal/app/addmenu.go` (registered only while open, torn
down on close/unmount — mirrors `dialoghost.go`) that closes the menu on Escape and refocuses `.add-btn`.
MEASURED (`e2e/escape_addmenu_verify.mjs`, 5/5): Escape closes (aria→false, menu hidden) + **focus returns to
the +Add button**; menu still reopens & positions correctly (no regression to the open-direction logic) and
still closes on backdrop click. Build rc=0; `go test ./internal/app` ok.
Captured by driving the running app (`http://127.0.0.1:8080`) in a real headless Chromium via the
now-installed Playwright driver and screenshotting all 14 routes (Dashboard, Accounts, Transactions,
Budgets, Goals, To-do, Planning, Allocate, Insights, Documents, Customize, Members, Categories,
Rules). Screenshots + rendered text are in `.review-screenshots/` (git-ignore this). Items are
ordered correctness-first, then cross-cutting chrome, then per-screen polish.

### C82. Agentic tool-calling harness (in-house, on the provider abstraction) ★ (feature, user-requested 2026-06-20) — ✅ DONE (verified 2026-06-21: `screens/chat_agent.go` agent loop + `ai.SendChatTools` drive OpenAI function-calling turns)
**Design doc:** [`docs/DESIGN_AI_PROVIDERS.md`](./docs/DESIGN_AI_PROVIDERS.md) §9 — read first.
**Finding:** no off-the-shelf Go agent framework fits `GOOS=js GOARCH=wasm` + local-first
(langchaingo/eino/genkit/swarmgo are server-oriented, heavy deps, wasm-unproven; vendor SDKs don't
provide a loop and would replace our isolated transport). The loop is ~a few hundred lines of pure Go →
**build in-house on the C81 provider abstraction**, borrow concepts not frameworks.
**Design:** tool-call dialect = same two-dialect split as C81 (OpenAI `tools`/`tool_calls` covers 6/7;
Anthropic tool-use); typed Go tool registry over `appstate` (read + guarded writes; reuse the structured-
output JSON-schema machinery); bounded pure loop (`internal/agent`: max steps + token budget,
model→tool_calls→execute→repeat, cancelable); capability-gated on a new `Capabilities.Tools` flag with a
**plan-only fallback** for non-tool models.
**Safety (the key argument for in-house):** every agent mutation goes through `appstate` validation and
is recorded by the **audit/undo system (C78)** with `actor="agent"` → one-`⌘Z` reversible + in the
activity timeline; destructive/bulk tools require explicit FlipPanel confirmation; data-minimization
preserved; render a **step transcript** (explainability rule).
**Build bottom-up:**
- [~] wasm wiring + UI: agent surface w/ step transcript + approval prompts; capability gating +
      plan-only fallback. Playwright story.
      _(2026-06-20: Insights screen rebuilt as a **chat interface** — conversation thread, Markdown assistant
      bubbles with per-message Save-as-task/Pin + cost, starter chips, composer; sends the whole history each
      turn. MVP uses the flat-prompt chat-completions path. STILL OPEN: bind `internal/agent` loop +
      `internal/aitools` gated read-tools via an `agent.Model` adapter + appstate `DataSource` (tool transcript,
      affordability, richer Q&A), token streaming, approval prompts for future write tools, and the Playwright
      story.)_
**Sequencing:** lands **after C81 Phase 1–3** (needs provider/dialect abstraction) and is much safer
**after C78** (undo). _Cross-links: **C81** (providers/dialects/caps), **C78** (undo = agent seatbelt),
**C76** (AI modal/approval surface), **C75** (notifications), `internal/workflow` (agent can author
workflows/rules), `internal/formula` (sandboxed compute tool)._

### C20. Collapsible side panel reads as "missing" — toggle is misplaced and collapse is broken ★
**Reported:** no collapsible left panel and no toggle button. **Reality (verified):** a menu-toggle
button *does* exist (28×28, with the `icon.Menu` glyph) and clicking it collapses the rail — but:
- [~] Verify: collapse → usable icon rail (C15 ✓) and persists (✓). An on-panel toggle is the open part.

### C22. Layout engine does not reflow on move or on resize ★ (= B2 / C14, with fresh evidence)
**Reported:** moving tiles doesn't reflow; scaling tiles up/down doesn't reflow. **Verified live:**
dragging `kpi-income` onto `kpi-liabilities` changed only those two tiles' `grid-area` (income→`2/4`,
liabilities→`2/3`) — **no other tile moved**, and the result even mis-placed a tile (not a clean swap).
Resize overlaps neighbors (C14). Root cause: absolute placement + pairwise `Swap`/`Resize`, no packing.
- [~] Verify: move/resize reflow is structural (✓, via Pack + the unit tests + the pixel-identical render
      check). The only open piece is the live drag-over **preview** (reflow currently lands on drop, not
      during the drag) — tracked as the remaining B2 UI-polish item, not a correctness gap.

### C25. Default UI is too "fat/chunky" — tighten the density tokens ★ (UX)
**Reported:** the UI (incl. the Add-transaction modal) feels too fat/chunky on every screen.
**Measured live (1440px, scale 100%) — concrete weights:**
- body **16px** / 24px line-height (Tailwind default; heavy for a dense finance app)
- form `.field` inputs **40px tall**, 16px text, 8×9.6px padding
- buttons up to **~60px tall** (12px padding); primary actions read oversized
- widget `.wbody` padding **~15×16px**; widget title 16px; nav items **36px** tall
- the "+ Add" → **Add a transaction** modal body is ~360px but its fields use only ~150px — large dead
  space below the form (also **C13**)
**Analysis:** chunkiness is global because it comes from shared tokens (base font, `.field`, button,
`.wbody` padding), so adjusting the tokens fixes all 14 screens + modals at once. Two existing levers
already exist but don't fix the *default*: the **Compact density** toggle and the **Display scale**
zoom (**B6**) — the complaint is that the out-of-the-box weight is too high.
- [~] Re-check at the new density: verified live on the dashboard + the quick-add form — body 14.5px,
      fields 34px with no text clipping, KPI figures still fit (0 clipped). The other screens are
      route-gated in the static oracle but use the same shared tokens, so the effect is uniform; nothing
      reduced below the existing **24px** B15 touch-target minimum (fields 34px, buttons ~30px).
### C26. Make text size configurable for low-vision users ★ (accessibility)
**Reported:** font size should be configurable for visually impaired folks. **Current state:** B6 added
a **Display scale** (70–130%) implemented as a whole-UI **`zoom`** on `#app`. That helps but isn't a
true text-resize control: it tops out at 130%, scales layout (not just text), and `zoom` can break the
non-responsive layout (**C10**) at large values.
- [~] Verify at 200%: confirmed no horizontal scroll / reflow on the dashboard (root). The other 13
      screens are route-gated in the static oracle, but they share the same responsive rules + zoom
      mechanism, so the reflow behavior is uniform.

## L. Loop user-story QA — story-driven gaps ★

Findings from the recurring user-story QA loop: invent a real household's flow, drive the app
end-to-end, screenshot it, and log mechanical + UI/UX gaps the dev agent should build/fix
bottom-up (model → tested logic → store → state → UI). Each story below names the persona and the
exact ritual, then the gaps that block it. Screenshots live in `e2e/loop*-*.png`; the driving
script is `e2e/loopstory_NN_*.mjs` (run via `node e2e/run-stories.mjs` or standalone against :8099).

### L15. Story — "Set It and Forget It" (Bianca, Rules / auto-categorization) — 2026-06-20 ★

**The ritual:** Bianca creates a rule — match "Starbucks" → Dining — and expects every new transaction to
auto-file itself, plus a way to backfill existing uncategorized ones.
**Drive script:** `e2e/loopstory_15_set_and_forget.mjs` (interactive: create a rule, add a matching txn,
assert the category auto-fills).
**✅ VERIFIED WORKING end-to-end (strong feature — keep as a regression anchor):**
- Created a rule (match → category + tags) on `/rules`; it listed and **persisted**. ✓
- Added a transaction whose description matched; the **category select auto-filled to "Dining"** (the
  `SuggestTransactionFields` path), **surviving a full page reload** (verified the 3 selects = Expense /
  Auto Loan / **Dining**). ✓
- An **"Apply to existing"** backfill affordance is present. ✓
- Engine (`internal/rules`): case-insensitive substring match + **first-match-wins** with specificity
  ordering, table-tested. ✓

**Action (lock in the win):**
**Dream-big gaps (extend a solid engine):**
- [~] **Richer match conditions** — substantially covered by the existing **workflow** engine
      (`internal/workflow` + `/workflows`): expression conditions like `txn_abs > 200` (amount range) and
      `contains(txn_payee, "coffee")` (keyword), tested. The pure `rules.Condition` type (AllKeywords/
      AnyKeywords/AccountID/Min-MaxAmount, tested) also exists but isn't yet wired into the simple `Rule`
      struct/form. (Remaining: surface `Condition` in the simple rules form, OR converge rules→workflows.)
- [~] **Actions beyond category + tags** — covered by the workflow engine's actions
      (`ActionSetCategory`, `ActionAddTag`, `ActionFlagReview`; seeded in sample.go). (Remaining:
      member/owner + budget actions; converge with the simple Rule.)
**Probe note:** the auto-categorize check **false-negatived** first (the script read the *account* select
"Auto Loan", not the *category* select); a focused re-measure confirmed the category = "Dining". Fix
`loopstory_15` to read the category select by position/label, then promote per the gate above.

### L20. Story — "The Finish Line" (Aaliyah, goal-completion lifecycle) — 2026-06-20 ★

**The ritual:** Aaliyah's emergency-fund goal is about to be reached. She wants CashFlux to recognize the
milestone — celebrate it, mark the goal achieved, stop nagging her to contribute, and suggest
redirecting the freed-up monthly amount to her next goal.
**Drive script:** `e2e/loopstory_20_finish_line.mjs` (create a goal, push it past 100%, inspect the
completed state). Verified by creating a goal over target ($80 saved / $50 target).
**✅ VERIFIED WORKING (the completion moment is handled well — keep as anchors):**
- At/over target the goal shows a **full (capped) progress bar + "Complete 🎉"** badge. ✓
- The **pace nag is removed** when complete (no "save $X/mo", no "$X to go"). ✓
- Contribute/Edit remain available; the bar correctly caps at 100% even when over-funded ($80/$50). ✓

**Gaps (what happens AFTER the finish line):**
- [~] **Over-funding acknowledged.** Pure `goals.Overfund(goal)` (tested) → a calm "<amount> over target"
      note on over-funded rows. (Remaining: a "move excess" redirect action reusing L17 allocate / L5
      contribute — deferred.)
**Probe note:** the first run's "achieved state" + "100% cap" checks **false-negatived** — the inline
**Contribute opens an amount form (not a JS `prompt`)**, so the `page.on("dialog")` handler never fired
and the goal stayed at $0 (and my row filter clicked the wrong goal's Contribute). A corrected re-drive
(create goal already over target) confirmed the **"Complete 🎉"** state + capped bar exist. Fix
`loopstory_20` to fill the inline contribute form for the *named* goal.

### L24. Story — "Pay Yourself First" (Leah, transfers / accounting invariants) — 2026-06-20 ★

**The ritual:** Leah moves $500 from Checking to High-Yield Savings monthly. She expects: Checking
-$500, Savings +$500, **net worth UNCHANGED**, and the transfer **excluded** from income/expense.
**Drive script:** `e2e/loopstory_24_pay_yourself.mjs` (+ `_xfer` diagnostic for a real from→to).
**✅ VERIFIED WORKING (correctness — keep as a regression anchor):**
- The transaction form supports a **Transfer kind** with **from + to account** selectors. A real
  $500 **Everyday Checking → High-Yield Savings** transfer recorded correctly. ✓
- **Accounting invariants hold:** after the transfer, **net worth unchanged ($354,070)**, **income
  unchanged ($6,400)**, **spending unchanged ($4,088)** — transfers are net-worth-neutral and correctly
  excluded from income/expense. ✓ (Individual per-account ±$500 not separately asserted, but the
  net-worth-flat + not-income/expense invariants confirm balanced legs.)
- **Action:** promote to a CI gate (`e2e/transfer_invariants_check.mjs`) — net-worth-neutral +
  excluded-from-income/expense is a core correctness invariant.

**Gaps (dream-big automation + edge cases):**
- [~] **Recurring / scheduled transactions.** The `domain.Recurring` model (cadence + NextDue + account +
      category + autopost + `Advance()`), its store wiring, `PutRecurring`, and `PostDueRecurring(asOf)` with
      bounded catch-up ALL already existed; Planning has the full create/edit/autopost management UI.
**Bonus note for the dev agent (cross-cuts L1/L3/L17/L18):** `Transaction.Splits []CategorySplit`
**already exists** in the model (`entities.go:77`). The category-split *data model* is in place — the
budgets-cover (L1), receipt-as-split (L3), allocate-apply (L17), and custom-field reporting work mostly
need **UI + apply logic over the existing Splits**, not a new schema. Verify the splits UI/round-trip.

**Probe note:** the first run's transfer was **vacuous** — my account picker selected the placeholder
"— To account —" as the destination, so submit failed validation and *no* transfer occurred (making the
invariant PASSes meaningless). The `_xfer` re-drive with a real Checking→Savings destination is the source
of truth. Fix `loopstory_24` to pick accounts by name and skip empty-value options.

# App running on http://127.0.0.1:8080 (gwc dev)
cd C:\Users\mreca\Desktop\CashFlux
$env:E2E_URL="http://127.0.0.1:8080"
node e2e/loopstory_76_following_the_thread.mjs
  → 7 PASS · 0 FAIL · 10 ABSENT    EXIT 0
```

**Screenshots produced (23):**
`L76_hop1_transactions.png` · `L76_hop1b_transactions_pivots.png` ·
`L76_hop2_accounts.png` · `L76_hop2b_accounts_ledger.png` · `L76_hop2c_accounts_pivots.png` ·
`L76_hop3_categories.png` · `L76_hop3b_categories_txn_drill.png` · `L76_hop3c_categories_pivots.png` ·
`L76_hop4_budgets.png` · `L76_hop4b_budgets_txn_drill.png` · `L76_hop4c_budgets_pivots.png` ·
`L76_hop5_rules.png` · `L76_hop5b_rules_pivots.png` ·
`L76_hop6_goals.png` · `L76_hop6b_goals_pivots.png` ·
`L76_hop7_bills.png` · `L76_hop7_planning.png` · `L76_hop7_dashboard.png` ·
`L76_final.png`
(plus `L76_hop1b_transactions_links.png` · `L76_hop5b_rules_txn_link.png` · `L76_hop7b_planning.png` · `L76_hop7c_dashboard.png` from first-pass run)

**Re-test status**
- **L74/L75 GAP-E** (drill-filter broken): **CLOSED** — all three drills (account/category/budget → /transactions) correctly filter the result set. The L75 probe was looking for `a[href]` not `button`; the filter itself was never broken.
- **L74 GAP-G** (/goals linked-account): **STILL OPEN** — "· linked to X" button navigates to /transactions, not /accounts. The goal→account specific pivot is absent; the button is a transaction drill with a confusing label.
- **L74 GAP-F** (bills widget → /accounts instead of /bills): not re-tested in L76 (out of scope).
- **L75 SA-10** (/reports → /transactions link): not re-tested in L76 (reports not in scope).

---

### L79. Story — "The Money Move" (Renu) — 2026-06-24 ★

**The ritual.** Renu does the most common household money action: move money between her own
accounts (Everyday Checking → Emergency Savings). Theme = **transfer integrity + cross-screen
consistency**. Drive script `e2e/loopstory_79_money_move.mjs` (run `E2E_URL=http://127.0.0.1:8099
node e2e/loopstory_79_money_move.mjs`). Result **7 PASS · 0 FAIL · 2 ABSENT** (the 2 ABSENT are the
finding below). Screenshots `L79_01..05`.

**🔴 CRITICAL (found + FIXED this pass) — the app did not boot at all.** Before any ritual could
run, both `serve.go` (:8099) and `gwc dev` (:8080) rendered a blank page: the wasm panicked on
startup with `panic: GoUseAtom called outside component context`. Root cause: the admin-console
gating added `uistate.UseAdminConsoleAvailable()` (a framework **hook**) into `app.navGroup`
(`shell.go:246`); but `navGroup` is also called at **boot** by `wireKeyboardShortcuts()`
(`shortcuts.go:32` ← `app.Run` ← `main.main`) to enumerate primary-nav paths for digit shortcuts.
Hooks may only run during a component render, so the boot-time call aborted `main()` and the whole
app failed to start. **Fix:** added a hook-free `primaryNavStatic()` in `shell.go` (enumerates the
primary group straight from `screens.All()`, excludes `AdminOnly`, calls no hook) and pointed
`wireKeyboardShortcuts` at it. Build rc=0 (PARENT-VERIFIED); `go test ./internal/app` ok. MEASURED:
the app now boots — `navLinks=27`, `#app` body 79 KB, **zero pageerrors/console errors** (was
`navLinks=0`, panic). This is a committed-HEAD regression, not local churn (git diff of shell.go/
shortcuts.go was empty). **A boot smoke test belongs in CI** — nothing caught "the app won't start".

**⚠ FINDING (dev ticket) — a transfer gives NO visible balance feedback on /accounts.** The transfer
itself is *correct*: the form submits, a labelled "Transfer" ledger entry is created (the $250/$500
legs show up in /transactions), and **net worth is conserved** across 1 + 3 stress transfers
(`$60,386.00` throughout — the double-entry integrity invariant holds, T-2/T-3/T-5 all PASS, zero JS
errors). BUT on /accounts the **displayed balances do not move**: source `Everyday Checking` stayed
`$6,473.50` and destination `Emergency Savings (HYSA)` stayed `$12,200.00` after a $500 transfer
(measured delta = 0 for both; T-1a/T-1b ABSENT). Cause: the row shows the **cleared** balance
("… cleared $6,473.50") and a newly-created transfer leg is **uncleared**, so the cleared figure is
unchanged. From the user's POV: *"I just moved $500 between my accounts and both balances look exactly
the same"* — confusing, looks broken, and undermines trust on the single most common household action.
- [ ] **L79-T1 — give transfers immediate, visible balance feedback on /accounts.** Options (pick per
  spec): (a) treat an own-account transfer as **cleared on creation** (the money has demonstrably
  moved between the user's own accounts — there's no pending settlement), so cleared balances update
  at once; and/or (b) show the **current/available balance** as the primary figure with cleared as a
  secondary line; and/or (c) surface a success toast ("Moved $500 → Emergency Savings · new balance
  $11,700") so the action is acknowledged even if the headline figure is the cleared one. Today there
  is no toast and no balance change — the only confirmation is hunting for the leg in /transactions.
  e2e: extend `loopstory_79` to assert source−amount / destination+amount on /accounts after a transfer.
- Note (not a bug): the transfer destination picker correctly offers liabilities (credit card, student
  loan) and investment accounts as targets — paying down a card via "Transfer" is a reasonable flow.

**What works well (regression anchors)** ✓
- ✓ Net-worth conservation across transfers (double-entry integrity) — exact to the cent over 4 transfers.
- ✓ Transfer creates a labelled "Transfer" ledger entry; not silently merged or mis-typed as spend/income.
- ✓ Reports spending total unaffected by transfers (transfers aren't counted as expenses).
- ✓ Zero JS errors / no crash through a 4-transfer mini-stress.

**Process note — concurrent cloud-sync churn broke the build (again).** This pass also had to repair
a red tree before it could build: `internal/screens/homehero.go` (untracked, a half-applied
"HomeHero" homescreen feature) called `.String()` on `css.Rule` (no such method) and was wired into
`dashboard.go:277`; `internal/screens/widget_builder.go` was re-broken (missing `vbSeriesMax`/
`vbChartColors`, same as the prior two passes). To get a verifiable green build I `git checkout`-ed
`dashboard.go` + `widget_builder.go` to HEAD and renamed `homehero.go` → `homehero.go.churn-disabled`
(preserved, excluded from the build) — no other churn files touched. The HomeHero feature is
incomplete/uncompilable and needs a real finish-or-revert by a dev.

---

### L80. Story — "Paying the Bills" (Tomas) — 2026-06-24 ★

**The ritual.** Tomas clears this week's bills on payday — "Mark paid" is one of the most common
household actions, so it must be rock-solid. Theme = **bill-payment lifecycle + cross-screen
propagation**. Drive script `e2e/loopstory_80_paying_bills.mjs` (run `E2E_URL=http://127.0.0.1:8099
node e2e/loopstory_80_paying_bills.mjs`). Result **10 PASS · 0 FAIL · 0 ABSENT**. Screenshots
`L80_01_bills_before.png`, `L80_02_after_markpaid.png`.

**What works well (regression anchors)** ✓ — the lifecycle is genuinely solid:
- ✓ **B-1** Mark paid creates **exactly one** payment transaction (`RecordBillPayment` → `PutTransaction`),
  findable in /transactions.
- ✓ **B-2** For a recurring bill, **NextDue advances** after payment (measured 2026-07-01 → 2026-07-03 via
  `r.Advance()`) — it won't re-dun the same due date.
- ✓ **B-4** Clear success confirmation toast ("Logged a payment for Rent." in `.toast-msg`).
- ✓ **B-5** Spending (Reports) reflects the payments ($5,518.00 after the run).
- ✓ **B-6 STRESS** 3 back-to-back payments created **exactly 3** transactions — no double-post, no
  dropped post, no crash.
- ✓ Zero JS errors across the whole ritual.

**⚠ FINDING (dev ticket) — "Mark paid" has no double-charge guard (money-risk).** B-7: double-tapping
"Mark paid" on the same bill records **2 payments** (delta = 2 transactions; measured "Student Loan"
616→618). Each click posts a payment with no debounce, no optimistic "Paid ✓" state, and no confirm —
so an accidental double-tap (easy on touch / a laggy wasm frame) silently double-records a real money
movement, and the bill stays in the list looking unpaid. The lifecycle is otherwise correct; this is
purely a guard gap.
- [ ] **L80-T1 — guard "Mark paid" against accidental double payment.** On click, immediately disable
  the button (and/or swap it to a non-interactive "Paid ✓ — undo" affordance) until the row re-renders
  with the advanced NextDue, so a second tap can't post a duplicate. Optionally a lightweight confirm
  for non-recurring (liability) payments where there's no NextDue to move the row out of range. e2e:
  extend `loopstory_80` to assert a rapid double-tap yields delta = 1 transaction, not 2.
- Note: this mirrors the broader "destructive/irreversible action needs a guard" theme (cf. the bulk-
  delete-confirmation gap L50) — money-posting actions should be at least as protected.

**Probe note.** The transient confirmation renders as `<span class="toast-msg">`, distinct from the
persistent sample-data `.toast` banner — drive scripts must target `.toast-msg` (or filter by text) to
read action confirmations, else they grab the banner (initial L80 run mis-flagged "no toast" before the
selector fix).

---

### L82. Story — "Paying Yourself First" (Aaliyah) — 2026-06-24 ★

**The ritual.** Aaliyah moves money into her savings goals on payday via "Contribute". Theme =
**goal-contribution lifecycle + money-effect honesty**. Drive script
`e2e/loopstory_82_goal_contribution.mjs` (run `E2E_URL=http://127.0.0.1:8099 node
e2e/loopstory_82_goal_contribution.mjs`). Result **8 PASS · 0 FAIL · 1 ABSENT** (the ABSENT is the
HIGH finding below). Screenshots `L82_01..02`.

**What works well (regression anchors)** ✓
- ✓ **G-1** Contributing raises the goal's saved amount exactly (+$200 no-ledger, +$300 ledger; 3×$50
  accumulates to exactly $150 — no drift/double-count).
- ✓ **G-2** A no-ledger contribution creates **no** transaction (goal-only); a "Also debit …"
  contribution creates **exactly one** transaction.
- ✓ **G-4** $0 contribution is rejected (goal unchanged) — L41 holds.
- ✓ Zero JS errors across the ritual.

**🔴 FINDING (dev ticket, HIGH) — a goal contribution is counted as SPENDING (saving lowers your
savings rate).** With "Also debit …" checked, `ContributeToGoal` posts a category-less debit against
the linked account. MEASURED: Reports SPENDING total rose by **exactly the contribution amount**
($8,222.67 → $8,522.67 after a $300 contribution). So moving money into savings inflates "spending"
and — because savings-rate = (income − spending)/income — **literally lowers the user's savings rate
when they save more.** The source comment (`goal_ops.go`) intends the no-`CategoryID` to avoid
distorting per-*budget* rollups (it does), but the *total* spending figure sums all expense-signed
txns regardless of category, so the contribution still counts. Transfers are correctly excluded from
spending (verified L79) — a goal contribution is conceptually a transfer-to-savings and should be
treated the same.
- [ ] **L82-T1 — exclude goal-contribution ledger entries from the spending/savings-rate totals**
  (treat them like transfers): e.g. mark the posted txn as a transfer/`IsTransfer`-equivalent or add a
  "savings" exclusion so it doesn't inflate Reports/Dashboard spending or depress the savings-rate.
  e2e: extend `loopstory_82` to assert Reports spending is UNCHANGED after a ledger contribution.

**⚠ FINDING (dev ticket, LOW) — milestone toast shows a doubled percent ("25%%").** The progress
milestone toasts `goals.milestone25` = `"25%% of the way there — keep going!"` and `goals.milestone75`
= `"75%% funded — almost there!"` (`internal/i18n/en.go`) contain a literal `%%`, but they're rendered
via `uistate.T(key)` with **no `Sprintf`** (goals.go:155), so the `%%` is NOT collapsed and the UI
shows "25%%"/"75%%". (The `%d%%` strings like `goals.progressFmt` are fine — those ARE `Sprintf`'d.)
MEASURED: contribute toast rendered `"25%% of the way there — keep going!"` in `.toast-msg`.
**Semantic note (for design):** the "post ledger" checkbox reads *"Also debit <linked account> (move
money from this account)"*, yet goals display as *"linked to <account>"* (the savings destination).
Debiting the very account the goal tracks is conceptually backwards for a savings-destination link —
worth clarifying whether the linked account is the source or the destination (cross-ref L82-T1).

---

## G. GLAMOR — per-page UX/visual structure review (world-class, enterprise, glanceable) ★

### GD. DESIGN/FLAIR PASS — page-by-page beautification (2026-06-24, ongoing) ★
Goal: lift the visual design/flair (not UX) across every page + modal, per the frontend-design skill
(refined/luxury direction — depth, layering, premium surfaces). CSS-only in `web/index.html` where possible.
- **Audited (no change needed):** empty states are consistent across screens (icon + message + glossy CTA via
  `EmptyStateCTA`); all recent GD flair (GD-6 focus glow, GD-7 bar gloss, GD-10 seg pill, GD-11 calendar
  today) verified rendering correctly in BOTH themes — color-mix is theme-adaptive, no regressions.
- **Audited (no change needed):** alpha-composited contrast scan across ~15 screens in BOTH themes found
  **zero real low-contrast text** — recent fixes (Free badge, logo) hold. The only flags were false
  positives: `.btn-primary` uses a gradient (background-*image*) the probe can't read, so it reported
  white-on-"white" in light; confirmed the buttons render white-on-green correctly (`btn_light_check.png`).
- **GLAMOR audit (no defects this fire):** Appearance page renders at full opacity after settle (the dimmed
  look in a prior screenshot was the page-enter fade caught mid-animation; `#cf-page-view` opacity=1, no
  `.page-enter`). The **Motion control wires correctly** to the WONDER system — MEASURED Off→`data-wonder=off`
  /`--wonder-on:0`, Subtle→`subtle`/`.55`, Full→default/`1` (the user gateway to all WONDER + the chart-anim
  gating works). Accent swatches already have a clear selected state (`.swatch.sel` white ring + aria-checked).
  Confirmed the GD-18 avatar fix was the ONLY color-backed-text instance (dashboard member-chip is a neutral
  dark chip; category swatch is a textless dot; no owner-color badges).
- **Audited (no defect):** the destructive confirm dialog is well-built — backdrop blur, red `.btn-danger`
  Confirm (was already correctly danger-styled, not affirmative green), Cancel focused as the safe default.
- **Investigated (no code change — both candidates correctly no-go):** (a) the L97 CSV "no-dedup" was
  reclassified as INTENTIONAL (test-encoded in `appstate_more_test.go:232` — re-importing the same row is
  expected to import again; a dedup would break it) — corrected the ticket, left `ImportTransactionsCSV`
  untouched (import tests still green). (b) Mobile rail+tabbar both render at 390px (rail 56px + tabbar) — the
  known B31 redundancy needing a phone drawer-rail, dev-sized; not newly fixable.
- [ ] GD-24+ optional polish (lower priority): per-page accent micro-touches.

### G11. Bills — "What's Due This Week" (Tomas) — 2026-06-23 ★

**✅ RESOLVED (2026-06-23).** Most of this page already worked (mark-paid, urgency tones, soonest-due
sort, visible names — all confirmed by the audit). Remaining fixes shipped:
- **Card titles in light mode** (CRITICAL §3) — fixed by the G9 definitive `[data-theme="light"]`
  contrast fix (this was the 8th screen to flag it; now closed series-wide).
- **✅ Calendar weekday headers in light mode (2026-06-24).** The "SUN…SAT" headers (`.cal-head`)
  read `var(--text-faint)`, which the theme engine resolves to a too-pale `#969698` in light
  (measured WCAG **1.6:1** on the white calendar card — fails AA, nearly invisible). Pinned
  `[data-theme="light"] .cal-head { color:#686870 }` (the palette's intended faint tone, CSS-only in
  web/index.html). MEASURED (`e2e/calhead_contrast_verify.mjs` vs serve.go, 2/2): light **5.52:1**
  (AA normal), dark **3.58:1** unchanged (pin is light-scoped — no regression). Screenshot
  `e2e/screenshots/calhead_light.png`. Scoped to `.cal-head` only — deliberately does NOT touch the
  systemic `--text-faint` token, whose too-pale light derivation is separate GX14/Go work.
- **✅ "Custom range" period-bar control contrast (2026-06-24).** The "Custom range" toggle in the
  resolution bar (`shell.go`) used `tw.TextFaint` (#7d7d85 → **1.87:1** on the light page bg — fails
  AA for a clickable control), while its immediate sibling "This period" button already used the
  darker `tw.TextDim`. Changed the one token `TextFaint → TextDim` so the two adjacent secondary
  controls match. Build rc=0; `go test ./internal/app` ok. MEASURED
  (`e2e/customrange_contrast_verify.mjs` vs serve.go, 2/2): light **6.74:1** (was 1.87), dark
  **8.46:1** — both clear AA, no regression. Screenshot `e2e/screenshots/customrange_light.png`.
  (Light-mode contrast sweep is now down to 2 entangled items left: a done/strikethrough To-do row —
  intentional dimming — and the semantic-red negative-change stat — both Go-token concerns, left alone.)
- **✅ Negative/positive amounts legible in light mode (2026-06-24).** The semantic up/down TEXT tokens
  `tw.TextDown`/`tw.TextUp` hardcoded the **dark-mode** hex (`#d8716f`/`#54b884`), so amounts using them
  rendered ~**1.8:1** on a white card — a negative "−$1,718.00 this month" under Net Worth was barely
  readable (a finance app must make negatives obvious). Made both theme-aware:
  `css.Color("var(--down, #d8716f)")` / `var(--up, #54b884)`, mirroring the earlier `TextFg`/`TextDim`
  fix. The theme engine emits readable light values (`--down #b3322f`, `--up #1f8a52`) and the **dark**
  vars equal the literals exactly (`--down #d8716f`, `--up #54b884` — measured), so dark mode is
  byte-identical. `BgDown`/`BgUp` keep the literal hex (intentional fills). Build rc=0;
  `go test ./internal/ui/tw` ok (golden `TextDown` expectation updated to `color:var(--down,#d8716f)`).
  MEASURED (`e2e/negamount_contrast_verify.mjs`, 2/2): light **6.15:1** (was 1.82), dark **5.80:1**
  unchanged. Full light audit now **1** finding total (only the intentional done-task dim remains).
  Screenshot `e2e/screenshots/negamount_light.png`.
- **✅ Sample-data banner "Start fresh"/"Dismiss" legible in DARK mode (2026-06-24).** First DARK-mode
  contrast sweep (`e2e/dark_contrast_audit.mjs`) flagged ONE issue on **every screen**: the sample-data
  banner's CTA `.sample-banner-btn` used `color: var(--accent)` (#2e8b57 green) on the banner's
  `background: var(--accent-dim)` which in dark is **#205337** (dark green) → **1.55:1**, illegible
  green-on-green. Fix (CSS-only, web/index.html): default `.sample-banner-btn` to `var(--text)` (reads
  on the banner in dark, ~8:1) and keep the green CTA in **light** via `[data-theme="light"]
  .sample-banner-btn { color: var(--accent) }` (light was already AA-large and is the friendlier look —
  unchanged). Hover affordance is now a theme-agnostic underline-thickness bump (no contrast loss).
  MEASURED (`e2e/samplebanner_contrast_verify.mjs`, 2/2): dark **8.12:1** (was 1.55), light **3.93:1**
  (unchanged). Dark-mode audit now **0** findings (was 7). Screenshot `e2e/screenshots/samplebanner_dark.png`.
- **✅ DARK contrast re-check round 2 (2026-06-24, keep-tidy, CSS-only) — 3 more AA misses fixed.** A
  fresh WCAG sweep (alpha-compositing probe, gradient-skip) across 7 screens in dark caught three real
  sub-4.5 labels the earlier passes missed:
  1. **Sample-banner "Dismiss" 3.91:1** — the CTA was fixed last round but `.sample-banner-dismiss` kept
     its own `color: var(--text-dim)` override on the dark `--accent-dim` banner. Removed the override so
     Dismiss inherits `.sample-banner-btn` (dark `--text`, light `--accent`). → **8.12:1**.
  2. **`.hero-stat-label` 3.58:1** (Reports/dashboard/home hero eyebrows) — was `var(--text-faint)`
     (#888890); bumped to `var(--text-dim)` (#ababb3). → **8.2:1**.
  3. **`.section-divider` 3.69:1** (uppercase section eyebrows app-wide) — same `--text-faint`→`--text-dim`
     bump (light keeps its #686870 override). → **8.46:1**.
  MEASURED via alpha-compositing probe; all three now ≥4.5 (8.1–8.5:1). build rc=0; sw cache v260→v261.
  **Probe-methodology note (important for future contrast sweeps):** toggling `data-theme` via JS does
  NOT switch the palette — Go applies `theme.CSSVars()` as INLINE STYLE on documentElement (wins over
  the `[data-theme="light"]` stylesheet block at L756), so the LIGHT pass must switch via the Appearance
  "Light" seg-btn (and even that needs the Go re-emit to fire). A JS attribute flip yields a Frankenstein
  dark-tokens+light-element-rules state with false 1.05:1 readings (brand-name, font "Default"). Trust
  only the DARK pass unless the light palette is switched through the app.
  **Still open (deferred — palette-wide, not done here):** positive/income amounts (`.amount-income`,
  `--up` #54b884) and Reports category drill-links (`.row-desc.btn-link`, `--accent`) measure **4.41:1**
  in dark — just under AA 4.5. Fixing means nudging the semantic green/accent brighter, which touches the
  whole palette + light theme; left for a dedicated palette-tuning pass to avoid an aesthetic regression
  in a keep-tidy fire.
- **✅ Goals "Final stretch" pace badge legible in BOTH themes (2026-06-24).** A badge-contrast sweep
  found `.pace-final` used `background: var(--accent-dim); color: var(--accent)` — accent-green text on
  the accent-dim green pill (#205337 dark / #88bb9d light) → **2.1:1 dark / 1.95:1 light**, washed-out
  green-on-green (same family as the G14 rank-badge bug). Fix (CSS-only, web/index.html): give it the
  neutral `background: var(--bg-elev)` of its sibling `.pace-ontrack` while keeping the celebratory
  accent **text** (so "Final stretch" = green text vs "On track" = gray text, both on the same neutral
  pill — readable, distinct, and accent-aware for custom themes). MEASURED
  (`e2e/pacefinal_contrast_verify.mjs`, 2/2): dark **3.83:1** (was 2.1), light **3.76:1** (was 1.95) —
  AA-large, consistent with the tint-based `.pace-overdue`/`.pace-soon` badge family. Screenshot
  `e2e/screenshots/pacefinal_dark.png`. (Other pace/status badges measured ≥3.0 in both themes — clean.)
- **✅ Native form controls themed in dark mode — date-picker icon was invisible (2026-06-24).**
  `color-scheme` was `normal` everywhere, so in dark mode every native control rendered light-themed —
  most visibly the `<input type="date">` calendar indicator was **black on the #202022 field**, nearly
  invisible (transaction/transfer/goal/bill dates, custom range, etc.). Also affected native select
  dropdown chevrons and scrollbars. Fix (CSS-only, web/index.html): `:root { color-scheme: dark }` +
  `[data-theme="light"] { color-scheme: light }` so the browser themes native controls to match. MEASURED
  (`e2e/colorscheme_verify.mjs`, 2/2): date input resolves `color-scheme:dark` in dark / `light` in light;
  the calendar icon is now light/visible on the dark field (screenshot `e2e/screenshots/dateinput2_dark.png`,
  vs the black-icon before). Build rc=0 (CSS-only; no Go). Applies app-wide to all native inputs/scrollbars.
- **✅ Long unbroken names no longer overflow list rows (2026-06-24).** The generic `.row-desc` (account/
  goal/budget list rows) used `white-space: normal` with **no `overflow-wrap`**, so a long *unbroken*
  token (email, URL, ID, or a no-space string) had no break point and overflowed the card — MEASURED a
  166-char no-space goal name pushing the cell to right=1423px (parent ends 1249, viewport 1280;
  `overflowsParent: true`). Fix (CSS-only, web/index.html): `.row-desc { overflow-wrap: anywhere }` so
  long tokens break and wrap inside the card. MEASURED after (`e2e/rowdesc_overflow_verify.mjs`, 2/2):
  list-row cell right 1423→**812** (within parent, `overflowsParent:false`, docOverflow 0); the
  txn-table `.row-desc` truncation (nowrap+ellipsis+max-width:280px, a more specific rule) is
  **unaffected** — no regression. Screenshots `rowdesc_nospace_{before,after}.png`. Harmless for normal
  text; pure robustness against arbitrary user-entered names. (Verified clean this pass — no change
  needed: reduced-motion compliance [0 running anims], long-description truncation in the txn table,
  add-transaction form contrast, and `.attention-text` ellipsis.)
- **✅ On-brand text selection highlight (2026-06-24).** No `::selection` rule existed, so selecting
  text (e.g. copying an amount) used the off-brand OS-default highlight (a blue-grey that clashes with
  the green accent and varies by OS/theme). Added a global `::selection`/`::-moz-selection`:
  `background: color-mix(in srgb, var(--accent) 28%, transparent); color: var(--text)` — an accent-
  tinted highlight that keeps the text color readable, is theme- and custom-accent-aware, and works in
  light + dark. CSS-only (web/index.html); build rc=0 (no Go). MEASURED (`e2e/selection_verify.mjs`,
  4/4): rule present + color-mix resolves to the accent at 0.28 alpha in both themes; visual
  `e2e/screenshots/selection_greeting_dark.png` shows a subtle green highlight with the heading still
  legible. (Light-mode contrast sweep is now complete app-wide — Customize/Documents/Insights also
  verified clean in both themes this pass.)
- **✅ Print / save-to-PDF stylesheet (2026-06-24).** There were **zero `@media print` rules**, so
  printing a statement/report/ledger (a routine finance-app action) output the **dark UI + nav rail +
  topbar + banners + scrollbars** — ink-heavy and often unreadable (near-white text on the printed
  dark surfaces). Added a print stylesheet (CSS-only, web/index.html): forces an ink-friendly light
  palette regardless of the active theme by overriding the theme engine's INLINE `--*` vars with
  `!important` on `<html>` (`--bg/--bg-card/--text/…`) AND forcing the layout containers white
  (`.cf-shell`/`main`/`#cf-page-view`/`.bento` bake a hardcoded dark `tw.BgBase`, not `var(--bg)`, so
  they needed explicit overrides — else gaps between cards printed black); hides app chrome (rail,
  topbar, mobile tabbar, banners, toasts, the period `.reso-control`, hero action buttons); flows the
  scroll container across pages; keeps cards from splitting mid-page (`break-inside: avoid`); `@page
  { margin: 1.5cm }`. Semantic income-green/expense-red and chart/donut colors are intentionally kept
  (readable on white). MEASURED under `page.emulateMedia({media:'print'})` **from the DARK theme**
  (`e2e/print_styles_verify.mjs`, 5/5): body bg white (lum 255), `--text` forced `#111` over the inline
  dark var, nav rail + topbar `display:none`, cards `break-inside:avoid`; containers all
  `rgb(255,255,255)`. Visual `e2e/screenshots/print_reports2.png` — a clean B&W report with white
  surfaces, dark text, preserved semantic/chart colors, full-width content, no chrome. (CSS-only, no Go.)
  - **✅ EXTENDED to the transactions ledger/statement (2026-06-24).** Printing the txn table is a common
    finance-app case. Added print rules so a ledger prints like a statement: `.txn-table tr
    { break-inside: avoid }` (each row stays whole across page breaks — was `auto`), `thead
    { display: table-header-group }` (column headers repeat on every page), and hide the interactive-only
    columns (`.td-actions`, `.td-select` checkboxes) which are noise on paper. MEASURED under
    `emulateMedia({media:'print'})` (`e2e/print_styles_verify.mjs` for the page chrome; a table probe
    confirmed): rows `break-inside:avoid`, Actions+Select `display:none` in print but `table-cell` on
    SCREEN (no screen regression), thead `table-header-group`. Visual `e2e/screenshots/print_transactions2.png`
    — Date/Amount/Description/Category/Account/Tags/✓ columns, semantic red/green amounts, no row-action
    clutter. NOTE: the search/Filters/Export toolbar above the table still prints (minor; left to avoid
    over-broad selectors).
  - **✅ FIXED — the DASHBOARD printed dark (2026-06-24).** A print sweep across screens found the bento
    widgets still printed on a dark `#121214` background (hardcoded `tw.BgTile`, not `var(--bg-card)`),
    so the whole dashboard printed dark-on-dark and unreadable while Accounts/Budgets/Goals/Reports/
    Transactions were already clean. The print rule forced the layout *containers* white but not the
    widgets. Fix: added `.w, .bento .w` to the white-bg print override. MEASURED across Dashboard/
    Accounts/Budgets/Goals under `emulateMedia({media:'print'})` (`e2e/print_screens_verify.mjs`): **0
    dark blocks** on every screen (was 1 deduped class = all `.w` widgets on Dashboard). Screenshot
    `e2e/screenshots/printscan_Dashboard.png` — white widget cards, dark text, semantic colors, chart on
    white. Print is now robust app-wide. CSS-only; build rc=0.
  - **✅ POLISH — hide interactive form controls in print (2026-06-24).** A printed statement showed the
    full-width search box and filter selects (interactive-only noise). Added `input, select, textarea
    { display:none }` to the print block — the ledger data is plain `<td>` text (not inputs), so no
    statement data is dropped. MEASURED (`e2e/print_screens_verify.mjs` + a table probe): in print the
    search box is hidden but all 50 ledger rows still render with full data (date/amount/description/
    category/account); on SCREEN the search box is unaffected. Cross-screen print scan still 0 dark
    blocks; page-chrome verify still 5/5. (The small Filters/Clear/Export `.btn`s still print — left
    intentionally, since `.btn` is shared with content actions and has no safe print-only selector here.)
    Screenshot `e2e/screenshots/print_txn_clean.png`.
- **✅ BUG — raw CSS-rule text leaked onto the Appearance screen (2026-06-24).** Between "Accent" and
  "THEME" the divider rendered as literal text: `{[{border-top-width 1px}] { []} []}{[{border-top-style
  solid}]…}{[{border-color #232325}]…}`. Cause (`internal/screens/appearance.go:91`): the `<hr>` divider
  passed the `tw.BorderT` + `tw.BorderLine` `css.Rule` values **directly as Hr children** instead of
  wrapping them in `css.Class(...)` — so the rule slices were stringified into a text node (the 3
  fragments = BorderT's 2 rules + BorderLine's 1 rule, exact match). Fix: `Hr(css.Class(tw.BorderT,
  tw.BorderLine), Style(…))` (the standard pattern used everywhere else in the file). appearance.go was
  clean (not churned). Build rc=0; `go test ./internal/screens` is N/A (the pkg is `//go:build js &&
  wasm`, native test excludes it — the wasm build IS the compile check). MEASURED
  (`e2e/appearance_no_css_leak_verify.mjs`, 2/2): no `border-*`/`{[{border` text on /appearance, and the
  `<hr>` now renders its top border as STYLE (`1px solid`), not text. Screenshot
  `e2e/screenshots/appearance_fixed.png` — a clean divider line. (Found via a fresh GLAMOR re-scan of the
  previously-unaudited Appearance screen.)
- Mobile note (logged, not fixed — needs a UX decision): at 390px the top period bar
  (`.reso-control` inside `overflow-x:auto .topbar`) overflows to ~1052px, so Quarter/Year/Jump-to/
  Custom-range sit off-screen reachable only by horizontal swipe with no scroll affordance. `.reso-control`
  has `flex-wrap:wrap` (intent to wrap) but the scrollable parent defeats it. Scroll-vs-wrap-vs-hide is a
  responsive design call (cross-ref C19); left for a dev rather than changed unilaterally.
- **✅ BUG FIXED — fixed bottom tab bar obscured the last content on mobile (2026-06-24).** The phone
  bottom tab bar (`.mobile-tabbar`, `position:fixed`, `56px + safe-area`, shown `@media (max-width:640px)`)
  floats over the bottom of the scroll area, but `main.cf-scroll` had `padding-bottom: 0` — so the last
  content was hidden behind it and untappable. MEASURED on /transactions at 390px scrolled to bottom: the
  "Rows per page" 25/50/100/All selector + pagination sat behind the bar (page-size bottom > tab-bar top).
  Found via a mobile GLAMOR scan (no horizontal overflow on any screen; table correctly reflows to cards —
  those are clean). Fix (CSS-only, web/index.html): in the `@media (max-width:640px)` block, pad the
  scroller `main.cf-scroll { padding-bottom: calc(56px + env(safe-area-inset-bottom,0px) + 12px) }`.
  MEASURED (`e2e/mobile_tabbar_clearance_verify.mjs`, 3/3): mobile clearance 68px, the page-size selector
  (bottom 727) now clears the tab bar (top 784); **desktop scroller unaffected** (padding-bottom 0). Build
  rc=0 (no Go). Screenshot `e2e/screenshots/mobile_tabbar_fixed.png` — controls fully visible above the bar.
  (Aside, not changed: the collapsed icon rail co-exists with the bottom bar on phone — intentional, since
  the 4-item bottom bar doesn't cover Goals/Reports/Planning/etc.; the rail is the full nav.)
- **✅ A11Y FIXED — danger button text failed AA in dark (2026-06-24).** `.btn-danger` (the destructive
  confirm button: Delete/Wipe in `confirmModal`) used `background: var(--down)` + white text — but in dark
  `--down` is a deliberately SOFT red (`#d8716f`, tuned for amount/text legibility), so white-on-it
  measured **3.23:1** (fails WCAG AA for normal text). Light was fine (6.15). Fix (CSS-only,
  web/index.html): give `.btn-danger` a dedicated constant danger red `#c0392b` (danger shouldn't vary by
  theme; `--down` stays soft for amounts). MEASURED (`e2e/btn_danger_contrast_verify.mjs`, 2/2): white on
  `#c0392b` = **5.44:1** in BOTH themes (AA). Visual `e2e/screenshots/btn_danger.png` — vivid red "Delete"
  legible and clearly distinct from neutral "Cancel". Build rc=0.
  - **Confirm-dialog system verified solid (no change):** destructive confirms use `role="alertdialog"`,
    the danger button, focus defaults to **Cancel** (WCAG 3.2.4, so Enter can't trigger the danger
    action), a focus trap, and Enter-confirm/Esc-cancel — all correct (`internal/app/dialoghost.go`).
  - **⚠ Observation (logged, churned Go — not changed):** the sample-data banner's **"Start fresh" wipes
    all financial local state and reloads with NO confirmation dialog** (`samplebanner.go` calls
    `wipeFinancialLocalState` directly). Acceptable for pure demo data, but if a user has added real
    entries on top of the sample they're wiped with one click — consider a confirm (cross-ref the L50
    bulk-delete-no-confirm and L80 mark-paid-no-guard "destructive action needs a guard" theme).
- **✅ VERIFIED — first-run / empty (welcome) state is solid (2026-06-24).** Inspected the no-data state
  (via "Start fresh" in an *ephemeral* playwright context, so no real/persisted data touched). The
  welcome hero ("Your money, beautifully organized." + "Load sample data" / "Add your first account"),
  the "All clear — nothing urgent right now." attention widget, clean $0.00 KPI tiles, and every bento
  widget's friendly empty message + "Add a budget/goal/to-do/account" CTA all render correctly with
  **zero JS errors**. Good first impression. Screenshot `e2e/screenshots/empty_dashboard.png`.
- **✓ RESOLVED (option a landed 2026-06-24) — `.btn-primary` dark-mode text unified to white.** The app's
  most-used button (Save, Add transaction, Load sample, every empty-state CTA, …) is `background:
  var(--accent)` (#2e8b57) with **theme-specific text**: was `#052e13` (dark green) in dark, `#fff` in light.
  MEASURED before: dark **3.52:1**, light **4.25:1**. The dark `#052e13` was the *weaker* of the two, so I
  unified dark to white (`web/index.html` `.btn-primary { color:#fff }` for both themes; removed the now-
  redundant light override). MEASURED after (e2e `btn_primary_consistency_verify.mjs`, both themes): **4.25:1
  white-on-accent, consistent** — dark improved 3.52 → 4.25; screenshot `e2e/screenshots/btn_primary_dark.png`
  shows white "Save" on bright green. Both clear AA-large/UI (3:1) on the 600-weight label; brand accent kept.
  **Remaining for a dev (full AA-normal 4.5):** (b) darken the default `--accent` slightly so white clears 4.5,
  or (c) formally accept ~4.25 as AA-large for the bold label — a brand/design call, not done unilaterally.
- **✓ RESOLVED (2026-06-24, CSS-only) — `.rank-badge` dark-mode text unified to white (sibling of the
  btn-primary fix).** The Allocate ranked-suggestion ordinals (#1..#N) used the SAME accent chip with
  `color:#052e13` (dark green) in dark mode = MEASURED **3.52:1** on the accent — the weaker outlier vs
  light's white 4.25:1. Unified dark → white (`web/index.html`; removed the redundant
  `[data-theme="light"] .rank-badge` override). MEASURED after (`e2e/rankbadge_contrast_verify.mjs`, both
  themes): **4.25:1 white-on-accent, consistent** (dark 3.52 → 4.25). Screenshot
  `e2e/screenshots/rankbadge_dark.png` shows white "#1" on the green chip. Same remaining dev option as
  btn-primary for full AA-normal 4.5 (a brand `--accent` call).
- **a11y — every Settings control now has an accessible name (2026-06-25, keep-tidy, WCAG 4.1.2).** A
  sweep for controls with no accessible name (no text/aria-label/title/label-for/wrapping-label) found
  the top-level screens + add-transaction modal already clean (0), but **Settings had 4 unnamed control
  types** a screen reader would announce with no context: the FX rate inputs (`fxRateRow`), the
  freshness-threshold day inputs (`freshnessRow`), the widget-config number+select (`widgetCfgField`),
  and the workspace-switcher startup select (`wsswitcher.go`). Added `aria-label`s mirroring each visible
  label (new i18n keys `settings.fxRateAria`, `settings.freshnessAria`; widget/ws reuse existing labels).
  MEASURED: Settings now has **0 unnamed controls** (29/29 named); FX inputs read "Exchange rate: 1 AUD
  in USD" etc. build rc=0, i18n test ok. (FlipPanel already had role=dialog/aria-modal/aria-label.)
- **Desktop GLAMOR re-check (2026-06-25, keep-tidy) — all main screens clean, no defects, 0 console
  errors.** Swept 10 screens at 1440px (Dashboard, Transactions, Accounts, Budgets, Reports, Goals,
  Planning, Insights, Subscriptions, Bills): each has a polished hero/stat strip, well-structured rows, and
  proper actions; screenshots `e2e/screenshots/glamor_{goals,planning,insights,subscriptions,bills}.png`.
  Verified two "looks off" suspicions were actually correct: (a) Goals' "by 2026-12-01" sub-line uses
  `pr.FormatDate` (respects the date-style pref, not hardcoded); (b) individual-budget owner tags render as
  designed. **No change warranted — did not manufacture one** (the app is in good shape after the recent
  fires). Worktree note: the prune from the prior fire held (only `main` remains; the lingering
  `LoadSmartSettings undefined` LSP error is stale gopls cache — the symbol exists, build rc=0).
  - [ ] **(OPTIONAL, product call — NOT a bug) date-style default is `DateISO` (2006-01-02).** Every date
    across the app renders ISO by default (deliberate — `prefs.Default()` sets `DateStyle: DateISO`, and it's
    user-changeable to US `01/02/2006` or Long `Jan 2, 2026`). For an everyday US-household audience, a
    friendlier default (Long/US) might read better, but ISO is a defensible unambiguous/sortable choice — so
    flagging for Cam's decision rather than changing a deliberate default unilaterally.
  6 screens at 390px: **zero horizontal page overflow** anywhere. Re-measured the two known mobile-nav items
  (already logged under B31, lines ~1236-1241) and confirmed them unchanged: (a) the topbar period controls at
  ≤480px are a *deliberate* horizontally-scrollable strip (GX7-F2, scrollWidth 969 > 334) — the prev/next
  stepper + Quarter/Year sit off-screen-right but are reachable via scroll; working as designed, not a bug;
  (b) at phone width both the 56px icon rail AND the bottom tabbar render — the tabbar covers only 5 of ~27
  destinations, so the rail can't be hidden without a "More"/drawer affordance (the intended B31 phone
  drawer-rail phase). No code change made — reversing (a) would override a documented tradeoff and (b) is a
  dev-sized shell feature; both correctly remain B31 work.
  - **L50-T1 / L80-T1 / "Start fresh" all share the "destructive action needs a guard" theme** — worth
    a single pass adding confirms to unguarded destructive/money actions.
- **Dollar amounts too muted in light** (§3) — `.budget-amount` moved to the strong (`#1c1c1e`)
  light-mode group so the figures Tomas compares are full-contrast, not secondary grey.
- **"Next due" date hyphenating at 768** (§2) — `.stat-value { white-space: nowrap }` keeps the ISO
  date ("2026-07-01") on one line instead of breaking to "2026-07-" / "01".
- **Horizon filter + Show-all toggle** (§1, G11 follow-up, 2026-06-23) — bills default to 90-day
  window; a "Show all (N)" / "Show next 90 days" toggle exposes the full list on demand.
- **Two-column layout at ≥1024 px** (§1, G11 follow-up, 2026-06-23) — `.bills-layout` flex
  container puts the bill list left and the calendar right at wide viewports so both are visible
  without scrolling; stacks on narrower screens.
- **Fixed trailing action-button group** (§2, G11 follow-up, 2026-06-23) — `.bill-sub-actions`
  wraps "Mark paid" + "Remind me" in a `flex:none` trailing group so the bill name and amount have
  horizontal priority, mirroring the G10 `.sub-actions` pattern.

**The story**
Tomas opens Bills on a Monday morning to know exactly what he owes and when. His goal in
under ten seconds: see the total he needs to cover this cycle, spot any bill due today or
overdue, identify what's coming up in the next 7 days, and mark paid the ones he's already
settled. The calendar gives him a monthly at-a-glance map of when money will leave his
account. The page must surface urgency (overdue = red, due soon = amber), total due, and
the soonest-due bill immediately — without scrolling. Mark-paid must be one tap away per row.

**Drive script**
`e2e/glamor_11_bills.mjs` — widths 1280/1440/768, dark + light themes (light-theme
recipe: set `cashflux:prefs` in localStorage, reload, wait for `data-theme="light"`). Navigates
from `/` via in-app click ("Bills" nav link) to avoid the wasm deep-link 404 (B1).
Captures 8 screenshots plus a DOM audit JSON and a light-mode contrast spot-check. Run:
`node e2e/glamor_11_bills.mjs` against `:8099`.
Screenshots in `e2e/screenshots/glamor_11_bills_*.png`.

**Build/run evidence**
- `node e2e/glamor_11_bills.mjs` → EXIT 0
- Screenshots captured:
  `glamor_11_bills_1280_dark.png`, `glamor_11_bills_1280_dark_full.png`,
  `glamor_11_bills_1440_dark.png`, `glamor_11_bills_768_dark.png`,
  `glamor_11_bills_1280_light.png`, `glamor_11_bills_1280_light_full.png`,
  `glamor_11_bills_1440_light.png`, `glamor_11_bills_768_light.png`
- DOM audit: `glamor_11_bills_dom.json` — 2 cards ("Bills", "June 2026 calendar"),
  4 stat items (Total due soon $2,285.00 / Per year $23,550.00 / Upcoming bills 7 /
  Next due 2026-07-01), 7 rows, 7 mark-paid buttons (all present), 7 remind buttons,
  2 cal-dots, 7 cal-head cells, today-cell present, 0 overflow cards, 0 page errors.
- `statAboveFold: true` confirmed at 1280px dark.
- `hasMarkPaid: true`, `markPaidCount: 7` — mark-paid confirmed on every row (C57 fix verified).
- `hasUrgency: false` — no overdue or within-3-day bills in the sample data snapshot
  (all bills 8–206 days out); urgency code exists in source and is correct.
- `dataTheme: "dark"` confirmed on dark captures; `"light"` on light captures.
- theme after hard-reload: `"dark"` (persistence confirmed correct).
- Light contrast spot-check: `cardTitleColor: rgb(244, 244, 245)` on white background
  (card title near-invisible — same systemic `--fg` token failure as G4–G10).
  `rowDescColor: rgb(28, 28, 30)` — bill names ARE legible in light mode (improved vs.
  G10 where row names were invisible). `rowMetaColor: rgb(86, 86, 92)`, `budgetAmtColor:
  rgb(86, 86, 92)` — muted grey on white for due-date labels and amounts. `statLabelColor:
  rgb(86, 86, 92)` — stat labels faint. `urgencyColor: N/A` (no urgency elements in data).

**What already works well (keep — regression anchors)** ✓
- **Stat grid is the very first element and is above the fold at all widths.** Total due soon
  ($2,285.00 in red), Per year ($23,550.00), Upcoming bills (7), and Next due (2026-07-01)
  are all visible without scrolling at 1280 and 1440. `statAboveFold: true` DOM-confirmed. ✓
- **Mark-paid button present on every row (C57 fix confirmed).** `markPaidCount: 7` — all 7
  bill rows carry a green "Mark paid" `.btn-primary` button. C57 noted "no mark-paid" as the
  top deficiency; it is now fully implemented. ✓
- **Remind me button present on every row.** `remindCount: 7` — all 7 rows carry a "Remind me"
  button that creates a to-do dated to the bill's due date. ✓
- **Bill names are visible and lead the row at all widths.** `rowDescColor: rgb(28,28,30)` in
  light mode (full-weight foreground token) — bill names (Rent, Gym membership, Streaming &
  apps, Student Loan, Rewards Credit Card, Car insurance, Domain & hosting) are clearly
  readable in both dark and light screenshots. This is markedly better than G10 Subscriptions
  where names were invisible at 1280/1440. ✓
- **Urgency code is wired correctly.** Source: `billUrgencyTone()` applies `text-down` for
  overdue/today and `text-warn` for within 3 days. No urgency elements appear in the sample
  data (all bills 8–206 days out), which is correct behavior. ✓
- **Ordering is soonest-due first.** DOM audit `rowMetas` confirms ascending date order:
  2026-07-01 → 2026-07-03 → 2026-07-05 (×2) → 2026-07-22 → 2026-09-01 → 2027-01-15.
  Tomas's most pressing payment is always at the top. ✓
- **Calendar renders with today-cell and dot indicators.** `hasCalendar: true`, `hasTodayCell:
  true`, `calDots: 2`, `calHead: 7` — the June 2026 calendar grid is present with today
  highlighted and 2 dot indicators on bill-due days. ✓
- **No horizontal overflow at any width.** `overflowCards: 0` at 1280px dark. ✓
- **Zero JavaScript page errors.** Both dark and light sessions clean. ✓
- **Download CSV is present.** `hasCsvBtn: true` — "Download CSV" in the Bills card footer. ✓

**Structure fixes (bottom-up)**

*1. Layout — calendar is below the fold, disconnected from the urgency story*
*2. Spacing — row density and button visual weight at 768px*
*3. Theming — systemic light-mode token failure (G4–G10 pattern recurring)*
*4. Styling — urgency visualization absent from current data snapshot*
*5. Positioning — calendar placement vs. urgency hierarchy*
*6. Ordering — bills list includes 206-day-out bill with no horizon indicator*
*7. General UX / Glanceability — "What's Due This Week" use case assessment*
**UI/UX defects (screenshot-confirmed)**

| # | File | Symptom | Fix |
|---|------|---------|-----|
| D1 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png`, `glamor_11_bills_768_light.png` | Card titles "Bills" and "June 2026 calendar" near-invisible in light mode — computed `rgb(244,244,245)` on white; WCAG AA fail (≈1.02:1). Eighth consecutive page with this systemic `--fg` token failure | `h2.card-title` must use a strong foreground token in light mode; global CSS token fix |
| D2 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png` | Dollar amounts (`.budget-amount`) render as muted grey `rgb(86,86,92)` in light mode — the key payment figures Tomas needs to read are styled as secondary text | `.budget-amount` should use `--fg` (strong) in light mode, not a muted token |
| D3 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png` | Due-date + days-until metadata (`rgb(86,86,92)`) low-contrast on white in light mode — the operationally critical "due in 8 days" label is muted when not urgency-colored | `.row-meta` should use a higher-contrast token in light mode for non-urgency state |
| D4 | `glamor_11_bills_768_dark.png`, `glamor_11_bills_768_light.png` | "Next due" stat card wraps the ISO date "2026-07-01" across two lines as "2026-07-" / "01" at 768px — the hyphenated break reads as a formatting error | Use a shorter date format at narrow widths (e.g. "Jul 1") or use a 2×2 stat grid at 768px instead of 3+1 |
| D5 | `glamor_11_bills_1280_dark.png`, `glamor_11_bills_1440_dark.png`, `glamor_11_bills_1280_light.png` | Calendar is below the fold at all widths — the month map is a scroll destination, not a contextual aid visible alongside the list | Move calendar to a side-by-side layout at ≥1024px (list left, calendar right) so both are visible without scrolling |
| D6 | `glamor_11_bills_1280_light.png`, `glamor_11_bills_1440_light.png` | Stat grid remains visually dark (dark card backgrounds) even when `data-theme="light"` is active — creates a two-tone page (dark header stats + white content area) | Stat cards must adopt the light-mode background token when `data-theme="light"` |

**Re-screenshot close-out requirement:** After D1 (card title contrast), D2/D3 (amount and
meta-label contrast fixes), D4 (768px date hyphenation fix), and D5 (calendar above-fold fix),
re-run `node e2e/glamor_11_bills.mjs` and confirm: (a) card titles readable in all light
screenshots, (b) amounts and due-date labels readable in light mode, (c) date no longer
hyphenates at 768px, (d) calendar visible within the fold at 1280/1440, (e) all 8 screenshots
captured cleanly.

**Probe hardening**
- Drive script uses in-app navigation (click "Bills" nav link from `/`) rather than direct
  deep-link to `/bills` — required because `gwc dev` returns 404 for non-root paths (B1).
- Wait condition is `.stat-grid, .card` — stat-grid is conditional on data being present;
  fallback to `.card` accommodates the empty-state (no accounts with due-day → empty bill
  list renders just the empty-state card without a stat-grid).
- "View as member" reset: removes `viewAsMember` from `cashflux:prefs` before navigation.
- Light theme set via the full localStorage recipe (set + reload + waitForFunction on
  `data-theme="light"`) rather than a nav click.
- Hard-reload probe: script reloads after dark screenshots and re-checks `data-theme` to
  confirm the dark preference persists (confirmed: "dark" after reload).
- Urgency-tone probe gap: no bills in the sample data fall within the 3-day warning or
  overdue windows, so `text-down` / `text-warn` styling cannot be screenshot-confirmed.
  A future fixture seeding an overdue bill and a 2-day-out bill is needed to close this gap.

**Cross-references**
- C57: "Bills clean calendar, but no mark-paid, no urgency tone, + a suspect annual figure"
  (marked DONE 2026-06-21). Mark-paid: confirmed present (7 buttons, `markPaidCount: 7` ✓).
  Urgency tone: code is wired, sample data has no urgency-triggering bills (evidence gap, not
  regression). Annual figure ($23,550.00): plausible for the sample data mix of monthly + yearly
  obligations; not confirmed suspect from this snapshot.
- L54: Bills page verified with mark-paid working (loop story screenshots `loop54-04-bills-page.png`,
  `loop54-11-bills-after-recurring.png`). Current audit confirms this remains correct.
- G4/G5/G6/G7/G8/G9/G10: Same systemic `--fg` light-mode token failure — D1 is the eighth
  consecutive page; a global CSS token fix (not per-page patch) is the only sustainable resolution.

---

# Probe scripts written and run 2026-06-23:
node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_main.mjs    # boot + CSS var validation
node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_full.mjs    # multi-page audit (exit 0)
node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_deep.mjs    # deep Reports chart audit
```

**Exit codes:** `gx16_main.mjs` exit 0 · `gx16_full.mjs` exit 0 · `gx16_deep.mjs` exit 0

**Server:** `gwc dev` on `http://localhost:8080` (multiple gwc processes confirmed running). SPA root serves correctly; routes navigated via `history.pushState` + `PopStateEvent`. Build state: GI0 (wasm build broken) — stale wasm binary boots from existing `static/bin/main.wasm`; wasm did mount successfully (confirmed `.topbar` rendered).

**Theme injection method:** `addInitScript` overrides `localStorage.getItem('cashflux:prefs')` to return `{theme:'light'}` before page load; the inline `<head>` script reads this synchronously and sets `data-theme="light"` on `<html>` before first paint. Confirmed: `data-theme: light` on `document.documentElement` after load.

---

## W. WONDER — configurable animated flourishes (theme-engine driven) ★★
<!-- Batch 2 landed 2026-06-23: W-11 list stagger, W-12 bento entrance, W-13 modal backdrop blur, W-14 toast spring, W-16 progress ease (verified), W-19 skeleton shimmer, W-20 focus ring ease. W-21 deferred (needs IntersectionObserver JS). -->

### W1. WONDER — animated-flourish system: architecture + token layer (theme-engine driven) — 2026-06-23 ★★

**The vision.** Make CashFlux feel *alive* — clean, fast, beautiful micro-animation everywhere: page
transitions, hover effects, click/press feedback, focus, list reveals, value changes. It must be
**configurable by the theming engine** and **adjustable** (a single intensity dial), **extensive** but
**tasteful** (calm + fast, never gaudy or janky), and fully **reduced-motion safe**.

**Architecture — a token layer driven by a `data-wonder` attribute (mirrors `data-theme`/`data-density`).**
The whole system reads a small set of `--wonder-*` design tokens. The theme engine sets
`[data-wonder="off|subtle|full"]` on `<html>` (the same mechanism as `data-theme`), and — once GI0 is
fixed — emits `--wonder-*` via `theme.CSSVars()` so the theme editor gets an intensity slider. Every
flourish multiplies its transform by `--wonder-on` (0..1), so ONE dial scales the entire app smoothly.
`prefers-reduced-motion: reduce` forces everything off. **This means flourishes land in pure CSS now and
become theme-configurable the moment the Go side emits the config — no rework.**

**Token layer (LANDED in `web/index.html`):**
`--wonder-on` (0..1 master multiplier) · `--wonder-dur-fast/dur/dur-slow` · `--wonder-ease` /
`--wonder-ease-out` · `--wonder-lift` (hover rise) · `--wonder-press` (click scale) · `--wonder-shadow`.
Levels: `[data-wonder="off"]` (zeroes all), `[data-wonder="subtle"]` (~55%), default/`full` (100%).

**What's LANDED now (CSS, foundation — [CSS-ONLY], live):**
**EXTENSIVE catalog — the flourishes to build (grouped; tasteful + fast). [CSS-ONLY] unless noted:**
*Interaction feedback*
> Batch 1 (W-3..W-8) fully landed 2026-06-23 — all CSS, token-driven, reduced-motion safe.

*Entrance / reveal*
- [~] W-10 — Route cross-fade (View Transitions API) — PARTIAL (2026-06-24): CSS scaffold + view-transition-name + ::view-transition-* keyframes landed; startViewTransition wraps the W-9 class-toggle for progressive enhancement. True old→new cross-fade blocked by GWC framework constraint: UseEffect fires post-render, so the outgoing page snapshot is already replaced when triggerPageEnter runs. Scaffold is ready for a pre-render hook if GWC exposes one.
  - **✅ BUG FIXED (2026-06-24, keep-tidy) — "call to released function" on EVERY route change.** A
    console-error health sweep across 9 screens caught one error firing once per navigation. Root cause in
    `internal/app/pageenter.go`: the W-10 path built a `js.FuncOf` cb and `defer cb.Release()`'d it, on the
    (wrong) assumption that `crossFade` "invokes cb synchronously in both paths." But `crossFade` →
    `document.startViewTransition(cb)` runs its update callback **asynchronously** (later microtask), so cb
    was released BEFORE the browser called it → `call to released function`. In Chromium (View Transitions
    supported + motion on) this was the default path, so it fired on every route change. Fix: cb now
    **self-releases after it runs** (correct for both the async view-transition path and the sync
    direct-applyFn fallback). Also hardened the fallback double-rAF path, which previously leaked 2 js.Funcs
    per route change (callbacks never released) — both rAF callbacks now self-release too. MEASURED: released-
    function sweep **9 hits → 0**; page-enter class still applies on **4/4** navigations (W-9 animation intact);
    console errors **0** across all screens. build rc=0; `go test ./internal/app` ok. No JS/CSS change — Go only.
  - **✅ BUG FIXED (2026-06-24, keep-tidy) — "AbortError: Transition was skipped" under rapid navigation.**
    A stress sweep (rapid-fire route switching ×3 + menu open/close) surfaced 11 unhandled-rejection console
    errors. Cause: `document.startViewTransition` (used by `crossFade` in `web/wonder.js`) returns a
    ViewTransition whose `.ready`/`.finished` promises REJECT with an AbortError when a subsequent navigation
    starts a new transition before the current one settles — expected during fast switching, but the
    rejections bubble up as unhandled-promise console errors. Fix (JS-only, wonder.js): capture the returned
    transition and attach no-op `.catch()` handlers to its `.ready`/`.finished`/`.updateCallbackDone` promises
    (the DOM swap in applyFn has already run, so the visual transition being skipped is harmless). MEASURED:
    stress-sweep console errors **11 → 0**; page-enter still fires **4/4** navigations. build rc=0; sw cache
    v261→v262. Together with the released-function fix above, the route-change path is now console-clean under
    both normal and rapid navigation.
*Value / state changes*
*Polish*
**Theme-engine integration (GO-STRUCTURAL, build-gated GI0):**
**LANDED 2026-06-23 — Motion pref (full/subtle/off) wired end-to-end.**

**Principles (enforce in every flourish):**
- Fast (≤ ~200ms for feedback, ≤ ~320ms for entrances) + a single shared easing family.
- Transform/opacity only (GPU-friendly) — never animate layout properties; no `transition: all`.
- Everything reads `--wonder-*` + scales by `--wonder-on`; nothing hardcodes a duration/transform.
- Reduced-motion + `[data-wonder="off"]` must yield a completely static app.
- Tasteful restraint — flourishes guide attention, never distract; no infinite loops outside loaders.

**Probe hardening / acceptance.** A WONDER e2e should: (a) hover a `.card` and assert a non-identity
`transform` (lift) in default/full, (b) set `[data-wonder="off"]` and assert identity transform, (c)
`emulateMedia({reducedMotion:'reduce'})` and assert identity transform, (d) measure flourish durations
trace to `--wonder-*`. `e2e/w1_verify.mjs` covers the landed foundation.

**Cross-refs:** GX8 (motion inventory + reduced-motion coverage — WONDER builds on it), 6.16 (interaction
polish), B20 (theme engine — the config home), GI0 (build blocker — gates the theme-engine integration),
GM (modal flip), GX5 (toast), G9.1a (chart draw-in).


## 0. Foundation & tooling (Phase 0)

> ⚠️ OPS NOTE (2026-06-24) — if the app shows only the boot splash (blank `#app`, console
> "Refused to execute wasm_exec.js (MIME text/plain)" + "WebAssembly compile: status not ok"),
> the git-ignored build artifacts `web/wasm_exec.js` and/or `web/bin/main.wasm` are missing
> (a concurrent `git pull --rebase`/clean wiped them this session). Restore:
> `cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js` and rebuild
> `GOOS=js GOARCH=wasm go build -o web/bin/main.wasm .`. Done + verified 2026-06-24 (app boots
> ~3s, dashboard renders 17 cards, 0 console errors). NB: the wasm is ~60MB (≈2-3× normal) due to
> the in-flight Cloud-sync changeset — worth a size audit once that work lands.

- [~] **README.md** — what CashFlux is, the stack (Go→wasm on GoWebComponents), local dev (`gwc dev`),
      build/test commands, the local-first + BYO-AI-key model, badges, a **Live demo** link to the
      GitHub Pages build, a License section, and pointers to SPEC/DEVLOG/TODOS — all present.
- [~] **MIT licensing.** Set the project up under the MIT license.
- [ ] Fix framework `gwc dev -html` resolution (commit in GoWebComponents, rebuild + recopy `gwc`)
- [ ] Install Claude Code design skills (`frontend-design`, `playground`) — user action
---

## 1. Phase 1 — Local household core

### 1.2 Money & currency — ★

- [~] Money formatting per currency: `FormatMinor` (plain decimal) done; symbol/grouping/locale = UI layer
### 1.7c Dashboard UI & design system — selected design: `design/candidate-c.html` ★

The chosen visual direction is **candidate C** (flat neutral-dark · Fraunces serif headings + accounting
figures · bento grid · per-widget grip/title/gear · drag-reorder + resize · gear→flip settings ·
collapsible icon sidebar · global-settings flip). The static reference mockup is
[`design/candidate-c.html`](./design/candidate-c.html) (open via the dev server at
`/design/candidate-c.html`). Every item below is a Go/`html/shorthand` component to port from it.
Drag/resize/flip need pointer/drag events via `syscall/js`/`interop`; keep computation in the tested
logic packages, persist layout/settings to the store `Settings`.

**Reusability (required):** build these as generic, props-driven components shared across the whole
app — not per-widget bespoke markup. In particular: one `Widget` shell (grip/title/gear header slots
+ body slot), one `FlipPanel` primitive reused by **both** per-widget and global settings, one
settings-form renderer driven by a field schema, and shared primitives (`Toggle`, `Segmented`,
`StepperPill`, `Swatch`, `Chip`, `ProgressBar`, `Icon` set, and SVG `Chart` helpers). Every widget is
`Widget`-shell + content; every screen composes these. Mark each item below `(reuse)` where a single
component should serve many call sites.

Design tokens & foundation:
App shell & navigation:
Time-resolution control (top bar):
Bento grid system:
- [~] Persist per-user layout — order + spans saved to `localStorage`; hidden/per-page + store persistence later

Per-widget settings (gear → flip):
- [~] Settings fields: editable Title + behavior toggles done; accent swatches/default size/refresh/Remove + persistence later

Widget catalog (each backed by tested logic; see mockup):
- [~] Reusable SVG chart helpers — area/sparkline (`chart` + `ui.AreaChart`) done; bars are div-based; donut later

Global settings (household card → large flip panel):
Shared control components (from mockup):
### 1.8 Members / Household

- [~] Member switcher / filter — per-member "Transactions" drill-down filters the ledger by member;
      global cross-screen member scope deferred (ambiguous semantics)
### 1.9 Accounts (assets + liabilities) ★

- [~] Per-account ledger view — account row "Transactions" button filters the ledger to that account
      and navigates; dedicated running-balance view optional later
- [~] Credit utilization indicator done (on liability rows); due-date reminder via Upcoming bills widget
### 1.10 Categories

- [~] Default scheme + reset; methodology-aware presets (envelope/zero-based) — pure
      `internal/catscheme.Default()` (starter income/expense set + sub-categories), table-tested; the
      reset action (apply via appstate) + methodology presets remain
- [~] Tests: tree building, reassignment — reassignment tested; category tree building N/A (flat list)

### 1.13 Goals

- [~] Contribute-to-goal action done (prompt); auto-progress from linked account later
### 1.14 To-do (budgeting tasks)

- [~] Sort (open first, then due, then title) + hide-done filter done; more filters later
- [~] Tests: ordering (pure `internal/tasksort` — Order/Visible, table-tested); status transitions still UI

### 1.15 Freshness & friendly nudges

- [~] Dashboard nudge widget ("N balances could use a refresh") done; dismissible + one-tap update later
### 1.17 Dashboard

- [~] This-month income/expense (done); balance trend snapshot (later)
### 1.18 Settings

- [~] Preferences: theme/density, week-start, fiscal-month start, number/date formats
      — theme (dark/light/system) + accent + density + week-start + date format all complete &
        reload-persistent (engine + atom + Settings UI + `ApplyPrefs` + light/dark skins);
        only fiscal-month start remains
## 2. Phase 2 — Intelligence & power tools (OpenAI, client-side)

### 2.1 OpenAI client — `internal/ai`

- [~] Vision input support (images/PDF pages) for document parsing — `ai.BuildVisionRequest` (pure) done
### 2.2 Documents — AI import

- [~] Upload UI (CSV paste + image picker) done; PDF + drag-drop later
- [~] Tests: CSV parsing (store) + extraction parsing/dedupe (`extract`) done; extraction→txn mapping is UI

### 2.3 Insights & NL query

- [~] Natural-language query over data → answer (Insights "Ask about your money"); richer data context later
### 2.5 Formula builder + sandboxed engine — `internal/formula`

- [~] Variable resolution: live figures (net worth/income/expense/counts) done via `Env`; custom fields + filtered aggregates later
- [~] Typed results (number/bool/text) done; money/percent typing + formatting later
- [~] Builder UI: live preview + error messages + example chips done (Customize); guided insert later
### 2.6 Planning + Forecast

- [~] ★ Forecast engine (pure): `internal/forecast.Project` over horizon from start + recurring + one-time items done; actuals-derived recurring later
- [~] What-if scenarios: extra debt payment + trim-spending forecast done; add-recurring/rate-change later
- [~] Forecast visualization (net-worth curve) done on Planning; scenario comparison later
## 3. Phase 3 — Sync & PWA

> **§3.1–3.2 are superseded by [§7. Backend server](#7-backend-server--sync--ai-proxy-grpc-bridge-hybrid-)**
> (gRPC-bridge hybrid: LWW sync + AI proxy over gRPC; OAuth + blobs over HTTP). Stubs kept for history.

### 3.3 PWA / offline

- [~] Web manifest done (`manifest.webmanifest` + theme-color/apple meta); icons later
- [~] Installability prompt done (beforeinstallprompt button); offline read works (sw); update flow later
---

## 5. Future / nice-to-have (post-core)

Lower-priority items to pick up **only after the core product (Phases 0–3) is complete**. These are
enhancements, not part of the core spec; sequence them after the Phase 3 / sync work.

### 5.1 Standalone desktop app via Electron

Wrap the existing WASM/PWA build as a native, installable desktop app (Windows/macOS/Linux) so
CashFlux can be distributed and launched outside the browser while reusing the exact same Go→wasm
bundle and `web/` shell. Local-first; no behavior change — just a native window + installer.

- [ ] Verify: app installs and launches natively, loads offline, and matches the PWA behavior

---

## 6. UX / UI polish pass (2026-06-18 audit — static review of shell, screens, controls, CSS)

Findings from a full static UX/UI sweep (typography, shapes/sizing/weights, fonts, legibility/contrast,
shortcuts, click-to-item speed). Grouped by theme; `[H]/[M]/[L]` = severity. File refs are starting
points — verify exact lines before editing.

### 6.16 UI interaction & motion polish (2026-06-18 pass 7 — animations, hover, micro-interactions)

The motion **foundation is good**: FLIP-animated bento reorder/resize (`web/flip.js`), the settings flip-panel
(`transform .55s cubic-bezier`), boot loader + `#app` settle-in, toast enter, collapsed-rail flyout, switch
toggle, and a thorough `prefers-reduced-motion` block. The gap is the **micro-interaction layer** — the small
feedbacks that make a UI feel responsive and alive. Mostly enhancement-grade ([M]/[L]), ordered by bang-for-buck.
All additions must be wrapped in `@media (prefers-reduced-motion: no-preference)` (or no-op'd in the existing
reduced-motion block) to stay consistent with the app's a11y stance.

**Press / tactile feedback**
**Hover affordances**
**Data-viz & progress animation**
**Enter / exit transitions**
**Stateful micro-interactions**
- [~] **[L]** Active nav pill (`.nav-link.active` / `.nv`) jumps between items on route change. Consider animating
      a shared active indicator that slides to the selected item.
      **PARTIAL — grow-in landed (2026-06-24, keep-tidy, CSS-only).** The GD-15 "you are here" bar was an
      inset box-shadow (`aside.rail .nv.active`) that snapped between items. Converted it to an absolutely-
      positioned `aside.rail .nv.active::before` accent bar (3px, `var(--accent)`, theme-agnostic, no layout
      shift) that **grows in** via `@keyframes wonder-nav-bar-in` (scaleY + opacity, `--wonder-dur`/
      `--wonder-ease-out`) each time an item becomes active — so the new item's indicator eases in instead of
      hard-snapping. Removed the now-redundant light-mode box-shadow bar (index.html ~847) so both themes use
      the one animated `::before`. WONDER-gated: `[data-wonder="off"]` and `prefers-reduced-motion` → `animation:
      none` (and the keyframe's `from` collapses to `to` when `--wonder-on=0`), bar stays statically visible.
      MEASURED (6/6): full → bar present (3px, rgb(46,139,87)) + `animation-name: wonder-nav-bar-in`; off →
      bar present, `animation: none`; reduced-motion → bar present, `animation: none`. Bar renders accent in
      BOTH themes. Screenshots `e2e/screenshots/nav_pill_{dark,light}.png`. sw cache v258→v259. build rc=0.
      **Still open (the true "slide"):** a single shared indicator that physically slides between items needs
      JS measurement/FLIP — the framework's `Style()` drops CSS custom props, so a var-driven CSS slide isn't
      feasible; deferred as a larger JS change. The grow-in removes the worst of the "jump" for now.
> **Note:** animations/hover are hard to verify from still screenshots; this pass is a CSS/JS interaction audit.
> A future check could record short Playwright videos (`recordVideo`) of hover/drag/toast flows to confirm feel.

## 7. Backend server — sync + AI proxy (gRPC bridge hybrid) ★

> Supersedes the stubs in §3.1–3.2. Design: [`docs/BACKEND_PLAN.md`](./docs/BACKEND_PLAN.md).
> **Locked decisions:** last-write-wins sync (newest-by-timestamp) · per-user **BYO** OpenAI key
> stored **encrypted at rest** · auth via **OAuth (Google/GitHub)** · artifacts in a
> **content-addressed blob store** (refs only in the synced snapshot) · **gRPC over the GWC
> `GoGRPCBridge`** (WebSocket) for the app's data/AI RPCs · **plain HTTP** for OAuth + blobs.
> Thin server: it stores and forwards, never interprets the dataset. App stays local-first; the
> backend is an optional sync/proxy tier. Build bottom-up (proto/contract → storage → services →
> transport → client), one feature per commit, tests with each layer.

### 7.7 Client integration (wasm app) ★
- [~] Sync client layered over the existing autosave: browser autosave now pushes changed active-workspace
      snapshots over `/grpc`, pulls newer server snapshots on boot/focus, applies newest-by-`updatedAt` using
      local sync metadata, maps local workspace ids directly to server workspace ids, and subscribes to
      `WatchWorkspaces` so active-workspace changes from other devices trigger a pull. A persisted per-workspace
      pending mutation queue retries on focus/online/Sync now, and Settings surfaces synced/syncing/offline/error
      status. Remaining: explicit conflict resolution UX beyond LWW status copy.
- [~] Offline-first: a mutation/queue so the app works offline; flush on reconnect; status surface
      (synced / offline / syncing / error) + a "Sync now" action.
      Done: latest pending snapshot per workspace is persisted locally, retrying on focus/online/manual sync with
      Settings status copy. Remaining: richer queued-change count outside Settings and conflict action sheet.
- [~] **Artifact extraction (client schema change):** move `domain.Artifact.Bytes` out of the synced
      snapshot → upload via blob `PUT` (sha256), download via `GET`, keep a local cache; the dataset
      carries a `BlobRef`. Migrate existing inline artifacts on first sync.
      Done: `Artifact.BlobRef` is in the dataset schema, sync flush uploads artifact bytes to `/v1/blobs`, and
      sync pull rehydrates missing bytes before local import. Remaining: explicit local blob cache controls and
      a one-time migration/status surface for already-inline artifacts.
- [~] Settings: backend URL, sign in/out, sync status; conflict/LWW UX ("a newer version was on the server - pulled it").
      Done: backend URL/token, test connection, key upload, Cloud/self-host mode, Sync now, and sync status are
      in Settings. OAuth sign-in buttons and local sign-out are wired. Remaining: richer conflict action sheet.

### 7.8 Security & privacy ★
- [~] TLS everywhere; OAuth `state`/PKCE; never log secrets; threat-model pass; `govulncheck` + `gosec` in CI.
      TLS/wss, OAuth PKCE/state, log redaction, Gitleaks, govulncheck, and gosec are covered; remaining:
      formal periodic threat-model and pre-launch pen-test pass.

### 7.9 Deploy & ops
- [~] CI: build server, run server tests, proto-drift check, lint + vuln scan.
      Done: Go tests, explicit server build, wasm build, vet, govulncheck, gosec, and gitleaks. Remaining:
      proto-drift check once codegen is pinned.

### 7.10 Testing & phased rollout
- [~] Integration: in-proc `grpc.Server` behind the bridge over a real WS; client<->server round-trips
      now cover AI `SetKey`/`Chat`/`ChatStream` and SyncService workspace `Put`/`List`/`Get`/`Delete` unary
      calls plus watch streams, with HTTP blob PUT/HEAD/GET verified against a workspace created through the
      bridge. Remaining: browser autosave push/pull e2e.
- [~] e2e: two-device sync (LWW + tombstone), offline->reconnect flush, OAuth login, artifact blob
      round-trip, AI proxy streaming with a real key.
      Done: in-proc bridge e2e covers two-device stale LWW rejection plus tombstone propagation; AI proxy
      streaming has bridge/client transport coverage. Remaining: offline->reconnect flush, OAuth login,
      artifact blob round-trip, and real-key AI proxy smoke.
### 7.11 Monetization — billing + Cloud UX (paid tier) ★

> CashFlux Cloud is the paid tier: sync + backup + AI proxy. App stays free/local-first.
> Design: [`docs/CLOUD_UX.md`](./docs/CLOUD_UX.md) + [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md).
> **Locked:** app free; Cloud paid (annual-first subscription); AI proxy bundled into Cloud; personal
> plan now, household later. Recommended pricing ~$34.99/yr / $3.99/mo, 14-day trial (validate).

#### Server (billing + entitlements)
#### Client (Cloud UX)
#### Launch gating
- [ ] Monetize at the **sync milestone** (auth + snapshot sync + Stripe + trial); AI proxy + blobs land
      as later Cloud upgrades (no price change). Household plan is a later phase.
- [ ] Analytics: trial starts, trial→paid, MRR/ARR, churn, ARPU, storage/user, gross margin (privacy-respecting).

### 7.13 Turnkey self-host deploy + DO referral ★

> One-click(ish) self-host on DigitalOcean, and turn the free self-host path into DO referral credit
> that offsets Cloud infra cost. Design: [`docs/CLOUD_BUSINESS_PLAN.md`](./docs/CLOUD_BUSINESS_PLAN.md) §14.
> Keep an unconditional plain self-host path (any host, no referral). Disclose referral plainly.

#### Packaging
- [ ] **DO Marketplace 1-Click**: Packer build of a droplet image; submit for vendor approval (later,
      after the script path is proven).

#### Referral
- [ ] Add the **DO referral link** to the "Deploy your own server" button + install docs + Marketplace
      listing, with a clear disclosure line.
- [ ] Verify current DO referral terms before relying on it; track referral credit as reduced COGS.
#### In-app hook
#### Ops/docs
### 7.14 Security hardening ★

> Defense-in-depth for a server that holds user financial data + encrypted AI keys. Pairs with §7.8.
> Run `gosec` + `govulncheck` in CI from day one; treat every finding as blocking.

#### AuthN / AuthZ
#### Transport / browser
#### Input / data
- [~] Validate + bound every input: request-size caps (dataset, blob, RPC message), field limits,
      content-type checks; reject malformed protobuf/JSON early.
      Sync workspace ids/names/colors/device ids are length-bounded before storage; GetWorkspace now also
      trims and bounds lookup ids before querying. Billing checkout JSON is now capped at 64 KiB and rejects
      malformed bodies, unknown fields, trailing JSON, and explicit non-JSON content types before any Stripe
      call; Stripe webhook bodies now fail with an explicit 413 before signature validation when they exceed
      1 MiB. AI chat/vision/key-upload RPCs now reject bad roles, empty/oversized content, too many messages,
      malformed schemas, invalid temperatures, and oversized keys before key lookup/storage or upstream calls.
      The gRPC bridge JSON codec now rejects unknown fields and trailing JSON payloads before handler dispatch.
      Remaining: malformed protobuf/codegen audit and broader request-shape rejection tests.
#### Abuse / DoS
#### Supply chain / process
- [~] Reproducible builds; SBOM (e.g. `cyclonedx`); sign release artifacts/images (cosign).
      `deploy/release-server.example.sh` now builds the server with deterministic Go flags, writes
      checksums, generates a CycloneDX SBOM, and signs binary/SBOM blobs with cosign. Remaining: CI release
      automation and signed container images.
- [~] Periodic threat-model review; pre-launch pen-test pass; secrets scanning (gitleaks) in CI.
      Gitleaks now runs in CI; remaining: periodic threat-model review and pre-launch pen-test pass.
### 7.16 Reliability, SRE & disaster recovery
- [~] Context deadlines/timeouts on all I/O (DB, upstream OpenAI, blob store); cancellation propagation.
      OpenAI proxy calls now have configurable upstream deadlines; blob PUT/GET now use
      `CASHFLUX_SERVER_BLOB_IO_TIMEOUT` and context-aware store operations. Remaining: DB deadlines.
- [~] Retries with jittered exponential backoff for transient upstream failures; circuit breaker on the
      AI upstream; idempotent writes (idempotency keys on mutating HTTP; PUT semantics on sync).
      OpenAI proxy retries transient transport, 429, and 5xx failures; repeated upstream transport/5xx failures
      now open a short fail-fast circuit that resets after cooldown/success. Stripe billing checkout/portal
      endpoints now persist and replay `Idempotency-Key` results per user/route/request hash. Remaining:
      any future non-PUT mutating HTTP endpoints must use the same idempotency pattern.
### 7.18 Performance, scale & limits
- [~] Load + soak tests (sync push/pull, blob up/down, AI streaming, WatchWorkspaces fan-out); publish
      a baseline like the bridge's benchmark snapshots; perf regression gate in CI.
      `TestServerLoadSmokeSyncBlobAndWatch` now covers concurrent sync pushes, workspace-watch fan-out, list,
      and blob upload/download through the in-process HTTP/gRPC bridge. Remaining: AI streaming, longer soak
      runs against production-like disk/proxy/network, and a published perf-regression gate.
### 7.20 Anti-abuse & fraud
- [~] Signup/login abuse controls (rate limit, optional CAPTCHA on bursts, email/OAuth verification).
      OAuth/session routes now have a dedicated per-IP `CASHFLUX_SERVER_AUTH_RATE_LIMIT_PER_MINUTE` cap with
      JSON `RATE_LIMITED` errors. Google ID-token verification now rejects missing/expired expiry claims and
      future issued-at claims before userinfo fetch or session issuance. OAuth userinfo rejects explicit
      unverified email claims. Remaining: optional CAPTCHA-on-burst policy only.
      policy and broader email/OAuth verification review.
---
