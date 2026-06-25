# Granular todo decomposition — batch 12 (research, 2026-06-25)

> Read-only research output. Fold into `TODOS.md` at a checkpoint. No code written.
> Big theme this batch: many clusters are largely ALREADY SHIPPED by the implementer agents.

## R16 recurring & bills (#432 → atomic)
ALREADY SHIPPED: C155 — `bills_screen.go:163` uses `pr.FormatDate(upcoming[0].DueDate)`.
- [ ] [C147][MAJOR] Surface SMART-P1 detection card on bills screen + per-sub "Add to recurring" CTA — `bills_screen.go:100` (collect smart.PagePlanning) + `smartengine/planning.go:204` (action → "/recurring"); thread detected subs as structured payload (`planning.go:174`).
- [ ] [C148][MAJOR] Month prev/next nav — `bills_screen.go` add `calMonth` state + prev/next `UseEvent` (unconditional, ~after l51); pass to `bills.MonthCalendar()` (l215); header chevrons + `pr.FormatMonthYear(calMonth)` (helper at `prefs.go:227`).
- [ ] [C149][MAJOR] Next-due date field — `planning.go` add `rNextDue` state (~after l89) + `<input type=date>` (l584-590); parse into `NextDue` (replace `time.Now()` at l111); i18n `recurring.nextDuePlaceholder`.
- [ ] [C150][MAJOR] Enrich + click-through calendar dots — `bills_screen.go:243-249` — urgency tone + amounts in tooltip; extract `CalDotButton` component (hook inside it, NOT in the MapKeyed loop).
- [ ] [C151][MINOR] Exclude liability payments from subs — `subscriptions_screen.go:81-89` — filter via `subscriptions.IsLiabilityPayment(s, app.Transactions(), app.Accounts())` (`classify.go:51`).
- [ ] [C152][MINOR] Biweekly + semi-monthly cadence — `domain/entities.go:213-233` add consts + `Next()` cases + `MonthlyEquivalent()` (biweekly a*26/12, semi a*2); `planning.go:547-552` options + `cadenceLabel()` (l938-949); tests `domain_test.go`.
- [ ] [C153][MINOR] Inline edit recurring — `planning.go` add `editID` state + Edit btn on `RecurringRow` (hooks inside the component); submit via `PutRecurring` with same ID.
- [ ] [C154][MINOR] Persistent paid/autopay — new `recurring_occurrences` store table + `appstate.MarkOccurrencePaid` (reuse domain `IsPaid`/`MarkPaid` in `occurrence.go`); `Autopay bool` on `domain.Recurring`; paid indicator in `BillRow` (`bills_screen.go:279`).
- [ ] [C156][DESIGN] `/recurring` route — extract `Recurring()` into `internal/screens/recurring_screen.go`; register in `screens.All()` (`screens.go:74`) + shell nav (`shell.go:236-240`); replace planning card with a summary tile.

