// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	// C681: Net covers CASH FLOW only. Transfer legs are money you still own, so
	// summing them into a figure labelled "net" describes moving money as earning
	// or spending it — and a filter showing one leg of a pair makes that reading
	// unavoidable. Moved is the total magnitude of each visible movement once,
	// MovedRows how many movements (or visible orphan legs) there are,
	// and UnpairedRows how many have no counterpart IN THIS VIEW (a claim about
	// the view, not about the data: the far side may sit outside the filter).
	Moved        money.Money
	MovedRows    int
	UnpairedRows int
	// Spent/Income are the cashflow totals over the SAME filtered set that
	// produced Shown/Net, computed once by the caller (the toolbar's
	// memoised summary). CashflowProvided distinguishes "the caller folded
	// the shown rows" from a genuine $0. When false (direct mounts in
	// tests), the line falls back to its own ledger pass.
	Spent, Income    money.Money
	CashflowProvided bool
}

// txnCountLine is the ledger's one-sentence answer to "what am I looking at?".
//
// C575: the page showed a review backlog ("Review inbox (249)"), quick-filter
// counts, and a pager range ("1–25 of 3,227") side by side, each measured against
// a different population, and the only statement that tied a number to its scope
// was SCREEN-READER ONLY — the sighted user got the numbers and none of the
// context. A count without its denominator is not information; "3,227" and "122
// of 3,227" are different facts and the page rendered them the same way.
//
// So the summary is visible, it always names the denominator, and it says whose
// money it is counting. It is also the live region, replacing the hidden one:
// announcing the same sentence twice — once visibly, once invisibly — is how a
// screen-reader user ends up hearing every filter change in duplicate.
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
	if props.Total == 0 && !futurePeriod {
		// The empty state below already explains an empty ledger, and a count line
		// reading "0 of 0" over it is noise on the emptiest possible screen.
		//
		// A hidden PLACEHOLDER, not an empty Fragment: the reconciler has no element
		// anchor for a fiber whose previous output was empty, so a component that
		// starts empty and fills in later is appended to the end of its container.
		// A brand-new ledger starts at zero and gains its first row minutes later —
		// exactly the sequence that would strand this line at the bottom of the
		// toolbar tile. (The return crumb hit this for real; this is the same trap
		// one component over.)
		return Span(css.Class("txn-countline-slot"), Attr("aria-hidden", "true"),
			Style(map[string]string{"display": "none"}))
	}
	// R3P: prefer the caller's fold over the already-filtered shown set. The
	// fallback re-runs a full filter pass over the ledger on every render
	// (sort, keystroke, selection) purely to re-derive two sums the toolbar
	// already has — the second pass C2-12 measured beside the table's own.
	var spent, income money.Money
	if props.CashflowProvided {
		spent, income = props.Spent, props.Income
	} else {
		spent, income = txnCountCashflow(filter)
	}
	period := txnCountPeriod(filter)
	parts := []ui.Node{}
	var searchWidenAction ui.Node
	if futurePeriod && props.Shown == 0 {
		items := txnFutureScheduledItems(filter, time.Now())
		total := money.New(0, "USD")
		if app := appstate.Default; app != nil {
			base := app.Settings().BaseCurrency
			if base == "" {
				base = "USD"
			}
			total = money.New(0, base)
			rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
			for _, item := range items {
				if converted, err := rates.Convert(item.Amount, base); err == nil {
					total.Amount += converted.Amount
				}
			}
		}
		parts = append(parts, Span(css.Class("txn-count-main"), uistate.T("r4a.futureSummary",
			period, txnCountNumber(0), txnCountNumber(scheduledCount), fmtMoney(total))))
	} else {
		parts = append(parts, Span(css.Class("txn-count-main"), period, " · ", txnCountTransactionCount(props.Shown)))
		showCashflow := spent.Amount != 0 || income.Amount != 0
		if showCashflow && filter.Flow != "in" {
			parts = append(parts,
				Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), " · "),
				Button(css.Class("txn-count-filter"), Type("button"), Attr("data-testid", "txn-count-spent"),
					Attr("aria-pressed", ariaBool(filter.Flow == "out")),
					OnClick(Prevent(func() {
						setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
							if f.Flow == "out" {
								f.Flow = ""
							} else {
								f.Flow = "out"
							}
						})
					})), uistate.T("r2a.spent", fmtMoney(spent))),
			)
		}
		if showCashflow && filter.Flow != "out" && (filter.Flow == "in" || income.Amount != 0 || spent.Amount == 0) {
			parts = append(parts,
				Span(css.Class("txn-count-sep"), Attr("aria-hidden", "true"), " · "),
				Button(css.Class("txn-count-filter"), Type("button"), Attr("data-testid", "txn-count-income"),
					Attr("aria-pressed", ariaBool(filter.Flow == "in")),
					OnClick(Prevent(func() {
						setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
							if f.Flow == "in" {
								f.Flow = ""
							} else {
								f.Flow = "in"
							}
						})
					})), uistate.T("r2a.income", fmtMoney(income))),
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
				OnClick(Prevent(func() {
					setTxFilterOn(filterAtom, func(f *uistate.TxFilter) {
						f.From, f.To, f.PeriodPreset = "", "", txnfilter.PeriodAllDates
					})
				})), uistate.T("r5n.searchAllDates", txnCountNumber(allCount))))
	}
	if props.Shown > 0 && filter.Flow == "" {
		// C681: only say "net" about rows that actually are cash flow. When the view
		// holds nothing BUT transfer legs, a net of $0.00 is true and useless — it
		// invites the reader to conclude the money vanished — so the figure is
		// dropped entirely and the moved-money sentence carries the view instead.
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
		Attr("aria-label", uistate.T("transactions.scopeAria")),
		parts)
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

