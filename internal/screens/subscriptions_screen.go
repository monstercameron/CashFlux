//go:build js && wasm

package screens

import (
	"fmt"
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

// Subscriptions lists recurring charges detected from transaction history (B25):
// each subscription's cadence, charge, normalized monthly cost, and next renewal,
// plus the total monthly/annual burden. Read-only over the pure detection core.
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

	rows := MapKeyed(subs,
		func(s subscriptions.Subscription) any { return s.Name + "|" + fmt.Sprint(s.Amount) },
		func(s subscriptions.Subscription) ui.Node {
			return ui.CreateElement(SubscriptionRow, subscriptionRowProps{Sub: s, Base: base, NextDate: pr.FormatDate(s.NextRenewal), OnRemind: remind, OnDrill: viewCharges})
		},
	)

	var body ui.Node
	if len(subs) == 0 {
		body = P(css.Class("empty"), uistate.T("subs.empty"))
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
					Span(ClassStr("row-meta inline-flex items-center gap-1 "+tone),
						uiw.Icon(arrow, css.Class("shrink-0", tw.W35, tw.H35)),
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

	return Div(
		If(len(subs) > 0, Div(css.Class("stat-grid"),
			stat(uistate.T("subs.monthlyBurden"), fmtMoney(money.New(subscriptions.MonthlyTotal(subs), base)), "neg"),
			stat(uistate.T("subs.annualBurden"), fmtMoney(money.New(annual, base)), ""),
			stat(uistate.T("subs.count"), fmt.Sprintf("%d", len(subs)), ""),
			shareStat,
		)),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("nav.subscriptions")),
			body,
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
			Div(css.Class("rows"), MapKeyed(soon,
				func(s subscriptions.Subscription) any { return s.Name + "|" + fmt.Sprint(s.Amount) },
				func(s subscriptions.Subscription) ui.Node {
					return Div(css.Class("row"),
						Div(css.Class("row-main"),
							Span(css.Class("row-desc"), s.Name),
							Span(css.Class("row-meta"), pr.FormatDate(s.NextRenewal)),
						),
						Span(css.Class("budget-amount"), fmtMoney(money.New(s.Amount, base))),
					)
				},
			)),
		)),
	)
}

type subscriptionRowProps struct {
	Sub      subscriptions.Subscription
	Base     string
	NextDate string // pre-formatted next-renewal date
	OnRemind func(subscriptions.Subscription)
	OnDrill  func(payee string) // open Transactions searched for this subscription's payee
}

// SubscriptionRow renders one detected subscription with a "remind me to cancel"
// action. It owns its click hook (per the On*-hooks-in-loops rule), so the list
// can render many rows without reordering hooks.
func SubscriptionRow(props subscriptionRowProps) ui.Node {
	s := props.Sub
	remind := ui.UseEvent(Prevent(func() { props.OnRemind(s) }))
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(s.Name)
		}
	}))
	meta := subscriptionCadenceLabel(s.Cadence) + " · " + uistate.T("subs.next", props.NextDate)
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Button(css.Class("row-desc sub-drill"), Type("button"), Title(uistate.T("nav.transactions")), OnClick(drill),
				Style(map[string]string{"background": "transparent", "border": "0", "padding": "0", "margin": "0", "font": "inherit", "color": "inherit", "text-align": "left", "cursor": "pointer", "text-decoration": "underline", "text-decoration-style": "dotted", "text-underline-offset": "3px"}),
				s.Name),
			Span(css.Class("row-meta"), meta),
		),
		// Only show the normalized "/mo" figure when it differs from the actual
		// charge (i.e. weekly/yearly). For monthly subs they're identical, so
		// showing both reads as a duplicated amount (C56).
		If(s.Cadence != subscriptions.CadenceMonthly,
			Span(css.Class("row-meta"), uistate.T("subs.perMonth", fmtMoney(money.New(s.MonthlyAmount(), props.Base))))),
		Span(css.Class("budget-amount"), fmtMoney(money.New(s.Amount, props.Base))),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("subs.remindTitle")), OnClick(remind), uistate.T("subs.remind")),
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
