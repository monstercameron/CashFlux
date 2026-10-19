// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"sort"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// coverAllFeatureCode is the SMART-B14 catalog code that gates the cover-overages
// feature (Free, on by default, opt-out under the Smart settings).
const coverAllFeatureCode = "SMART-B14"

// coverAllNextSource is the sentinel select value meaning "borrow from next month's
// same budget" rather than moving slack from another budget.
const coverAllNextSource = "__next__"

var coverAllOpener js.Value

// coverAllOver is one over-budget the modal offers to cover: the shortfall to clear
// and the period boundaries used to apply the coverage as a per-period boost.
type coverAllOver struct {
	ID        string
	Name      string
	Rollover  bool
	Currency  string
	Shortfall money.Money // positive amount over the limit this period
	ThisStart time.Time   // current period start (the boost target)
	NextStart time.Time   // next period start (for the next-month borrow)
}

// coverAllSource is a budget with slack this period that can fund another's overage.
type coverAllSource struct {
	ID        string
	Name      string
	Currency  string
	Avail     money.Money // positive free room this period after committed bills
	FreeRoom  money.Money // remaining after committed bills, normalized per month for ranking
	Monthly   money.Money // free room normalized per month, for the donor label
	ThisStart time.Time
}

// coverAllData reuses the SAME evaluated statuses the budget grid shows (via
// computeBudgetView — rollup, smoothing, and member scope all applied) so the modal's
// over/source list matches the banner exactly, then derives each budget's period
// bounds the identical way computeBudgetView anchors them (so the per-period boost
// lands on the period whose overage it's covering). Pure read model.
func coverAllData(app *appstate.App, member string, vw period.Window, pr prefs.Prefs, showLastMonth bool) (overs []coverAllOver, sources []coverAllSource, base string) {
	_ = showLastMonth // statuses and writes always describe the viewed period.
	v := computeBudgetView(app, member, vw, pr, false)
	base = v.Base
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	weekStart := pr.WeekStartWeekday()
	payAnchor := budgetPayCycleAnchor(pr.PayCycleAnchor)
	// Anchor exactly like computeBudgetView: today when today is inside the viewed
	// window, otherwise the window's start.
	anchor := budgetViewAnchor(vw, dateutil.WallClockUTC(time.Now()))
	input, err := app.BudgetViewInput(anchor)
	if err != nil {
		return nil, nil, base
	}
	for _, st := range v.Statuses {
		b := st.Budget
		if b.IsSaving() {
			continue
		}
		var bs, be time.Time
		bs, be = budgeting.PeriodRangeAnchored(b.Period, anchor, weekStart, payAnchor)
		switch {
		case st.State == budgeting.StateOver:
			overs = append(overs, coverAllOver{ID: b.ID, Name: b.Name, Rollover: b.Rollover, Currency: b.Limit.Currency, Shortfall: budgeting.CoverAmount(st), ThisStart: bs, NextStart: be})
		case st.State == budgeting.StateOK && st.Remaining.Amount > 0:
			funds, err := budgeting.CoverSourceFunds(b, input)
			if err != nil {
				continue
			}
			if !funds.CanGive() {
				continue
			}
			monthlyFree, err := coverAllMonthlyEquivalent(funds.Free, b.Period)
			if err != nil || monthlyFree.Amount < monthlyDonorMinimum(monthlyFree.Currency) {
				continue
			}
			sources = append(sources, coverAllSource{ID: b.ID, Name: b.Name, Currency: funds.Free.Currency, Avail: funds.Free,
				FreeRoom: monthlyFree, Monthly: monthlyFree, ThisStart: bs})
		}
	}
	sort.Slice(overs, func(i, j int) bool { return overs[i].Name < overs[j].Name })
	sort.SliceStable(sources, func(i, j int) bool { return coverAllSourceLess(sources[i], sources[j]) })
	return overs, sources, base
}

