//go:build js && wasm

package screens

import (
	"fmt"
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
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// nameSlug returns a stable, lowercase, hyphen-separated slug from a subscription
// name — used in data-testid attributes so selectors are readable and URL-safe.
func nameSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	return strings.NewReplacer(" ", "-", "/", "-", ".", "-", "'", "", "\"", "").Replace(s)
}

// Subscriptions lists recurring charges detected from transaction history (B25):
// each subscription's cadence, charge, normalized monthly cost, and next renewal,
// plus the total monthly/annual burden. Includes:
//   - Per-row cancel-candidate selection with running "save $X/year" summary.
//   - Bulk "mark selected as cancelled" action.
//   - Quiet "worth reviewing?" badge on subscriptions not seen in 2+ cadence intervals.
func Subscriptions() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	pr := uistate.UsePrefs().Get()

	// Drill from a detected subscription to its underlying charges: open
	// Transactions searched for the payee, so the user can verify the detection
	// (mirrors the Accounts/Budgets/Goals drill pattern, C30/C56).
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	viewCharges := func(payee string) {
		f := uistate.TxFilter{Text: payee}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	subs, _ := subscriptions.Detect(app.Transactions(), rates, 2)
	changes, _ := subscriptions.DetectPriceChanges(app.Transactions(), rates, 3)
	soon := subscriptions.UpcomingRenewals(subs, 7, time.Now())

	var annual int64
	for _, s := range subs {
		annual += s.AnnualAmount()
	}

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
	notice := uistate.UseNotice()
	remind := func(s subscriptions.Subscription) {
		app := appstate.Default
		if app == nil {
			return
		}
		task := domain.Task{
			ID:       id.New(),
			Title:    uistate.T("subs.reminderTitle", s.Name),
			Notes:    uistate.T("subs.reminderNote", fmtMoney(money.New(s.Amount, base)), subscriptionCadenceLabel(s.Cadence)),
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

	// --- Cancel-candidates multi-select (L12) ---
	// Session state: map of sub Name → selected. All mutations copy-on-write so
	// UseState detects the change and re-renders.
	selectedState := ui.UseState(map[string]bool{})

	toggle := func(name string) {
		cur := selectedState.Get()
		next := make(map[string]bool, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[name] = !next[name]
		selectedState.Set(next)
	}

	// Bulk cancel: mark every selected (non-cancelled) subscription as cancelled,
	// then clear the selection so the UI resets cleanly.
	doBulkCancel := func() {
		app := appstate.Default
		if app == nil {
			return
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
			if err := app.MarkSubscriptionCancelled(name, now); err != nil {
				notice.Set(notice.Get().With(err.Error(), true))
				return
			}
		}
		selectedState.Set(map[string]bool{})
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

	// Build the savings summary bar (only shown when ≥1 subscription is selected).
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
				OnClick(ui.UseEvent(Prevent(doBulkCancel))),
				uistate.T("subs.cancelSelected"),
			),
		)
	} else {
		savingsSummary = Fragment()
	}

	now := time.Now()
	rows := MapKeyed(subs,
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
				OnRemind:       remind,
				OnDrill:        viewCharges,
				OnCancel:       doCancel,
				OnUncancel:     doUncancel,
				OnToggleSelect: toggle,
			})
		},
	)

	var body ui.Node
	if len(subs) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("subs.empty"), CTALabel: uistate.T("subs.addFirst"), Href: "/transactions"})
	} else {
		body = Div(css.Class("rows"), rows)
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

	// Subscriptions as a share of this month's spending — a "how much of my
	// outflow is recurring?" gauge, shown only when there's spending to compare to.
	shareStat := Fragment()
	ms, me := dateutil.MonthRange(time.Now())
	if _, expense, err := ledger.PeriodTotals(app.Transactions(), ms, me, rates); err == nil && expense.Amount > 0 {
		pct := subscriptions.MonthlyTotal(subs) * 100 / expense.Amount
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

	return Div(
		If(len(lateCharges) > 0, Section(
			css.Class("card"),
			Attr("role", "alert"),
			Attr("aria-live", "polite"),
			Style(map[string]string{"border-left": "4px solid var(--color-danger, #ef4444)"}),
			H2(css.Class("card-title "+tw.ColorClass("text-down")),
				uistate.T("subs.lateChargesTitle")),
			Div(css.Class("rows"), lateChargeRows),
		)),
		If(len(subs) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("subs.monthlyBurden"), fmtMoney(money.New(subscriptions.MonthlyTotal(subs), base)), "neg"),
			stat(uistate.T("subs.annualBurden"), fmtMoney(money.New(annual, base)), ""),
			stat(uistate.T("subs.count"), fmt.Sprintf("%d", len(subs)), ""),
			shareStat,
		)),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("nav.subscriptions")),
			body,
			savingsSummary,
			If(len(subs) > 0, Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("subs.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("subscriptions.csv", "text/csv", subscriptions.CSV(subs, csvAmount))
				}), uistate.T("subs.downloadCsv")),
			)),
		),
		If(len(changes) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("subs.priceChangesTitle")),
			Div(css.Class("rows"), changeRows),
		)),
		If(len(soon) > 0, Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("subs.renewingSoon")),
			// Renewing-soon rows reuse the full SubscriptionRow so each imminent
			// renewal is actionable in place (remind / cancel) — not a stripped
			// read-only card (C56). The cancelledOn lookup, selection state, and
			// all callbacks are wired identically to the main list.
			Div(css.Class("rows"), MapKeyed(soon,
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
						OnRemind:       remind,
						OnDrill:        viewCharges,
						OnCancel:       doCancel,
						OnUncancel:     doUncancel,
						OnToggleSelect: toggle,
					})
				},
			)),
		)),
	)
}