// txnCashflowFromShown folds an already-filtered set into Spent (positive
// magnitude of money out) and Income in the base currency. Transfer legs are
// excluded because they do not change household cash flow; rows that cannot
// be converted are skipped, exactly as txnCountCashflow did — the caller
// states the unconverted count separately. Folding the shown set costs one
// pass over rows already in memory, versus a second full filter+sort of the
// ledger (R3P).
func txnCashflowFromShown(shown []domain.Transaction, rates currency.Rates, base string) (spent, income money.Money) {
	if base == "" {
		base = "USD"
	}
	spent = money.New(0, base)
	income = money.New(0, base)
	for _, txn := range shown {
		if txn.IsTransfer() {
			continue
		}
		converted, err := rates.Convert(txn.Amount, base)
		if err != nil {
			continue
		}
		if converted.Amount < 0 {
			spent.Amount -= converted.Amount
		} else {
			income.Amount += converted.Amount
		}
	}
	return spent, income
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

// txnCountCashflow totals visible incoming and outgoing activity in the selected
// base currency. Transfer legs are excluded because they do not change household cash flow.
// Live mounts pass CashflowProvided and never reach here; this remains the
// fallback for direct mounts that have no pre-folded shown set.
func txnCountCashflow(filter txnfilter.Criteria) (spent, income money.Money) {
	app := appstate.Default
	if app == nil {
		return money.New(0, "USD"), money.New(0, "USD")
	}
	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	spent = money.New(0, base)
	income = money.New(0, base)
	rates := currency.Rates{Base: base, Rates: settings.FXRates}
	resolver := app.PayeeResolver()
	labels := txnfilter.Labels{Payee: resolver.Resolve}
	for _, txn := range txnfilter.ApplyWithLabels(app.Transactions(), filter, labels) {
		if txn.IsTransfer() {
			continue
		}
		converted, err := rates.Convert(txn.Amount, base)
		if err != nil {
			continue
		}
		if converted.Amount < 0 {
			spent.Amount -= converted.Amount
		} else {
			income.Amount += converted.Amount
		}
	}
	return spent, income
}

func txnCountPeriod(filter txnfilter.Criteria) string {
	if filter.From == "" && filter.To == "" {
		return uistate.T("r2a.allDates")
	}
	date, err := time.Parse("2006-01-02", filter.From)
	if err != nil {
		date, err = time.Parse("2006-01-02", filter.To)
	}
	if err == nil {
		if filter.To == "" || filter.From == "" {
			return uistate.T("r2a.month", date.Format("January"), date.Year())
		}
		toDate, toErr := time.Parse("2006-01-02", filter.To)
		if toErr == nil && date.Year() == toDate.Year() && date.Month() == toDate.Month() {
			return uistate.T("r2a.month", date.Format("January"), date.Year())
		}
	}
	return uistate.T("r2a.customRange")
}

func txnCountSearchPeriod(filter txnfilter.Criteria) string {
	date, err := time.Parse("2006-01-02", filter.From)
	if err == nil {
		toDate, toErr := time.Parse("2006-01-02", filter.To)
		if toErr == nil && date.Year() == toDate.Year() && date.Month() == toDate.Month() {
			return date.Format("January")
		}
	}
	return txnCountPeriod(filter)
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