func coverAllSourceLess(a, b coverAllSource) bool {
	if a.FreeRoom.Amount != b.FreeRoom.Amount {
		return a.FreeRoom.Amount > b.FreeRoom.Amount
	}
	return a.Name < b.Name
}

func monthlyDonorMinimum(code string) int64 {
	minimum := int64(1)
	for range currency.Decimals(code) {
		minimum *= 10
	}
	return minimum
}

func coverAllMonthlyRoom(remaining money.Money, cadence domain.Period) (money.Money, error) {
	monthly, err := budgeting.PeriodEquivalent(remaining, cadence, domain.PeriodMonthly)
	if err != nil {
		return money.Money{}, err
	}
	// The monthly equivalent ranks and describes a donor; the amount movable
	// now remains capped at the actual period balance.
	if monthly.Amount > remaining.Amount {
		return remaining, nil
	}
	return monthly, nil
}

func coverAllMonthlyEquivalent(amount money.Money, cadence domain.Period) (money.Money, error) {
	return budgeting.PeriodEquivalent(amount, cadence, domain.PeriodMonthly)
}

func coverAllCoveredAt(over coverAllOver) time.Time {
	return over.ThisStart
}

func coverAllDefaultSource(over coverAllOver, sources []coverAllSource) string {
	for _, source := range sources {
		if source.Currency == over.Currency {
			return source.ID
		}
	}
	return ""
}

func coverAllSourcesFor(over coverAllOver, sources []coverAllSource) []coverAllSource {
	matching := make([]coverAllSource, 0, len(sources))
	for _, source := range sources {
		if source.Currency == over.Currency {
			matching = append(matching, source)
		}
	}
	return matching
}

func coverAllHasDonor(chosen map[string]string) bool {
	for _, source := range chosen {
		if source != "" {
			return true
		}
	}
	return false
}

func coverAllSelectedTotal(overs []coverAllOver, chosen map[string]string) (int64, int) {
	amount, count, _ := coverAllSelectedTotalChecked(overs, chosen)
	return amount, count
}

func coverAllSelectedTotalChecked(overs []coverAllOver, chosen map[string]string) (int64, int, error) {
	var amount int64
	count := 0
	for _, over := range overs {
		if chosen[over.ID] == "" {
			continue
		}
		var err error
		amount, err = money.Add(amount, over.Shortfall.Amount)
		if err != nil {
			return 0, 0, err
		}
		count++
	}
	return amount, count, nil
}

func coverAllHasNextMonthSelection(chosen map[string]string) bool {
	for _, source := range chosen {
		if source == coverAllNextSource {
			return true
		}
	}
	return false
}

// coverAllButtonProps carries nothing — the button reads shared state itself.
type coverAllButtonProps struct{}

// coverAllBannerButton is the over-banner entry point that opens the cover-overages
// modal. Its own component so its click hook stays at a stable position; the CALLER
// gates it on SMART-B14 + an over-count so this only mounts when there's work to do.
func coverAllBannerButton(_ coverAllButtonProps) ui.Node {
	open := uistate.UseCoverAllOpen()
	onOpen := ui.UseEvent(Prevent(func() {
		coverAllOpener = js.Global().Get("document").Get("activeElement")
		open.Set(true)
	}))
	return Button(css.Class("btn btn-tool cover-all-open", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-cover-all"), Title(uistate.T("bv7g.cover.title")), OnClick(onOpen),
		uiw.Icon(icon.Scale, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("coverAll.button")))
}

// budgetsCoverAllModal renders the cover-overages flip modal when its atom is set,
// as a shell-root sibling of the bento (no tile transform clips it). Matches the
// Add-budget footer standard (NoFooter + FlushBody + the form's pinned .modal-foot).
func budgetsCoverAllModal() ui.Node {
	open := uistate.UseCoverAllOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("bv7g.cover.title"),
		Width:     uiw.FlipMediumW,
		Height:    uiw.FlipMediumH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { closeCoverAll(func() { open.Set(false) }) },
		Back:      ui.CreateElement(coverAllForm, coverAllFormProps{OnDone: func() { closeCoverAll(func() { open.Set(false) }) }}),
	})
}

