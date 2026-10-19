// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// txnCountLineProps carries the numbers the line states and the scope that
// produced them.
type txnCountLineProps struct {
	Shown int    // rows after every filter and the member lens
	Total int    // rows in the whole ledger
	Range string // retained for the caller; page ranges are no longer shown here
	Net   money.Money
	Lens  string // the lensed member's name, "" for everyone
	// Unconverted is how many of the Shown rows could not be folded into the base
	// currency (no FX rate), and so are missing from Net. The count and the net are
	// printed side by side, so a net that quietly covers fewer rows than the count
	// is two different facts wearing one sentence — it is stated instead.
	Unconverted int
	// Excluded counts visible rows intentionally omitted from reports and budgets.
	Excluded int
	// Net covers cash flow only; transfers are summarized separately as Moved.
	// Moved is the total magnitude of each visible movement once,
	// MovedRows how many movements (or visible orphan legs) there are,
	// and UnpairedRows how many have no counterpart IN THIS VIEW (a claim about
	// the view, not about the data: the far side may sit outside the filter).
	Moved        money.Money
	MovedRows    int
	UnpairedRows int
	// Spent and Income summarize the same filtered rows as Shown and Net.
	Spent, Income money.Money
	// CashflowProvided remains for callers compiled against the prior props shape.
	CashflowProvided bool
}

// txnCountLine summarizes the visible transaction scope and its totals.
func txnCountLine(props txnCountLineProps) ui.Node {
	return ui.CreateElement(txnCountLineComponent, props)
}