// subscriptionRowProps holds the props for a single subscription row.
type subscriptionRowProps struct {
	Sub            subscriptions.Subscription
	Base           string
	NextDate       string // pre-formatted next-renewal date
	Cancelled      bool
	CancelledOn    string // pre-formatted cancellation date, set when Cancelled is true
	Selected       bool   // whether the row's cancel-candidate checkbox is checked
	NeedsReview    bool   // whether the subscription hasn't been charged in 2+ cadence intervals
	OnRemind       func(subscriptions.Subscription)
	OnDrill        func(payee string) // open Transactions searched for this subscription's payee
	OnCancel       func(name string)
	OnUncancel     func(name string)
	OnToggleSelect func(name string) // toggle cancel-candidate selection for this row
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
	toggleSelect := ui.UseEvent(Prevent(func() {
		if props.OnToggleSelect != nil {
			props.OnToggleSelect(s.Name)
		}
	}))

	slug := nameSlug(s.Name)
	meta := subscriptionCadenceLabel(s.Cadence) + " · " + uistate.T("subs.next", props.NextDate)

	// Quiet "worth reviewing?" nudge — only shown for non-cancelled rows where the
	// last charge is suspiciously old (2+ cadence intervals). Not shown for
	// cancelled subs (the user already knows they cancelled it).
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

	return Div(css.Class("row sub-row"),
		selectCheckbox,
		Div(css.Class("row-main"),
			Button(css.Class("row-desc sub-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drill),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "font-weight": "600", "color": "var(--text)", "text-align": "left", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
				s.Name),
			Span(css.Class("row-meta"), meta),
			statusArea,
			reviewBadge,
		),
		// Only show the normalized "/mo" figure when it differs from the actual
		// charge (i.e. weekly/yearly). For monthly subs they're identical, so
		// showing both reads as a duplicated amount (C56).
		If(s.Cadence != subscriptions.CadenceMonthly,
			Span(css.Class("row-meta"), uistate.T("subs.perMonth", fmtMoney(money.New(s.MonthlyAmount(), props.Base))))),
		Span(css.Class("budget-amount"), fmtMoney(money.New(s.Amount, props.Base))),
		actions,
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
