//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Subscriptions lists recurring charges detected from transaction history (B25):
// each subscription's cadence, charge, normalized monthly cost, and next renewal,
// plus the total monthly/annual burden. Read-only over the pure detection core.
func Subscriptions() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	pr := uistate.UsePrefs().Get()

	subs, _ := subscriptions.Detect(app.Transactions(), rates, 2)
	changes, _ := subscriptions.DetectPriceChanges(app.Transactions(), rates, 3)

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
			return ui.CreateElement(SubscriptionRow, subscriptionRowProps{Sub: s, Base: base, NextDate: pr.FormatDate(s.NextRenewal), OnRemind: remind})
		},
	)

	var body ui.Node
	if len(subs) == 0 {
		body = P(Class("empty"), uistate.T("subs.empty"))
	} else {
		body = Div(Class("rows"), rows)
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
			key := "subs.priceDown"
			if c.Increased() {
				key = "subs.priceUp"
			}
			return Div(Class("row"),
				Div(Class("row-main"),
					Span(Class("row-desc"), c.Name),
					Span(Class("row-meta"), uistate.T(key, delta, pctStr, date)),
				),
				Span(Class("budget-amount"), fmtMoney(money.New(c.NewAmount, base))),
			)
		},
	)

	return Div(
		If(len(subs) > 0, Div(Class("stat-grid"),
			stat(uistate.T("subs.monthlyBurden"), fmtMoney(money.New(subscriptions.MonthlyTotal(subs), base)), "neg"),
			stat(uistate.T("subs.annualBurden"), fmtMoney(money.New(annual, base)), ""),
			stat(uistate.T("subs.count"), fmt.Sprintf("%d", len(subs)), ""),
		)),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("nav.subscriptions")),
			body,
			If(len(subs) > 0, Div(Class("flex flex-wrap gap-2 py-1"),
				Button(Class("btn"), Type("button"), Title(uistate.T("subs.downloadCsvTitle")), OnClick(func() {
					csvAmount := func(v int64) string { return money.FormatMinor(v, currency.Decimals(base)) }
					downloadBytes("subscriptions.csv", "text/csv", subscriptions.CSV(subs, csvAmount))
				}), uistate.T("subs.downloadCsv")),
			)),
		),
		If(len(changes) > 0, Section(Class("card"),
			H2(Class("card-title"), uistate.T("subs.priceChangesTitle")),
			Div(Class("rows"), changeRows),
		)),
	)
}

type subscriptionRowProps struct {
	Sub      subscriptions.Subscription
	Base     string
	NextDate string // pre-formatted next-renewal date
	OnRemind func(subscriptions.Subscription)
}

// SubscriptionRow renders one detected subscription with a "remind me to cancel"
// action. It owns its click hook (per the On*-hooks-in-loops rule), so the list
// can render many rows without reordering hooks.
func SubscriptionRow(props subscriptionRowProps) ui.Node {
	s := props.Sub
	remind := ui.UseEvent(Prevent(func() { props.OnRemind(s) }))
	meta := subscriptionCadenceLabel(s.Cadence) + " · " + uistate.T("subs.next", props.NextDate)
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), s.Name),
			Span(Class("row-meta"), meta),
		),
		Span(Class("row-meta"), uistate.T("subs.perMonth", fmtMoney(money.New(s.MonthlyAmount(), props.Base)))),
		Span(Class("budget-amount"), fmtMoney(money.New(s.Amount, props.Base))),
		Button(Class("btn"), Type("button"), Title(uistate.T("subs.remindTitle")), OnClick(remind), uistate.T("subs.remind")),
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
