// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// nameSlug returns a stable, lowercase, hyphen-separated slug from a subscription
// name — used in data-testid attributes so selectors are readable and URL-safe.
func nameSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer(" ", "-", "/", "-", ".", "-", "'", "", "\"", "").Replace(s)
}

// SubscriptionsPanelProps holds configuration for SubscriptionsPanel. Currently
// the panel reads all state from appstate.Default; the struct exists so call sites
// pass SubscriptionsPanelProps{} and future props can be added without altering
// existing callers.
type SubscriptionsPanelProps struct{}

// SubscriptionsPanel is a registered component that owns all subscriptions-view
// logic and hooks. It is mounted on the /subscriptions route (via the
// Subscriptions() thin shell) and embedded in the tabbed /recurring hub
// (FEATURE_MAP §5.3/§5.7b). Each mount gets an isolated hook scope so
// tab-switching does not share selection or preferences state.
//
// Subscriptions lists recurring charges detected from transaction history (B25):
// each subscription's cadence, charge, normalized monthly cost, and next renewal,
// plus the total monthly/annual burden. Includes per-row cancel-candidate
// selection with a running "save $X/year" summary, bulk cancel, and a quiet
// "worth reviewing?" badge on subscriptions not seen in 2+ cadence intervals.
func SubscriptionsPanel(p SubscriptionsPanelProps) ui.Node {
	app := appstate.Default
	base := "USD"
	if app != nil {
		if b := app.Settings().BaseCurrency; b != "" {
			base = b
		}
	}

	// === Hooks — all unconditional (GWC rule) ===

	// Drill from a detected subscription to its underlying charges.
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	// Defer the page-level smart-insight scan off the initial mount — it's a
	// full-transaction engine pass that only feeds secondary badges.
	subReady := useAfterSettle("subscriptions")
	viewCharges := func(payee string) {
		f := uistate.TxFilter{Text: payee}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	// viewSubPayments drills to the transactions the user marked as payments toward a
	// subscription (by name) — the proof behind the row's "last paid" line.
	viewSubPayments := func(name string) {
		f := uistate.TxFilter{Subscription: name}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	pr := uistate.UsePrefs().Get()
	notice := uistate.UseNotice()
	// Re-render when the detection preferences change from the flip modal (a
	// separate component tree — its saves bump the data revision).
	_ = uistate.UseDataRevision().Get()

	// --- Cancel-candidates multi-select (L12) ---
	// Session state: map of sub Name → selected. All mutations copy-on-write so
	// UseState detects the change and re-renders.
	selectedState := ui.UseState(map[string]bool{})

	// allSelected and allSubsForSelect are set during rendering and captured by the
	// selectAllToggle closure so the event handler always reads the latest values.
	var allSelected bool
	var allSubsForSelect []subscriptions.Subscription

	selectAllToggle := ui.UseEvent(Prevent(func() {
		if allSelected {
			selectedState.Set(map[string]bool{})
		} else {
			next := make(map[string]bool, len(allSubsForSelect))
			for _, s := range allSubsForSelect {
				next[s.Name] = true
			}
			selectedState.Set(next)
		}
	}))

	// doBulkCancel is self-contained: it re-reads cancelList from app at call time
	// so it can be registered as a stable hook before cancelMap is computed during
	// rendering. notice and selectedState are captured by reference from the hooks
	// above and always reflect the most recent render's values.
	doBulkCancel := func() {
		a := appstate.Default
		if a == nil {
			return
		}
		cancelList := a.Cancellations()
		cancelMap := make(map[string]time.Time, len(cancelList))
		for _, c := range cancelList {
			cancelMap[strings.ToLower(strings.TrimSpace(c.SubName))] = c.CancelledOn
		}
		sel := selectedState.Get()
		now := time.Now()
		for name, isSelected := range sel {
			if !isSelected {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(name))
			if _, alreadyCancelled := cancelMap[key]; alreadyCancelled {
				continue
			}
			if err := a.MarkSubscriptionCancelled(name, now); err != nil {
				notice.Set(notice.Get().With(err.Error(), true))
				return
			}
		}
		selectedState.Set(map[string]bool{})
	}
	bulkCancelEvt := ui.UseEvent(Prevent(doBulkCancel))

	// insightsNav is the cross-link handler wired to the footer "Spending analysis"
	// button; registered unconditionally so hook count is stable.
	insightsNav := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/insights")) })

	// === Rendering (nil-guarded) ===
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// C166: load detection preferences and filter transactions before detection so
	// charges from ignored categories or account types are never considered. This
	// lets users tune out noisy spending (e.g. "Dining" one-offs or investment
	// account transfers) without touching the per-subscription ignore list.
	detectPrefs := uistate.LoadSubsDetectPrefs()
	allTxns := app.Transactions()
	allAccts := app.Accounts()

	// Build accountID → AccountType map for the account-type filter.
	acctTypeByID := make(map[string]string, len(allAccts))
	for _, ac := range allAccts {
		acctTypeByID[ac.ID] = string(ac.Type)
	}

	// Apply detection preferences: exclude transactions from ignored categories
	// or account types before running detection.
	ignoredCatSet := make(map[string]bool, len(detectPrefs.IgnoredCategoryIDs))
	for _, id := range detectPrefs.IgnoredCategoryIDs {
		ignoredCatSet[id] = true
	}
	ignoredAcctTypeSet := make(map[string]bool, len(detectPrefs.IgnoredAccountTypes))
	for _, typ := range detectPrefs.IgnoredAccountTypes {
		ignoredAcctTypeSet[typ] = true
	}
	filteredTxns := allTxns
	if len(ignoredCatSet) > 0 || len(ignoredAcctTypeSet) > 0 {
		filteredTxns = make([]domain.Transaction, 0, len(allTxns))
		for _, t := range allTxns {
			if ignoredCatSet[t.CategoryID] {
				continue
			}
			if ignoredAcctTypeSet[acctTypeByID[t.AccountID]] {
				continue
			}
			filteredTxns = append(filteredTxns, t)
		}
	}

	rawSubs, _ := subscriptions.Detect(filteredTxns, rates, detectPrefs.MinOccurrencesOrDefault()) // C166: user-tunable sensitivity

	// Build a lookup of ignored subscriptions by lower-case name so we can
	// filter them out of the active detected list and show them in a separate
	// "ignored" section with an undo action.
	ignoreList := app.IgnoredSubscriptions()
	ignoreMap := make(map[string]bool, len(ignoreList))
	for _, ig := range ignoreList {
		ignoreMap[strings.ToLower(strings.TrimSpace(ig.SubName))] = true
	}

	// Partition detected subscriptions into active (not ignored) and ignored.
	// C161/C151: drop entries that are really liability payments (loan/credit-card
	// debits, lender-named charges) — they recur like subscriptions but aren't ones,
	// and counting them inflated both the list and the annual total. IsLiabilityPayment
	// checks the debiting account's class/type and lender-phrase labels.
	// A charge the user already models as a planned recurring flow (HOA dues,
	// utilities) is a known bill, not a mystery "subscription" to cancel —
	// listing it here produced nonsense like "How to cancel HOA dues subscription".
	// Cross-check detected names against the recurring labels and drop matches.
	recurringNames := make(map[string]bool)
	for _, r := range app.Recurring() {
		if n := strings.ToLower(strings.TrimSpace(r.Label)); n != "" {
			recurringNames[n] = true
		}
	}

	// QA M5: category/name resolver for the essential-spend check — Electricity,
	// Gas, Cigarettes, Pharmacy recur like subscriptions but are everyday
	// spending; listing them with "How to cancel" inflated the counts and costs.
	catNames := make(map[string]string)
	for _, c := range app.Categories() {
		catNames[c.ID] = c.Name
	}
	catNameOf := func(id string) string { return catNames[id] }

	var subs []subscriptions.Subscription
	var ignoredSubs []subscriptions.Subscription
	for _, s := range rawSubs {
		if subscriptions.IsLiabilityPayment(s, allTxns, allAccts) {
			continue
		}
		if recurringNames[strings.ToLower(strings.TrimSpace(s.Name))] {
			continue // already a planned recurring flow — not a subscription
		}
		if subscriptions.IsEssentialSpend(s, allTxns, catNameOf) {
			continue // ordinary recurring spending (utility/grocery/pharmacy/rent) — not cancellable
		}
		if ignoreMap[strings.ToLower(strings.TrimSpace(s.Name))] {
			ignoredSubs = append(ignoredSubs, s)
		} else {
			subs = append(subs, s)
		}
	}

	// A detected pattern whose renewal is long past isn't an ACTIVE
	// subscription — the 2023 COBRA premium must not lead the list with
	// "next Jun 4, 2023" and Remind-me buttons. Lapsed patterns drop out of
	// the list, the counts, and the money totals, and get their own quiet
	// section at the bottom.
	var lapsedSubs []subscriptions.Subscription
	{
		now := time.Now()
		live := make([]subscriptions.Subscription, 0, len(subs))
		for _, s := range subs {
			if s.Lapsed(now) {
				lapsedSubs = append(lapsedSubs, s)
			} else {
				live = append(live, s)
			}
		}
		subs = live
	}

	// QA R3 CF-03: the price-change list used to bypass every classification
	// filter, so Cigarettes/Gas/Pharmacy "price changes" survived the M5 cleanup.
	// It now shares the exact rule the active list applies (essential spend,
	// liability payments, planned recurring flows all excluded).
	rawChanges, _ := subscriptions.DetectPriceChanges(app.Transactions(), rates, 3)
	changes := rawChanges[:0:0]
	for _, pc := range rawChanges {
		if subscriptions.IsRealSubscriptionName(pc.Name, allTxns, allAccts, catNameOf, recurringNames) {
			changes = append(changes, pc)
		}
	}
	soon := subscriptions.UpcomingRenewals(subs, 7, time.Now())

	// #52: confidence tiers. Confirmed names persist in the settings KV; the
	// assessor grades everything else on detection evidence (charge count,
	// cadence regularity, amount spread). The needs-review tier stays VISIBLE in
	// the list (with its chip and a review inbox below) but never folds into the
	// headline totals — the hero carries an explicit exclusion caption instead —
	// and its names are kept out of the price-change section until resolved.
	confirmedSet := loadConfirmedSubs()
	assess := func(s subscriptions.Subscription) subscriptions.Assessment {
		return subscriptions.Assess(s, confirmedSet)
	}
	var reviewSubs []subscriptions.Subscription
	reviewNames := map[string]bool{}
	countedSubs := subs[:0:0]
	var annual int64
	for _, s := range subs {
		if assess(s).Level == subscriptions.ConfidenceReview {
			reviewSubs = append(reviewSubs, s)
			reviewNames[subscriptions.ConfirmKey(s.Name)] = true
			continue
		}
		countedSubs = append(countedSubs, s)
		annual += s.AnnualAmount()
	}
	changesKept := changes[:0:0]
	for _, pc := range changes {
		if reviewNames[subscriptions.ConfirmKey(pc.Name)] {
			continue
		}
		changesKept = append(changesKept, pc)
	}
	changes = changesKept

	// Build a lookup of cancelled subscriptions by lower-case name.
	cancelList := app.Cancellations()
	cancelMap := make(map[string]time.Time, len(cancelList))
	for _, c := range cancelList {
		cancelMap[strings.ToLower(strings.TrimSpace(c.SubName))] = c.CancelledOn
	}

	// late charges: charges that arrived after a cancellation.
	lateCharges, _ := subscriptions.ChargedAfterCancel(app.Transactions(), cancelList, rates)

	// remind creates a to-do dated to the subscription's next renewal, so a
	// "should I keep this?" task surfaces before the next charge (B25).
	remind := func(s subscriptions.Subscription) {
		app := appstate.Default
		if app == nil {
			return
		}
		// Turn the reminder into a GUIDED cancellation: a step-by-step checklist led by
		// the annual savings — the local stand-in for a "cancel it for you" service, so
		// it actually gets done and a re-charge gets caught (all on-device).
		savingsLine := uistate.T("subs.cancelSaveLine", fmtMoney(money.New(s.AnnualAmount(), base)))
		task := domain.Task{
			ID:       id.New(),
			Title:    uistate.T("subs.reminderTitle", s.Name),
			Notes:    subscriptions.ChecklistNotes(savingsLine, subscriptions.CancelChecklist(s.Name)),
			Status:   domain.StatusOpen,
			Priority: domain.PriorityMedium,
			Due:      s.NextRenewal,
			Source:   domain.SourceNudge,
		}
		if err := app.PutTask(task); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		notice.Set(notice.Get().With(uistate.T("subs.reminderAdded", s.Name), false))
	}

	// guide is the LOCAL "How to cancel" path (QA R3 CF-03: a local-first product
	// shouldn't lead with a DuckDuckGo hop): it files the guided cancellation
	// checklist as a due-today task — savings headline, step-by-step checklist,
	// post-cancellation re-charge check — all on-device. The web search remains
	// as a small secondary link on the row.
	guide := func(s subscriptions.Subscription) {
		app := appstate.Default
		if app == nil {
			return
		}
		savingsLine := uistate.T("subs.cancelSaveLine", fmtMoney(money.New(s.AnnualAmount(), base)))
		task := domain.Task{
			ID:       id.New(),
			Title:    uistate.T("subs.guideTitle", s.Name),
			Notes:    subscriptions.ChecklistNotes(savingsLine, subscriptions.CancelChecklist(s.Name)),
			Status:   domain.StatusOpen,
			Priority: domain.PriorityMedium,
			Due:      time.Now(),
			Source:   domain.SourceNudge,
		}
		if err := app.PutTask(task); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		notice.Set(notice.Get().With(uistate.T("subs.guideAdded", s.Name), false))
	}

	doCancel := func(name string) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.MarkSubscriptionCancelled(name, time.Now()); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		// Success notice triggers a re-render so the row immediately shows its
		// cancelled state (L49: without this the UI stayed stale after cancel).
		notice.Set(notice.Get().With(uistate.T("subs.cancelledConfirm", name), false))
	}
	doUncancel := func(name string) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.UnmarkSubscriptionCancelled(name); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		// Success notice triggers a re-render so the row immediately shows its
		// active state again (L49).
		notice.Set(notice.Get().With(uistate.T("subs.uncancelledConfirm", name), false))
	}

	// #52: confirming a detection promotes it to the Confirmed tier and folds it
	// into the headline totals on the very next render (the notice re-render);
	// BumpDataRevision refreshes the annual report's figures immediately too.
	doConfirm := func(name string) {
		addConfirmedSub(name)
		uistate.BumpDataRevision()
		notice.Set(notice.Get().With(uistate.T("subs.confirmedNotice", name), false))
	}

	doIgnore := func(name string) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.IgnoreSubscription(name); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		// Success notice triggers a re-render so the row immediately disappears from
		// the active list.
		notice.Set(notice.Get().With(uistate.T("subs.ignoredConfirm", name), false))
	}
	doUnignore := func(name string) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.UnignoreSubscription(name); err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		// Success notice triggers a re-render so the subscription reappears in the
		// active list.
		notice.Set(notice.Get().With(uistate.T("subs.unignoredConfirm", name), false))
	}

	// Derive savings and count from current selection for the summary bar.
	sel := selectedState.Get()
	savings := subscriptions.AnnualSavings(subs, sel)
	selectedCount := 0
	for _, v := range sel {
		if v {
			selectedCount++
		}
	}

	// Set variables captured by the hooks registered above so the event handlers
	// read the correct render-time values when they fire.
	allSelected = selectedCount == len(subs) && len(subs) > 0
	allSubsForSelect = subs

	toggle := func(name string) {
		cur := selectedState.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[name] = !next[name]
		selectedState.Set(next)
	}

	// Build the savings summary bar (only shown when ≥1 subscription is selected).
	// Uses the pre-registered bulkCancelEvt hook instead of an inline UseEvent
	// call so the hook count stays stable regardless of selectedCount (GWC rule).
	var savingsSummary ui.Node
	if selectedCount > 0 {
		savingsLabel := uistate.T("subs.cancelSavings", selectedCount, fmtMoney(money.New(savings, base)))
		if selectedCount > 1 {
			savingsLabel = uistate.T("subs.cancelSavingsMany", selectedCount, fmtMoney(money.New(savings, base)))
		}
		savingsSummary = Div(
			css.Class(tw.Fold(tw.Flex, tw.FlexWrap, tw.ItemsCenter, tw.Gap2, tw.Py1)+" savings-summary"),
			Attr("data-testid", "subs-cancel-savings"),
			Attr("role", "status"),
			Attr("aria-live", "polite"),
			Span(css.Class(tw.Fold(tw.FontMedium, tw.Text14)), savingsLabel),
			Button(
				css.Class("btn btn-danger"),
				Type("button"),
				Title(uistate.T("subs.cancelSelectedTitle")),
				Attr("aria-label", uistate.T("subs.cancelSelectedTitle")),
				Attr("data-testid", "subs-bulk-cancel-btn"),
				OnClick(bulkCancelEvt),
				uistate.T("subs.cancelSelected"),
			),
		)
	} else {
		savingsSummary = Fragment()
	}

	// Monthly total drives the per-row share-bar width (G10 follow-up).
	monthlyTotal := subscriptions.MonthlyTotal(subs)

	now := time.Now()

	// Compute page-level smart insights once (not per row). Current subscription
	// engines do not set RelatedID (they use subscription names), so byEntity is
	// empty and badges are silent today — the wiring is forward-compatible with
	// future engines that do set RelatedID = subscription.Name or a real ID.
	subSmartSettings := uistate.LoadSmartSettings()
	subSmartIn := buildSmartInput(app, pr.WeekStartWeekday())
	var subInsights []smart.Insight
	if subReady {
		subInsights = smartengine.RunPage(subSmartIn, subSmartSettings, smart.PageSubscriptions)
	}
	subByEntity := insightsByEntity(subInsights)

	// C162: the "Renewing soon" section reuses the same SubscriptionRow, so any sub
	// renewing within 7 days otherwise appeared twice. Render the main list as the
	// subs NOT shown in that section (totals/CSV/savings still use the full `subs`).
	soonSet := make(map[string]bool, len(soon))
	for _, s := range soon {
		soonSet[s.Name+"|"+fmt.Sprint(s.Amount)] = true
	}
	mainSubs := subs[:0:0]
	for _, s := range subs {
		if !soonSet[s.Name+"|"+fmt.Sprint(s.Amount)] {
			mainSubs = append(mainSubs, s)
		}
	}

	// Above the fold on mount: cap the rendered rows; the rest render after first
	// paint (subReady) so a long subscription list paints immediately.
	displayMain := mainSubs
	if !subReady && len(mainSubs) > 6 {
		displayMain = mainSubs[:6]
	}
	rows := MapKeyed(displayMain,
		func(s subscriptions.Subscription) any { return s.Name + "|" + fmt.Sprint(s.Amount) },
		func(s subscriptions.Subscription) ui.Node {
			cancelledOn, isCancelled := cancelMap[strings.ToLower(strings.TrimSpace(s.Name))]
			cancelledDate := ""
			if isCancelled {
				cancelledDate = pr.FormatDate(cancelledOn)
			}
			return ui.CreateElement(SubscriptionRow, subscriptionRowProps{
				Sub:            s,
				Base:           base,
				NextDate:       pr.FormatDate(s.NextRenewal),
				Cancelled:      isCancelled,
				CancelledOn:    cancelledDate,
				Selected:       sel[s.Name],
				NeedsReview:    !isCancelled && subscriptions.NeedsReview(s, now),
				Conf:           assess(s),
				MonthlyTotal:   monthlyTotal,
				OnRemind:       remind,
				OnGuide:        guide,
				OnDrill:        viewCharges,
				OnCancel:       doCancel,
				OnUncancel:     doUncancel,
				OnToggleSelect: toggle,
				OnIgnore:       doIgnore,
				SmartSettings:  subSmartSettings,
				SmartByEntity:  subByEntity,
				SubPayment:     ledger.SubscriptionPaymentForName(s.Name, allTxns),
				OnViewPayments: viewSubPayments,
			})
		},
	)

	var body ui.Node
	if len(subs) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("subs.empty"), CTALabel: uistate.T("subs.addFirst"), Href: "/transactions"})
	} else {
		selectAllLabel := uistate.T("subs.selectAll")
		if allSelected {
			selectAllLabel = uistate.T("subs.clearSelection")
		}
		body = Fragment(
			Div(css.Class(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)+" subs-select-all-bar"),
				Button(
					css.Class("btn btn-sm"),
					Type("button"),
					Attr("aria-label", selectAllLabel),
					Attr("data-testid", "subs-select-all-btn"),
					OnClick(selectAllToggle),
					selectAllLabel,
				),
				If(selectedCount > 0,
					Span(css.Class("row-meta"),
						uistate.T("subs.selectedCount", selectedCount),
					),
				),
			),
			Div(css.Class("rows"), rows),
		)
	}

	// Net price-change summary (G10 §5): sum all deltas to give an instant
	// "your subscriptions cost $X more/less per month" headline above the rows.
	var netChangeDelta int64
	for _, c := range changes {
		netChangeDelta += c.Delta
	}

	// Price-change rows have no per-row interactive elements, so they render
	// inline (no component needed). DetectPriceChanges already sorts them
	// most-recent-first.
	changeRows := MapKeyed(changes,
		func(c subscriptions.PriceChange) any { return c.Name + "|" + fmt.Sprint(c.NewAmount) },
		func(c subscriptions.PriceChange) ui.Node {
			pct := c.PercentChange
			if pct < 0 {
				pct = -pct
			}
			delta := fmtMoney(money.New(c.Delta, base).Abs())
			pctStr := fmt.Sprintf("%d%%", pct)
			date := pr.FormatDate(c.ChangedAt)
			// A price increase is worse (red, up arrow); a decrease is better
			// (green, down arrow) — color-plus-shape, matching Reports (C56/C46).
			key, tone, arrow := "subs.priceDown", "text-up", icon.ArrowDown
			if c.Increased() {
				key, tone, arrow = "subs.priceUp", "text-down", icon.ArrowUp
			}
			return Div(css.Class("row"),
				Div(css.Class("row-main"),
					Span(css.Class("row-desc"), c.Name),
					Span(ClassStr("row-meta "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)+" "+tw.ColorClass(tone)),
						uiw.Icon(arrow, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
						Text(uistate.T(key, delta, pctStr, date))),
				),
				Span(css.Class("budget-amount"), fmtMoney(money.New(c.NewAmount, base))),
			)
		},
	)

	// Subscriptions as a share of a TYPICAL month's spending — a "how much of
	// my outflow is recurring?" gauge. Month-to-date is an unfair denominator
	// early in the month (on the 5th the ratio read 233%), so compare against
	// the average of the last three full months instead.
	shareStat := Fragment()
	var trailingExpense int64
	trailingMonths := 0
	for k := 1; k <= 3; k++ {
		ms, me := dateutil.MonthRange(time.Now().AddDate(0, -k, 0))
		if _, expense, err := ledger.PeriodTotals(app.Transactions(), ms, me, rates); err == nil && expense.Amount > 0 {
			trailingExpense += expense.Amount
			trailingMonths++
		}
	}
	if trailingMonths > 0 {
		pct := subscriptions.MonthlyTotal(countedSubs) * 100 * int64(trailingMonths) / trailingExpense
		shareStat = stat(uistate.T("subs.shareOfSpending"), fmt.Sprintf("%d%%", pct), "")
	}

	// Build the charged-after-cancel alert section. Each late charge gets its
	// own plain-English line in a danger-toned banner at the very top.
	lateChargeRows := MapKeyed(lateCharges,
		func(lc subscriptions.LateCharge) any {
			return lc.SubName + "|" + fmt.Sprint(lc.ChargeDate.Unix())
		},
		func(lc subscriptions.LateCharge) ui.Node {
			return P(
				css.Class("row-desc"),
				Text(uistate.T("subs.lateCharge",
					lc.SubName,
					pr.FormatDate(lc.CancelledOn),
					fmtMoney(money.New(lc.Amount, base)),
					pr.FormatDate(lc.ChargeDate),
				)),
			)
		},
	)

	// C166: the detection preferences live in a flip modal (SubsPrefsHost); the tab
	// keeps a compact trigger button whose label carries the active-filter count.
	activeFilterCount := len(detectPrefs.IgnoredCategoryIDs) + len(detectPrefs.IgnoredAccountTypes)
	prefsToggleLabel := uistate.T("subs.detectPrefsShow")
	if activeFilterCount > 0 {
		badge := uistate.T("subs.detectActiveFilters", activeFilterCount)
		if activeFilterCount > 1 {
			badge = uistate.T("subs.detectActiveFiltersMany", activeFilterCount)
		}
		prefsToggleLabel += " (" + badge + ")"
	}
	detectPrefsPanel := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			ui.CreateElement(subsPrefsButton, subsPrefsButtonProps{Label: prefsToggleLabel}),
		),
	)

	// The subscriptions tab shares the recurring surface chrome: a bento host with
	// the monthly-burden hero + chips, then each section as a tile.
	subsHero := Div(css.Class("rec-hero"),
		Div(css.Class("rec-hero-main"),
			Div(css.Class("rec-hero-label "+tw.Fold(tw.TextDim, tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
				uistate.T("subs.monthlyBurden"),
				smartTooltipFor(subSmartSettings, "subs-monthly", uistate.T("subs.monthlyBurden"), uistate.T("smart.tipSubsMonthly")),
			),
			Div(ClassStr("rec-hero-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass("text-down")), fmtMoney(money.New(subscriptions.MonthlyTotal(countedSubs), base))),
		),
		Div(css.Class("debt-chips"),
			recurStatChip(uistate.T("subs.annualBurden"), fmtMoney(money.New(annual, base)), ""),
			recurStatChip(uistate.T("subs.count"), fmt.Sprintf("%d", len(countedSubs)), ""),
			shareStat,
		),
		// #52 headline honesty: needs-review detections are excluded from every
		// figure above, and the hero says so rather than silently folding them in.
		subsConfExcludedNote(len(reviewSubs)),
	)

	return Div(css.Class("bento bento-recurring"),
		If(len(lateCharges) > 0, recurTile("subs-late", Section(
			css.Class("card"),
			Attr("role", "alert"),
			Attr("aria-live", "polite"),
			Style(map[string]string{"border-left": "4px solid var(--color-danger, #ef4444)"}),
			H2(css.Class("card-title "+tw.ColorClass("text-down")),
				uistate.T("subs.lateChargesTitle")),
			Div(css.Class("rows rec-cardrows"), lateChargeRows),
		))),
		recurTile("subs-prefs", detectPrefsPanel),
		If(len(subs) > 0, recurTile("subs-hero", subsHero)),
		recurTile("subs-list", recurSection("sec-subs", uistate.T("nav.subscriptions"),
			// CSV export and smart section action share the header (G10 §7).
			Fragment(
				smartSectionAction(subSmartSettings),
				If(len(subs) > 0,
					Button(css.Class("btn btn-sm"), Type("button"), Title(uistate.T("subs.downloadCsvTitle")),
						OnClick(func() {
							csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
							downloadBytes("subscriptions.csv", "text/csv", subscriptions.CSV(subs, csvAmount))
						}), uistate.T("subs.downloadCsv")),
				),
			),
			Fragment(
				body,
				savingsSummary,
			),
		)),
		// #52: the review inbox — every needs-review detection with its evidence
		// and one-click Confirm / Not-a-subscription resolutions. Confirming folds
		// it into the totals immediately; rejecting feeds the persisted ignore
		// list, so future detection runs skip it everywhere.
		If(len(reviewSubs) > 0, recurTile("subs-review", recurSection("sec-subs-review", uistate.T("subs.reviewInboxTitle"), nil,
			SubsReviewInbox(subsReviewInboxProps{
				Subs: reviewSubs, Assess: assess, Base: base,
				OnConfirm: doConfirm, OnReject: doIgnore,
			}),
		))),
		If(len(changes) > 0, recurTile("subs-changes", recurSection("sec-subs-changes", uistate.T("subs.priceChangesTitle"), nil,
			Fragment(
				// Net summary line (G10 §5): "Your subscriptions cost $X more/less
				// per month than they did recently" — instant context before the
				// per-row detail.
				netChangeSummary(netChangeDelta, base),
				Div(css.Class("rows rec-cardrows"), changeRows),
			),
		))),
		If(len(soon) > 0, recurTile("subs-soon", recurSection("sec-subs-soon", uistate.T("subs.renewingSoon"), nil,
			// Renewing-soon rows reuse the full SubscriptionRow so each imminent
			// renewal is actionable in place (remind / cancel) — not a stripped
			// read-only card (C56). The cancelledOn lookup, selection state, and
			// all callbacks are wired identically to the main list.
			Div(css.Class("rows rec-cardrows"), MapKeyed(soon,
				func(s subscriptions.Subscription) any { return "soon|" + s.Name + "|" + fmt.Sprint(s.Amount) },
				func(s subscriptions.Subscription) ui.Node {
					cancelledOn, isCancelled := cancelMap[strings.ToLower(strings.TrimSpace(s.Name))]
					cancelledDate := ""
					if isCancelled {
						cancelledDate = pr.FormatDate(cancelledOn)
					}
					return ui.CreateElement(SubscriptionRow, subscriptionRowProps{
						Sub:            s,
						Base:           base,
						NextDate:       pr.FormatDate(s.NextRenewal),
						Cancelled:      isCancelled,
						CancelledOn:    cancelledDate,
						Selected:       sel[s.Name],
						NeedsReview:    false, // renewing soon ≠ stale
						Conf:           assess(s),
						MonthlyTotal:   monthlyTotal,
						OnRemind:       remind,
						OnGuide:        guide,
						OnDrill:        viewCharges,
						OnCancel:       doCancel,
						OnUncancel:     doUncancel,
						OnToggleSelect: toggle,
						OnIgnore:       doIgnore,
						SmartSettings:  subSmartSettings,
						SmartByEntity:  subByEntity,
						SubPayment:     ledger.SubscriptionPaymentForName(s.Name, allTxns),
						OnViewPayments: viewSubPayments,
					})
				},
			)),
		))),
		If(len(ignoredSubs) > 0, recurTile("subs-ignored", recurSection("sec-subs-ignored", uistate.T("subs.ignoredTitle"), nil,
			Fragment(
				P(css.Class("row-meta"), uistate.T("subs.ignoredDesc")),
				Div(css.Class("rows rec-cardrows"), MapKeyed(ignoredSubs,
					func(s subscriptions.Subscription) any { return "ignored|" + s.Name },
					func(s subscriptions.Subscription) ui.Node {
						return ui.CreateElement(IgnoredSubscriptionRow, ignoredSubRowProps{
							Sub:        s,
							Base:       base,
							OnUnignore: doUnignore,
						})
					},
				)),
			),
		))),
		// Lapsed patterns: recognized as past subscriptions, kept out of the live
		// list/totals, shown quietly for the record (e.g. the layoff-era COBRA).
		If(len(lapsedSubs) > 0, recurTile("subs-lapsed", recurSection("sec-subs-lapsed", uistate.T("subs.lapsedTitle"), nil,
			Fragment(
				P(css.Class("row-meta"), uistate.T("subs.lapsedDesc")),
				Div(css.Class("rows rec-cardrows"), MapKeyed(lapsedSubs,
					func(s subscriptions.Subscription) any { return "lapsed|" + s.Name },
					func(s subscriptions.Subscription) ui.Node {
						return Div(css.Class("row"),
							Span(css.Class("row-desc", tw.Truncate), s.Name),
							Span(css.Class("row-meta"), uistate.T("subs.lapsedLast", pr.FormatDate(s.Last))),
						)
					},
				)),
			),
		))),
		// C253: cross-link to /insights so users know where to find the broader
		// per-category spending anomaly highlights (which live on the Insights screen
		// under "Spending highlights"). Reduces fragmentation: /subscriptions covers
		// the recurring-charge view; /insights covers the per-category trend view.
		recurTile("subs-links", P(css.Class("muted "+tw.Fold(tw.Text12)),
			Text(uistate.T("subs.seeSpendingAnalysis")+" · "),
			Button(ClassStr("btn-link "+tw.Fold(tw.Text12)),
				Type("button"),
				Title(uistate.T("subs.seeSpendingAnalysisTitle")),
				Attr("data-testid", "subs-see-insights-link"),
				OnClick(insightsNav),
				uistate.T("nav.insights"),
			),
		)),
	)
}

// Subscriptions is the /subscriptions route — a thin shell that renders
// SubscriptionsPanel. The shell provides the heading and subtitle from the route
// registry; SubscriptionsPanel owns all content, hooks, and logic
// (FEATURE_MAP §5.3/§5.7b).
func Subscriptions() ui.Node {
	return ui.CreateElement(SubscriptionsPanel, SubscriptionsPanelProps{})
}

// subscriptionRowProps holds the props for a single subscription row.
type subscriptionRowProps struct {
	Sub         subscriptions.Subscription
	Base        string
	NextDate    string // pre-formatted next-renewal date
	Cancelled   bool
	CancelledOn string // pre-formatted cancellation date, set when Cancelled is true
	Selected    bool   // whether the row's cancel-candidate checkbox is checked
	NeedsReview bool   // whether the subscription hasn't been charged in 2+ cadence intervals
	// Conf is the detection's confidence assessment (#52): tier + the
	// plain-English evidence rendered as a chip with the WHY in its tooltip.
	Conf           subscriptions.Assessment
	MonthlyTotal   int64 // sum of all active subscriptions' monthly amounts; drives the per-row share-bar
	OnRemind       func(subscriptions.Subscription)
	OnGuide        func(subscriptions.Subscription) // file the local how-to-cancel checklist (QA R3 CF-03)
	OnDrill        func(payee string)               // open Transactions searched for this subscription's payee
	OnCancel       func(name string)
	OnUncancel     func(name string)
	OnToggleSelect func(name string) // toggle cancel-candidate selection for this row
	OnIgnore       func(name string) // mark as "not a subscription"; nil = action not available (ignored rows)
	// SubPayment is this subscription's payment-check: the most recent transaction the
	// user marked as a payment toward it (by name). OnViewPayments drills to those
	// linked payments. HasAny false = no linked payment yet (the line is hidden).
	SubPayment     ledger.SubscriptionPaymentInfo
	OnViewPayments func(name string)
	// Smart badge inputs: SmartSettings + byEntity index from the page's insight run.
	// Current subscription engines do not set RelatedID (they use subscription names
	// as keys), so byEntity will be empty and badges won't appear until future engines
	// add RelatedID support. The wiring is forward-compatible.
	SmartSettings smart.Settings
	SmartByEntity map[string][]smart.Insight
}

// SubscriptionRow renders one detected subscription with cancel/uncancel and
// "remind me to cancel" actions. It owns all click hooks (per the
// On*-hooks-in-loops rule), so the list renders many rows without reordering
// hooks. A cancelled subscription shows its cancel date and an Undo action
// instead of the standard remind button.
//
// When NeedsReview is true a quiet "worth reviewing?" badge appears on the row —
// a low-pressure cue that the subscription hasn't been charged recently.
//
// The cancel-candidate checkbox (data-testid="sub-cancel-select-<slug>") lets the
// user build up a selection; the parent renders the savings summary + bulk action.
func SubscriptionRow(props subscriptionRowProps) ui.Node {
	s := props.Sub
	remind := ui.UseEvent(Prevent(func() { props.OnRemind(s) }))
	guideEvt := ui.UseEvent(Prevent(func() {
		if props.OnGuide != nil {
			props.OnGuide(s)
		}
	}))
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Name)
		}
	}))
	cancel := ui.UseEvent(Prevent(func() {
		if props.OnCancel != nil {
			props.OnCancel(s.Name)
		}
	}))
	uncancel := ui.UseEvent(Prevent(func() {
		if props.OnUncancel != nil {
			props.OnUncancel(s.Name)
		}
	}))
	ignore := ui.UseEvent(Prevent(func() {
		if props.OnIgnore != nil {
			props.OnIgnore(s.Name)
		}
	}))
	toggleSelect := ui.UseEvent(Prevent(func() {
		if props.OnToggleSelect != nil {
			props.OnToggleSelect(s.Name)
		}
	}))
	viewPayments := ui.UseEvent(Prevent(func() {
		if props.OnViewPayments != nil {
			props.OnViewPayments(s.Name)
		}
	}))

	slug := nameSlug(s.Name)

	// Payment-check line: the most recent transaction the user linked to this
	// subscription (proof it was actually paid), with a drill to those payments.
	var payNode ui.Node = Fragment()
	if props.SubPayment.HasAny {
		payNode = Span(css.Class("row-meta"), Attr("data-testid", "sub-pay-"+slug),
			uistate.T("subs.paymentMeta", fmtMoney(props.SubPayment.Latest)),
			Button(css.Class("btn-link", tw.Ml1), Type("button"), Attr("data-testid", "sub-pay-link-"+slug),
				Title(uistate.T("subs.paymentLinkTitle")), OnClick(viewPayments),
				uistate.T("subs.paymentCount", plural(props.SubPayment.Count, "payment"))))
	}
	meta := subscriptionCadenceLabel(s.Cadence) + " · " + uistate.T("subs.next", props.NextDate)

	// Quiet "worth reviewing?" nudge — only shown for non-cancelled rows where the
	// last charge is suspiciously old (2+ cadence intervals). Not shown for
	// cancelled subs (the user already knows they cancelled it).
	// #52: detection-confidence chip — tier label with the concrete evidence in
	// its tooltip/aria. Hidden on cancelled rows (their state chip already leads).
	confChip := Fragment()
	if !props.Cancelled && props.Conf.Level != "" {
		confChip = subConfidenceChip(props.Conf, slug)
	}

	reviewBadge := Fragment()
	if props.NeedsReview {
		reviewBadge = Span(
			css.Class(tw.Fold(tw.TextXs, tw.FontMedium)+" review-nudge"),
			Attr("title", uistate.T("subs.needsReviewTitle")),
			uistate.T("subs.needsReview"),
		)
	}

	var statusArea ui.Node
	if props.Cancelled {
		statusArea = Span(
			css.Class(tw.ColorClass("text-down")+" "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
			Text(uistate.T("subs.cancelledState", props.CancelledOn)),
		)
	} else {
		statusArea = Fragment()
	}

	var actions ui.Node
	if props.Cancelled {
		actions = Button(
			css.Class("btn"),
			Type("button"),
			Title(uistate.T("subs.uncancelTitle")),
			Attr("aria-label", uistate.T("subs.uncancelTitle")+" "+s.Name),
			OnClick(uncancel),
			uistate.T("subs.uncancel"),
		)
	} else {
		// Actions live in a fixed-width trailing group (G10): keeps the row's name
		// from being squeezed to nothing, and the destructive "Cancel" is a compact
		// ghost-danger button so the list reads as subscriptions, not 10 cancel alerts.
		actions = Div(css.Class("sub-actions"),
			Button(css.Class("btn btn-sm"), Type("button"), Title(uistate.T("subs.remindTitle")), OnClick(remind), uistate.T("subs.remind")),
			Button(
				css.Class("btn btn-sm btn-ghost-danger"),
				Type("button"),
				Title(uistate.T("subs.cancelTitle")),
				Attr("aria-label", uistate.T("subs.cancelTitle")+" "+s.Name),
				OnClick(cancel),
				uistate.T("subs.cancel"),
			),
			// QA R3 CF-03: "How to cancel" now leads with LOCAL guidance — it files
			// the step-by-step cancellation checklist as a due-today task — and the
			// external web search demotes to a small secondary link (local-first:
			// nothing leaves the device unless that link is deliberately clicked).
			Button(
				css.Class("btn btn-sm"),
				Type("button"),
				Attr("data-testid", "sub-howto-cancel-"+slug),
				Title(uistate.T("subs.guideBtnTitle")),
				Attr("aria-label", uistate.T("subs.guideBtnTitle")+" "+s.Name),
				OnClick(guideEvt),
				uistate.T("subs.howToCancel"),
			),
			A(
				css.Class("btn-link", tw.Text12),
				Attr("href", "https://duckduckgo.com/?q="+url.QueryEscape("how to cancel "+s.Name+" subscription")),
				Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
				Attr("data-testid", "sub-cancel-web-"+slug),
				Title(uistate.T("subs.howToCancelTitle")),
				Attr("aria-label", uistate.T("subs.howToCancelTitle")+" "+s.Name),
				uistate.T("subs.webSearch"),
			),
			If(props.OnIgnore != nil, Button(
				css.Class("btn btn-sm"),
				Type("button"),
				Title(uistate.T("subs.ignoreTitle")),
				Attr("aria-label", uistate.T("subs.ignoreTitle")+" "+s.Name),
				Attr("data-testid", "sub-ignore-"+slug),
				OnClick(ignore),
				uistate.T("subs.ignore"),
			)),
		)
	}

	// Cancel-candidate checkbox. Not shown for already-cancelled subs (they are
	// already tracked in the cancellations store; selecting them would be a no-op).
	var selectCheckbox ui.Node
	if !props.Cancelled {
		selectCheckbox = Input(
			Type("checkbox"),
			Attr("aria-label", uistate.T("subs.selectCancel")+" "+s.Name),
			Attr("data-testid", "sub-cancel-select-"+slug),
			Checked(props.Selected),
			OnClick(toggleSelect),
		)
	} else {
		selectCheckbox = Fragment()
	}

	// For non-monthly subscriptions, build the cadence badge and normalized
	// monthly average. The actual charge comes first (C56 fix, G10 §4):
	// "$540.00 [YEARLY] · avg $45/mo" reads clearly — the big number leads, the
	// badge explains why it appears after the $55 row, and the /mo average gives
	// the mental-model anchor without confusing the sort order (G10 §6).
	var cadenceBadge ui.Node
	var perMonthNote ui.Node
	if s.Cadence != subscriptions.CadenceMonthly {
		cadenceKey := "subs.yearly"
		if s.Cadence == subscriptions.CadenceWeekly {
			cadenceKey = "subs.weekly"
		}
		cadenceBadge = Span(
			css.Class(tw.Fold(tw.TextXs, tw.FontMedium)+" cadence-badge"),
			Style(map[string]string{
				"display":        "inline-block",
				"padding":        "0.1em 0.4em",
				"border-radius":  "4px",
				"border":         "1px solid var(--border)",
				"color":          "var(--text-dim)",
				"text-transform": "uppercase",
				"letter-spacing": "0.04em",
				"font-size":      "0.68rem",
				"vertical-align": "middle",
			}),
			uistate.T(cadenceKey),
		)
		perMonthNote = Span(css.Class("row-meta"),
			uistate.T("subs.perMonth", fmtMoney(money.New(s.MonthlyAmount(), props.Base))))
	}

	return Div(css.Class("row sub-row"),
		selectCheckbox,
		Div(css.Class("row-main"),
			Button(css.Class("row-desc sub-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drill),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "font-weight": "600", "color": "var(--text)", "text-align": "left", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
				s.Name,
				smartBadgeFor(props.SmartSettings, props.SmartByEntity, s.Name),
			),
			Span(css.Class("row-meta"), meta),
			payNode,
			statusArea,
			confChip,
			reviewBadge,
			subShareBar(s.MonthlyAmount(), props.MonthlyTotal),
		),
		// Actual charge amount leads (G10 §4/§6): the charge the user sees on
		// their bank statement is the primary figure; cadence badge clarifies
		// why a $540 yearly charge appears after a $55 monthly one.
		Span(css.Class("budget-amount"), fmtMoney(money.New(s.Amount, props.Base))),
		cadenceBadge,
		perMonthNote,
		actions,
	)
}

// subShareBar renders a thin proportional bar showing this subscription's
// share of the total monthly recurring burden (G10 follow-up). Width equals
// MonthlyAmount / MonthlyTotal × 100%, capped at 100%. Returns an empty
// fragment when the total is zero to avoid division-by-zero and visual noise
// in edge cases (e.g. all subscriptions are free trials at $0).
func subShareBar(monthly, total int64) ui.Node {
	if total <= 0 || monthly <= 0 {
		return Fragment()
	}
	pct := int(monthly * 100 / total)
	if pct > 100 {
		pct = 100
	}
	return Div(
		css.Class("share-bar"),
		Style(map[string]string{
			"height":        "4px",
			"max-width":     "180px",
			"margin-top":    "0.3rem",
			"background":    "var(--border)",
			"border-radius": "999px",
			"overflow":      "hidden",
		}),
		Div(Style(map[string]string{
			"height":        "100%",
			"width":         fmt.Sprintf("%d%%", pct),
			"background":    "var(--accent)",
			"border-radius": "999px",
		})),
	)
}

// netChangeSummary renders the one-line net price-change headline for the price-changes
// card (G10 §5). A positive delta (net price rise) renders in the danger tone; a
// negative delta (net savings) renders in the success tone. Returns an empty fragment
// when delta is zero or when there are no changes.
func netChangeSummary(netDelta int64, base string) ui.Node {
	if netDelta == 0 {
		return Fragment()
	}
	abs := netDelta
	if abs < 0 {
		abs = -abs
	}
	amt := fmtMoney(money.New(abs, base))
	key := "subs.netPriceUp"
	tone := "text-down"
	if netDelta < 0 {
		key = "subs.netPriceDown"
		tone = "text-up"
	}
	return P(
		ClassStr("row-meta "+tw.ColorClass(tone)),
		uistate.T(key, amt),
	)
}

// ignoredSubRowProps holds the props for one row in the "not a subscription" section.
type ignoredSubRowProps struct {
	Sub        subscriptions.Subscription
	Base       string
	OnUnignore func(name string) // restore to the active detected list
}

// IgnoredSubscriptionRow renders one subscription that the user has marked as
// "not a subscription". It owns its own click hook (per the On*-hooks-in-loops
// rule) and shows the charge amount plus an "Undo" button to restore it.
func IgnoredSubscriptionRow(props ignoredSubRowProps) ui.Node {
	s := props.Sub
	unignore := ui.UseEvent(Prevent(func() {
		if props.OnUnignore != nil {
			props.OnUnignore(s.Name)
		}
	}))
	slug := nameSlug(s.Name)
	return Div(css.Class("row sub-row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), s.Name),
			Span(css.Class("row-meta"), uistate.T("subs.ignoredState")),
		),
		Span(css.Class("budget-amount"), fmtMoney(money.New(s.Amount, props.Base))),
		Button(
			css.Class("btn btn-sm"),
			Type("button"),
			Title(uistate.T("subs.unignoreTitle")),
			Attr("aria-label", uistate.T("subs.unignoreTitle")+" "+s.Name),
			Attr("data-testid", "sub-unignore-"+slug),
			OnClick(unignore),
			uistate.T("subs.unignore"),
		),
	)
}

// subscriptionCadenceLabel renders a detected cadence as a friendly label.
func subscriptionCadenceLabel(c subscriptions.Cadence) string {
	switch c {
	case subscriptions.CadenceWeekly:
		return uistate.T("subs.weekly")
	case subscriptions.CadenceYearly:
		return uistate.T("subs.yearly")
	default:
		return uistate.T("subs.monthly")
	}
}

// ---------------------------------------------------------------------------
// C166: Detection preference checkbox row components
// ---------------------------------------------------------------------------

// subsDetectCatRowProps holds the props for one category filter checkbox.
type subsDetectCatRowProps struct {
	CatID   string // category ID
	Label   string // display name
	Ignored bool   // whether this category is currently in the ignore list
}

// SubsDetectCatRow renders one category checkbox in the "Detection preferences"
// panel. Each row is its own component so its OnChange hook sits at a stable
// render position regardless of how many categories exist (variable-length loop
// rule). Toggling saves immediately via SaveSubsDetectPrefs, which bumps the
// store mutation revision and re-renders the screen.
func SubsDetectCatRow(props subsDetectCatRowProps) ui.Node {
	id := props.CatID
	onChange := ui.UseEvent(func(e ui.Event) {
		p := uistate.LoadSubsDetectPrefs()
		uistate.SaveSubsDetectPrefs(p.WithCategoryToggled(id))
		uistate.BumpDataRevision() // the panel behind the modal recomputes live
	})
	return Label(
		css.Class(tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)+" detect-pref-label"),
		Style(map[string]string{
			"padding":       "0.25rem 0.5rem",
			"border-radius": "4px",
			"border":        "1px solid var(--border)",
			"font-size":     "0.8rem",
			"cursor":        "pointer",
			"user-select":   "none",
		}),
		Attr("data-testid", "subs-detect-cat-"+id),
		Input(
			Type("checkbox"),
			Attr("aria-label", props.Label),
			CheckedIf(props.Ignored),
			OnChange(onChange),
		),
		Text(props.Label),
	)
}