func txnCountLineComponent(props txnCountLineProps) ui.Node {
	filterAtom := uistate.UseTxFilter()
	memberLens := uistate.UseMemberLens()
	filter := filterAtom.Get()
	scope := txnAnnouncementScope(filter)
	lastScope := ui.UseState(scope)
	announceFilterChange := lastScope.Get() != scope
	ui.UseEffect(func() func() {
		if announceFilterChange {
			lastScope.Set(scope)
		}
		return nil
	}, scope)

	filter, _ = filter.WithOwnerLens(memberLens)
	futurePeriod, scheduledCount := txnFutureScheduleCount(filter, time.Now())
	onSpent := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			if f.Flow == "out" {
				f.Flow = ""
			} else {
				f.Flow = "out"
			}
		})
	}))
	onIncome := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			if f.Flow == "in" {
				f.Flow = ""
			} else {
				f.Flow = "in"
			}
		})
	}))
	onSearchAllDates := ui.UseEvent(Prevent(func() {
		setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
			f.From, f.To, f.PeriodPreset = "", "", txnfilter.PeriodAllDates
		})
	}))
	if props.Total == 0 && !futurePeriod {
		// Keep an invisible anchor so the line stays in place when the ledger fills.
		return Span(css.Class("txn-countline-slot"), Attr("aria-hidden", "true"),
			Style(map[string]string{"display": "none"}))
	}
	// Use the caller's single-pass summary for the current filtered rows.
	spent, income := props.Spent, props.Income
	period := txnCountPeriod(filter)
	parts := []ui.Node{}
	var searchWidenAction ui.Node
	if futurePeriod && props.Shown == 0 {
		items := txnFutureScheduledItems(filter, time.Now())
		base := ""
		var rates currency.Rates
		if app := appstate.Default; app != nil {
			settings := app.Settings()
			base = settings.BaseCurrency
			rates = currency.Rates{Base: base, Rates: settings.FXRates}
		}
		total, unconverted := txnFutureTotal(items, rates, base)
		amount, tone := fmtLedgerAmount(money.New(-total.Amount, total.Currency), false)
		parts = append(parts, Span(css.Class("txn-count-main"),
			uistate.T("fut2.futureSummary", period, txnCountNumber(0), txnCountNumber(scheduledCount)),
			Span(css.Class(tw.ColorClass(tone)), amount)))
		if unconverted > 0 {
			parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), " · "),
				Span(css.Class("txn-count-unconverted"), uistate.T("transactions.scopeUnconverted", unconverted)))
		}
	} else {
		parts = append(parts, Span(css.Class("txn-count-main"), period, " · ", txnCountTransactionCount(props.Shown)))
		showCashflow := spent.Amount != 0 || income.Amount != 0
		if showCashflow && filter.Flow != "in" {
			parts = append(parts,
				Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), " · "),
				Button(css.Class("txn-count-filter", "text-down"), Type("button"), Attr("data-testid", "txn-count-spent"),
					Attr("aria-pressed", ariaBool(filter.Flow == "out")),
					OnClick(onSpent), uistate.T("r2a.spent", fmtTxnSignedMoney(money.New(-spent.Amount, spent.Currency)))),
			)
		}
		if showCashflow && filter.Flow != "out" && (filter.Flow == "in" || income.Amount != 0 || spent.Amount == 0) {
			parts = append(parts,
				Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), " · "),
				Button(css.Class("txn-count-filter"), Type("button"), Attr("data-testid", "txn-count-income"),
					Attr("aria-pressed", ariaBool(filter.Flow == "in")),
					OnClick(onIncome), uistate.T("r2a.income", fmtMoney(income))),
			)
		}
	}
	preset := filter.PeriodPreset
	if preset == "" {
		preset = txnfilter.InferPeriodPreset(filter.From, filter.To, time.Now())
	}
	if q := strings.TrimSpace(filter.Text); q != "" && preset != txnfilter.PeriodAllDates && preset != txnfilter.PeriodCustom {
		allDates := filter
		allDates.From, allDates.To, allDates.PeriodPreset = "", "", txnfilter.PeriodAllDates
		allCount := txnSearchAllDatesCount(allDates)
		searchWidenAction = Span(css.Class("txn-count-search-action"), Style(map[string]string{"marginLeft": "0.35rem"}),
			Button(css.Class("btn-link txn-count-search-widen"), Type("button"), Attr("data-testid", "txn-search-all-dates"),
				OnClick(onSearchAllDates), uistate.T("r5n.searchAllDates", txnCountNumber(allCount))))
	}
	if props.Shown > 0 && filter.Flow == "" {
		// Suppress a zero net when all visible rows are transfers.
		if txnCountShowsNet(spent, income) && (props.MovedRows == 0 || props.Shown > props.MovedRows) {
			parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
				Span(css.Class("txn-count-net"), uistate.T("r2a.net", signedTxnMoney(props.Net))))
		}
	}
	if props.MovedRows > 0 {
		// Money that changed accounts without entering or leaving the household.
		// Said separately from the net rather than folded into it: a transfer leg
		// summed into a cash-flow figure describes moving money as spending it.
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-moved"), Attr("data-testid", "txn-count-moved"),
				uistate.T("transactions.scopeMoved", fmtMoney(props.Moved))))
		if props.UnpairedRows > 0 {
			// The far side is not in view. It may exist outside the filter, so this
			// says the view is one-sided rather than claiming the data is broken.
			parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
				Span(css.Class("txn-count-unpaired"), Attr("data-testid", "txn-count-unpaired"),
					uistate.T("transactions.scopeUnpaired", props.UnpairedRows)))
		}
	}
	if props.Unconverted > 0 {
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-unconverted"),
				uistate.T("transactions.scopeUnconverted", props.Unconverted)))
	}
	if props.Excluded > 0 {
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-excluded"), Attr("data-testid", "txn-count-excluded"),
				uistate.T("transactions.scopeExcluded", props.Excluded)))
	}
	if props.Lens != "" {
		// Stated here as well as on the chip: the chip is a control the eye skips, and
		// this is the sentence that explains the number sitting next to it.
		parts = append(parts, Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), "·"),
			Span(css.Class("txn-count-lens"), uistate.T("transactions.scopeLens", props.Lens)))
	}
	if !(futurePeriod && props.Shown == 0) {
		parts = append(parts, ui.CreateElement(txnUpcomingInline, struct{}{}))
	}
	if searchWidenAction != nil {
		parts = append(parts, searchWidenAction)
	}
	return Span(css.Class("txn-countline", tw.MinW0),
		Attr("data-testid", "txn-count-line"),
		Attr("role", "status"), Attr("aria-live", liveMode(announceFilterChange)), Attr("aria-atomic", "true"),
		parts)
}