func closeCoverAll(close func()) {
	close()
	opener := coverAllOpener
	if !opener.Truthy() || opener.Get("focus").Type() != js.TypeFunction {
		return
	}
	var restore js.Func
	restore = js.FuncOf(func(js.Value, []js.Value) any {
		restore.Release()
		if opener.Get("isConnected").Bool() {
			opener.Call("focus")
		}
		return nil
	})
	js.Global().Call("setTimeout", restore, 0)
}

type coverAllFormProps struct {
	OnDone   func()
	TargetID string
}

// coverAllForm is the modal body: an intro, one row per over-budget with a per-budget
// coverage-source picker (leave / next month / another budget's slack), and a pinned
// Cancel + "Cover all" footer that applies every chosen coverage in one pass.
func coverAllForm(props coverAllFormProps) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	member := uistate.UseActiveMember().Get()
	pr := uistate.UsePrefs().Get()
	vw := uistate.UsePeriod().Get()
	showLM := uistate.UseBudgetsLastMonth().Get()
	choices := ui.UseState(map[string]string{})
	errText := ui.UseState("")
	checkpointSaving := ui.UseState(false)
	seededSig := ui.UseState("")
	var overs []coverAllOver
	var sources []coverAllSource
	base := budgetBaseCurrency("")
	if app != nil {
		overs, sources, base = coverAllData(app, member, vw, pr, showLM)
	}
	if props.TargetID != "" {
		filtered := overs[:0]
		for _, over := range overs {
			if over.ID == props.TargetID {
				filtered = append(filtered, over)
			}
		}
		overs = filtered
	}
	currencies := map[string]bool{}
	for _, o := range overs {
		currencies[o.Currency] = true
	}
	for _, s := range sources {
		currencies[s.Currency] = true
	}
	currencyError := len(currencies) > 1
	if len(currencies) == 1 {
		for code := range currencies {
			base = code
		}
	}
	dec := currency.Decimals(base)

	// choices: overBudgetID → source value ("" leave, "__next__", or a source budget id).
	// Seed each over to "next month" the first time this over-set is shown (a stable
	// signature so reopening with the same overs keeps the user's picks).
	sig := ""
	for _, o := range overs {
		sig += o.ID + ";"
	}
	if seededSig.Get() != sig {
		seed := map[string]string{}
		choices.Set(seed)
		errText.Set("")
		seededSig.Set(sig)
	}

	setChoice := func(overID, v string) {
		m := choices.Get()
		next := make(map[string]string, len(m))
		for k, val := range m {
			next[k] = val
		}
		next[overID] = v
		choices.Set(next)
	}

	srcByID := make(map[string]coverAllSource, len(sources))
	for _, s := range sources {
		srcByID[s.ID] = s
	}

	apply := ui.UseEvent(Prevent(func() {
		if checkpointSaving.Get() || currencyError {
			return
		}
		if app == nil {
			return
		}
		sel := choices.Get()
		if _, _, err := coverAllSelectedTotalChecked(overs, sel); err != nil {
			errText.Set(budgetErrorText(err))
			return
		}
		// Validate that no source budget is over-drawn across all rows that draw on it.
		drawn := map[string]int64{}
		for _, o := range overs {
			src := sel[o.ID]
			if src == coverAllNextSource && o.Rollover {
				errText.Set(uistate.T("coverAll.rolloverBorrowBlocked", o.Name))
				return
			}
			if src == "" || src == coverAllNextSource {
				continue
			}
			s, ok := srcByID[src]
			if !ok || s.Currency != o.Currency {
				errText.Set(uistate.T("coverAll.currencyError"))
				return
			}
			amount, err := money.Add(drawn[src], o.Shortfall.Amount)
			if err != nil {
				errText.Set(budgetErrorText(err))
				return
			}
			drawn[src] = amount
		}
		for id, amt := range drawn {
			s := srcByID[id]
			if amt > s.Avail.Amount {
				errText.Set(uistate.T("coverAll.sourceShort", s.Name, fmtMoney(money.New(amt-s.Avail.Amount, s.Currency))))
				return
			}
		}

		// #55: checkpoint before the multi-budget rewrite so Settings → Data can
		// roll the whole cover-all back in one click.
		checkpointSaving.Set(true)
		uistate.SaveCheckpointThen(uistate.T("ckpt.beforeCoverAll", budgetCountPhrase(len(overs))), func(_ string, checkpointErr error) {
			checkpointSaving.Set(false)
			if checkpointErr != nil {
				uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
				return
			}

			requests := make([]budgeting.CoverRequest, 0, len(overs))
			for _, over := range overs {
				source := sel[over.ID]
				if source == "" || !over.Shortfall.IsPositive() {
					continue
				}
				request := budgeting.CoverRequest{DestinationID: over.ID}
				if source == coverAllNextSource {
					request.BorrowNext = over.Shortfall
				} else {
					request.Shares = map[string]money.Money{source: over.Shortfall}
				}
				requests = append(requests, request)
			}
			applied := len(requests)
			if applied == 0 {
				errText.Set(uistate.T("coverAll.doneNone"))
				return
			}
			// Re-evaluate room after the checkpoint, then commit all pairs in
			// one transaction. The preview and every other cover share a planner.
			if err := app.CoverBudgetsAt(requests, budgetViewAnchor(vw, budgetNow())); err != nil {
				slog.Error("budgets: cover-all write rejected", "err", err)
				errText.Set(budgetErrorText(err))
				return
			}
			uistate.RequestPersist()
			uistate.BumpDataRevision()
			msg := uistate.T("coverAll.doneMany", applied)
			if applied == 1 {
				o := overs[0]
				sourceID := sel[o.ID]
				if sourceID != "" && sourceID != coverAllNextSource {
					msg = uistate.T("br3dlg.coveredFrom", fmtMoney(o.Shortfall), srcByID[sourceID].Name)
				} else {
					msg = uistate.T("coverAll.doneOne")
				}
			}
			uistate.PostNotice(msg, false)
			if props.OnDone != nil {
				props.OnDone()
			}
		})
	}))
	onCancel := ui.UseEvent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	})

	empty := len(overs) == 0

	var totalOver int64
	var totalErr error
	for _, o := range overs {
		totalOver, totalErr = money.Add(totalOver, o.Shortfall.Amount)
		if totalErr != nil {
			break
		}
	}
	intro := uistate.T("coverAll.intro", len(overs), fmtMoney(money.New(totalOver, base)))
	if len(overs) == 1 {
		intro = uistate.T("coverAll.introOne", fmtMoney(overs[0].Shortfall))
	}
	impactReach := fundsReachThisPeriod
	movesFromSelectedBudgets := false
	for _, source := range choices.Get() {
		if source == coverAllNextSource {
			impactReach = fundsReachThisAndNext
		} else if source != "" {
			movesFromSelectedBudgets = true
		}
	}
	if currencyError {
		intro = uistate.T("coverAll.currencyError")
	}
	selectedAmount, _, selectedErr := coverAllSelectedTotalChecked(overs, choices.Get())
	if totalErr != nil || selectedErr != nil {
		intro = uistate.T("budgets.incomplete")
	}
	canApply := coverAllHasDonor(choices.Get()) && totalErr == nil && selectedErr == nil
	applyLabel := uistate.T("bv7g.cover.applyEmpty")
	if selectedAmount > 0 {
		applyLabel = uistate.T("bv7g.cover.apply", fmtMoney(money.New(selectedAmount, base)))
	}
	showNextMonthHint := coverAllHasNextMonthSelection(choices.Get())

	listArgs := []any{css.Class("cover-all-list")}
	for _, o := range overs {
		oid := o.ID
		listArgs = append(listArgs, ui.CreateElement(coverAllRow, coverAllRowProps{
			Over: o, Sources: sources, Dec: dec, Selected: choices.Get()[oid],
			OnChange: func(v string) { setChoice(oid, v) },
		}))
	}

	return Form(css.Class("acct-edit-form cover-all"), OnSubmit(apply),
		Div(css.Class("modal-scroll"),
			If(empty, P(css.Class("empty cover-all-empty"), Attr("data-testid", "cover-all-empty"), uistate.T("coverAll.none"))),
			If(!empty, Fragment(
				P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
					Attr("data-testid", "cover-all-intro"), intro),
				P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cover-all-impact"),
					Style(map[string]string{"margin": "0"}), fundsImpactLine(budgeting.ImpactCover, fundsImpactOption{
						reach: impactReach, movesFromSelectedBudgets: movesFromSelectedBudgets,
					})),
				Div(listArgs...),
				If(showNextMonthHint, P(css.Class("t-caption", tw.TextDim, "cover-all-hint"), uistate.T("bv6g.cover.nextMonthHint"))),
				If(currencyError, P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "cover-all-currency-error"), uistate.T("coverAll.currencyError"))),
				If(errText.Get() != "", P(css.Class("err"), Attr("role", "alert"), errText.Get())),
			)),
		),
		Div(css.Class("modal-foot"),
			IfElse(empty,
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "cover-all-close"), OnClick(onCancel), uistate.T("action.close")),
				Fragment(
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "cover-all-cancel"), OnClick(onCancel), uistate.T("coverAll.cancel")),
					buttonWithDisabled(checkpointSaving.Get() || !canApply || currencyError, []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "cover-all-apply")}, IfElse(checkpointSaving.Get(), Text(uistate.T("enc34.saving")), Text(applyLabel))),
				)),
		),
	)
}