// subsDetectAcctTypeRowProps holds the props for one account-type filter checkbox.
type subsDetectAcctTypeRowProps struct {
	AcctType string // account type string (e.g. "checking")
	Label    string // display label (from i18n)
	Ignored  bool   // whether this type is currently in the ignore list
}

// SubsDetectAcctTypeRow renders one account-type checkbox in the "Detection
// preferences" panel. Each row is its own component for hook-stability (same
// rationale as SubsDetectCatRow). Toggling saves immediately.
func SubsDetectAcctTypeRow(props subsDetectAcctTypeRowProps) ui.Node {
	typ := props.AcctType
	onChange := ui.UseEvent(func(e ui.Event) {
		p := uistate.LoadSubsDetectPrefs()
		uistate.SaveSubsDetectPrefs(p.WithAccountTypeToggled(typ))
		uistate.BumpDataRevision() // the panel behind the modal recomputes live
	})
	return Label(
		css.Class(tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)+" detect-pref-label"),
		Style(map[string]string{
			"padding":       "0.25rem 0.5rem",
			"border-radius": "4px",
			"border":        "1px solid var(--border)",
			"font-size":     "0.8rem",
			"cursor":        "pointer",
			"user-select":   "none",
		}),
		Attr("data-testid", "subs-detect-accttype-"+typ),
		Input(
			Type("checkbox"),
			Attr("aria-label", props.Label),
			CheckedIf(props.Ignored),
			OnChange(onChange),
		),
		Text(props.Label),
	)
}