func txnFutureTotal(items []bills.Bill, rates currency.Rates, base string) (money.Money, int) {
	total := money.New(0, base)
	unconverted := 0
	for _, item := range items {
		converted, err := rates.Convert(item.Amount, base)
		if err != nil {
			unconverted++
			continue
		}
		total.Amount += converted.Amount
	}
	return total, unconverted
}

func txnCountShowsNet(spent, income money.Money) bool {
	return spent.Amount > 0 && income.Amount > 0
}

func txnCountTransactionCount(n int) string {
	key := "r3n.transactionCountMany"
	if n == 1 {
		key = "r3n.transactionCountOne"
	}
	return uistate.T(key, txnCountNumber(n))
}

// txnSearchAllDatesMemo caches the "search all dates" widening count. That
// count needs a full filter pass under WIDENED criteria, and the count line
// re-renders on sort/selection changes that cannot move it. Keyed on the app
// and its data revision plus the widened criteria; every transaction write
// bumps the revision, so a stale count cannot outlive an edit.
var txnSearchAllDatesMemo struct {
	app    *appstate.App
	rev    int
	filter txnfilter.Criteria
	count  int
	valid  bool
}

func txnSearchAllDatesCount(allDates txnfilter.Criteria) int {
	app := appstate.Default
	if app == nil {
		return 0
	}
	rev := uistate.CurrentDataRevision()
	m := &txnSearchAllDatesMemo
	if m.valid && m.app == app && m.rev == rev && m.filter == allDates {
		return m.count
	}
	count := len(txnfilter.ApplyWithLabels(app.Transactions(), allDates, txnfilter.Labels{Payee: app.PayeeResolver().Resolve}))
	m.app, m.rev, m.filter, m.count, m.valid = app, rev, allDates, count, true
	return count
}

func txnCountPeriod(filter txnfilter.Criteria) string {
	if filter.From == "" && filter.To == "" {
		return uistate.T("r2a.allDates")
	}
	from, fromErr := time.Parse("2006-01-02", filter.From)
	to, toErr := time.Parse("2006-01-02", filter.To)
	if filter.From == "" && toErr == nil {
		return uistate.T("txnTQ6.throughDate", uistate.LoadPrefs().FormatDate(to))
	}
	if filter.To == "" && fromErr == nil {
		return uistate.T("txnTQ6.fromDate", uistate.LoadPrefs().FormatDate(from))
	}
	if fromErr == nil && toErr == nil && from.Year() == to.Year() && from.Month() == to.Month() && from.Day() == 1 && to.AddDate(0, 0, 1).Day() == 1 {
		return uistate.T("r2a.month", uistate.T("budgets.month."+strings.ToLower(from.Month().String())), from.Year())
	}
	if fromErr == nil && toErr == nil {
		return uistate.T("txnperiod.customRange", uistate.LoadPrefs().FormatDate(from), uistate.LoadPrefs().FormatDate(to))
	}
	return uistate.T("r2a.customRange")
}

func signedTxnMoney(value money.Money) string {
	return fmtTxnSignedMoney(value)
}

func txnAnnouncementScope(filter txnfilter.Criteria) txnfilter.Criteria {
	filter.Sort, filter.Dir, filter.Page, filter.PageSize = "", "", 0, 0
	return filter
}

func liveMode(announce bool) string {
	if announce {
		return "polite"
	}
	return "off"
}