type coverAllRowProps struct {
	Over     coverAllOver
	Sources  []coverAllSource
	Dec      int
	Selected string
	OnChange func(string)
}

// coverAllRow is one over-budget line: its name + overage, and a source picker. Own
// component so the select's change hook sits at a stable position (never in a loop).
func coverAllRow(props coverAllRowProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) { props.OnChange(e.GetValue()) })
	o := props.Over
	opts := []any{css.Class("field cover-all-src"), Attr("data-testid", "cover-all-src-"+o.ID),
		Attr("aria-label", uistate.T("coverAll.sourceLabel")), OnChange(onChange)}
	opts = append(opts,
		Option(Value(""), SelectedIf(props.Selected == ""), uistate.T("coverAll.sourceSkip")),
		If(!o.Rollover, Option(Value(coverAllNextSource), SelectedIf(props.Selected == coverAllNextSource), uistate.T("coverAll.sourceNextMonth"))))
	for _, s := range coverAllSourcesFor(o, props.Sources) {
		opts = append(opts, Option(Value(s.ID), SelectedIf(props.Selected == s.ID),
			uistate.T("bv7g.cover.sourceBudget", s.Name, fmtMoney(s.Avail), fmtMoney(s.Monthly))))
	}
	return Div(css.Class("cover-all-row"), Attr("data-testid", "cover-all-row"),
		Div(css.Class("cover-all-row-main"),
			Span(css.Class("cover-all-row-name"), o.Name),
			Span(css.Class("cover-all-row-over", tw.TextDown), uistate.T("coverAll.overBy", fmtMoney(o.Shortfall)))),
		Div(css.Class("cover-all-row-src"),
			Span(css.Class("cover-all-row-srclabel", tw.Text12, tw.TextDim), uistate.T("coverAll.sourceLabel")),
			Select(opts...)))
}