## R31 plans/pricing (#463 → atomic)
ALREADY SHIPPED: "Manage subscription"→Stripe portal (`settings_section.go:262`); trial note (`settings.cloudTrialNote`); annual/monthly price disclosure (Settings); UpgradeSheet trust line; SubscriptionBanner trial countdown; server-side trial-already-used guard (`billing_http.go:75`).
- [ ] [C301][CRITICAL] Decouple `ShowUpgradeSheet()` from CloudMention (only call site is `cloudmention.go:39`; once dismissed it's unreachable) — add a permanent "Try Cloud →"/Upgrade entry in sidebar (`shell.go`) + queue pending-open if called pre-mount (`upgradesheet.go:19-30`).
- [ ] [C300][MAJOR] Add `/plans` page (new `internal/app/plans.go`) reusing the Settings billing block + `startCheckout`/`openPortal`; "View plans & pricing" link in sidebar/Help; show both annual+monthly in UpgradeSheet (lift interval toggle from `settings.go:681`).
- [ ] [C302][MAJOR] Surface Manage/Cancel from `SubscriptionBanner` directly (deep-link to billing section; canceled-banner → checkout) — `subscriptionbanner.go:110-120`; add "canceling returns you to free local mode" copy.
- [ ] [C303][MAJOR] Plain-English free-vs-paid + trial in UpgradeSheet — add trial line + "Always free vs Cloud" comparison (`upgradesheet.go:37-74`); hint cost in CloudMention body.
- [ ] [C304][DESIGN] Split "Cloud & server" into "Connection" vs "Plan & billing" sub-sections w/ headings — `settings_section.go:194-265`; hint to switch to Cloud to see pricing when self-hosted.
- Gotcha: checkout/portal handlers close over endpoint/token (stale-snapshot risk) — pass as args or read fresh; `fetchBillingStatus` goroutine→UseState setter (confirm goroutine-safe).

## R28 alerts (#450 + #451 → atomic)
ALREADY SHIPPED (close as done): C263 (per-type settings UI `settings.go:95-160`), C264 (thresholds l208-260), C265 (paycheck `notify.go:44`+`notifyrun.go:331`), C266 (low-balance `notify.go:43`+`notifyrun.go:300`), C267 (severity pills `notifications.go:28`), C268 (read/dismiss/snooze `uistate/notifyfeed.go:101-156`), C269 (jump-nav `settingssectionnav.go:29`), C270. All have e2e tests.
- [ ] [#451][MAJOR] Add shared `OnTxnMutated func(*domain.Transaction)` seam on `App` — `appstate.go:69`; call at end of `PutTransaction` (l1554) guarded by `!triggersSuspended`; also fire on delete. (SHARED with #427 R13-reactivity — one field, two consumers.)
- [ ] [#451][MAJOR] New wasm-only `internal/app/livenotify.go` — `wireLiveNotify(app)` sets the hook; `runLiveNotifyFor(t)` runs only large/low-balance/paycheck/budget generators (skip time-based), config-gated via `notify.EnabledRules`, persists delivered log, prepends feed; recover() guard.
- [ ] [C272][MINOR] `runNotifyCatchUp` recover() → also `PostNotice(notify.catchUpError)` (`notifyrun.go:40-45`).
- [ ] [C271][MAJOR] "While you were away" digest grouping — `notifications.go:209-227` split `newSince` vs older into two `role=list` groups w/ headers (data already split at l159).
- [ ] [C268/snooze][MINOR] `pruneSnoozedFeed(now)` in `uistate/notifyfeed.go`; call from livenotify + NotificationCenter effect (l164).

## R29 roles (#462 → atomic)
ALREADY SHIPPED (close as done): C275 (role field in add `memberaddform.go:101-108` + edit `members.go:415-422`), C276 cosmetic badge (`members.go:432-441`), C274 disclosure note (`members.go:231`); full `internal/memberrole` pkg + tests; `domain.Member.Role` (`entities.go:50`); store round-trip; active-member switcher (`memberswitcher.go`).
- [ ] [C273][MAJOR] New `uistate.ActiveMemberRole()` helper (js&wasm) — resolves active member→role, `RoleOwner` when "Everyone".
- [ ] [C273][MAJOR] Gate Add/Delete/Make-default in Members on `CanManageMembers(role)` — `members.go:76-111,446-452` (derive once, pass `canManage` prop; no hook in loop).
- [ ] [C273][MAJOR] Gate write CTAs (Quick-Add/Add-menu/inline edit/delete) when Viewer (`CanViewOnly`) — add `uistate.IsViewerMode()`; wire in quickadd/addmenu/transactions/accounts/budgets/goals (one bool down).
- [ ] [C276][MINOR] Show role label in member switcher + txn member-filter options (`memberswitcher.go:52`, `transactions.go:745`).
- [ ] [C276][DESIGN] "Viewing as Viewer — read-only" banner in shell when CanViewOnly (overlaps C281).
- [ ] [cleanup] Remove orphaned i18n `members.roleMember`/`members.roleDefault`; seed `Role: RoleOwner` explicitly for default member (`sample.go`).
- Gotcha: local-first single-device → enforcement is SOFT UI only (no server auth).

## R33 a11y (#458 → atomic)
ALREADY SHIPPED (close as done): C318 radiogroup/role=radio/roving-tabindex (`ui/controls.go:131-190`) + server-mode/billing Segmented labels; most C315 aria-labels (rail-collapse, mobile +Add, NotifyBell, HelpButton, Muzak, offline, skip link, nav, breadcrumb, chart role=img); C317 `toggleTheme()` palette-wired + `/appearance` screen; C319 `DashboardLayoutControls` exists in Settings.
- [ ] [C315][MAJOR] aria-label on TopBar menu button (`shell.go:734`) + `aria-hidden` on brand "C" span (l502) + aria-label on HouseholdCard settings btn (l688-699).
- [ ] [C315][MINOR] i18n the chart default label `"Trend chart"` → `a11y.trendChart` (`ui/chart.go:56`).
- [ ] [C316][MAJOR] Sample-banner + subscription-banner text contrast — add `tw.TextFg` token to the text Span (`samplebanner.go:61`, `subscriptionbanner.go`).
- [ ] [C317][MAJOR] Visible theme-toggle button in TopBar controls (`shell.go:749-757`) calling `toggleTheme()` w/ Sun/Moon icon + aria-label.
- [ ] [C318][MINOR] Add `Label:` to remaining unlabeled Segmenteds: ResolutionControl (`shell.go:928`), week-start (`settings_section.go:162`), quickadd (`quickadd.go:246`).
- [ ] [C319][DESIGN] aria-label on layout-mode Select (`dashboard.go:1201`) + surface a layout/customize entry on the dashboard itself (not only Settings).

## R26 recommendations (#453 → atomic)
ALREADY SHIPPED (close as done): C256 executable actions (`smart_card.go:70-203`), C258a/b (SU1 same-page scroll, SU9 toast), C259b "enable free only" (`smart.go:277`), C259c per-rule cap (`smart/cap.go`). Settings KV persists across wipe by design.
- [ ] [C254][MAJOR] Verify `Settings{}.IsEnabled(free) == true` (add test); first-run auto-enable free via KV sentinel `cashflux:smart-first-run` in `SmartHub()` (~l39).
- [ ] [C255][MAJOR] Pre-init KV race — gate SmartHub/digest on `appstate.Default != nil` (already l28-29); add native tests for `LoadSmartSettings()` nil-app fallback + browser-store→SQLite migration on next get.
- [ ] [C257][MAJOR] Make /smart a ranked hub: relabel Insights tab "Recommendations" + subtitle; ensure `smart-digest` widget is in the DEFAULT bento layout (`dashboard.go:252` registered — add to default order in widgetcfg); `data-testid` on digest (l1378).
- [ ] [C259][DESIGN] Total cap (~25) before pagination (`smart.go:209`) + "Sorted by urgency" label.
- Gotcha: bulk-enable must bump `DataRevision` (SetSettingKV doesn't); digest widget hardcoded `GridRow 10` won't show unless in default layout list.
