// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgets_trackuntracked.go is the bulk half of the "Unbudgeted spending" strip:
// one sheet listing every expense category no budget watches, each row handled
// individually, applied in one pass.
//
// The strip's per-chip "Budget this" is a good invitation and stays. What it
// could not do is the thing this sheet exists for:
//
//   - It showed the top FOUR categories and never said there were more.
//   - It scanned only the VIEWED period, so quarterly and yearly obligations were
//     invisible eleven months out of twelve — you could "finish" tidying up in
//     August with a yearly property tax still untracked.
//   - "Budget this" only ever created a NEW budget, while a budget tracks a LIST
//     of categories — so the reading "add this to an existing budget" was
//     legitimate, unoffered, and, next to a Transportation budget whose figure
//     happened to match Auto loans exactly, the more natural one (Cam, 2026-08-31).
//
// Two consequences the per-chip flow hid are stated here BEFORE anything is
// written, and both come from the pure `untracked` package so the sheet can never
// disagree with the arithmetic:
//
//   - Aiming a category at an existing budget makes its spend count while leaving
//     that budget's limit alone, which can flip a healthy budget straight to
//     "over". The raise toggle defaults ON for exactly that reason.
//   - In zero-based, tracking spending pushes To Assign further negative. Making
//     the plan honest makes the headline look worse, and the footer says so.

const trackScanMonths = 12

// trackRow is one resolved row: the scanned candidate plus the user's edits.
type trackRow struct {
	Cand     untracked.Candidate
	Include  bool
	Amount   int64  // minor, base
	BudgetID string // "" = create a new budget
	Raise    bool
}

// parseTrackRows decodes the per-row edit atom. Unknown categories are ignored, so
// a stale entry (a category tracked since the sheet was last open) cannot resurrect
// a row that no longer exists.
func parseTrackRows(s string) map[string]trackRow {
	out := map[string]trackRow{}
	for _, seg := range strings.Split(s, "|") {
		if seg == "" {
			continue
		}
		p := strings.Split(seg, "~")
		if len(p) != 5 {
			continue
		}
		amt, err := strconv.ParseInt(p[2], 10, 64)
		if err != nil {
			continue
		}
		out[p[0]] = trackRow{
			Include: p[1] == "1", Amount: amt, BudgetID: p[3], Raise: p[4] == "1",
		}
	}
	return out
}

func encodeTrackRow(catID string, r trackRow) string {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	return catID + "~" + b(r.Include) + "~" + strconv.FormatInt(r.Amount, 10) + "~" + r.BudgetID + "~" + b(r.Raise)
}

// putTrackRow replaces one category's entry in the encoded atom value.
func putTrackRow(encoded, catID string, r trackRow) string {
	var keep []string
	for _, seg := range strings.Split(encoded, "|") {
		if seg == "" || strings.HasPrefix(seg, catID+"~") {
			continue
		}
		keep = append(keep, seg)
	}
	return strings.Join(append(keep, encodeTrackRow(catID, r)), "|")
}

// budgetsTrackUntrackedModal renders the sheet when its atom is set, as a
// shell-root sibling of the bento so no tile transform clips it.
func budgetsTrackUntrackedModal() ui.Node {
	open := uistate.UseTrackUntrackedOpen()
	if !open.Get() {
		return Fragment()
	}
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("track.title"),
		Width:     uiw.FlipLargeW,
		Height:    uiw.FlipLargeH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { open.Set(false) },
		Back:      ui.CreateElement(trackUntrackedForm, trackUntrackedFormProps{}),
	})
}

type trackUntrackedFormProps struct{}

// trackUntrackedForm is the sheet body: the scan, one row per category, and a
// footer stating the plan-level consequence before it can be applied.
func trackUntrackedForm(_ trackUntrackedFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	openAtom := uistate.UseTrackUntrackedOpen()
	rowsAtom := uistate.UseTrackUntrackedRows()

	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()

	closeSheet := ui.UseEvent(Prevent(func() { openAtom.Set(false) }))

	if app == nil {
		return Fragment()
	}
	v := computeBudgetView(app, activeMemberID, vw, pr, false)
	base := v.Base
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// TWELVE months back from the viewed period's end, not the viewed period. A
	// tidy-up that only sees this month leaves every non-monthly obligation behind.
	_, wEnd := vw.Range()
	from := wEnd.AddDate(0, -trackScanMonths, 0)

	// The hint: a category's own suggested limit from real history, which beats the
	// window's raw total for anything that does not bill monthly. SuggestLimit
	// returns a per-period figure; window spend is the fallback inside Scan.
	hint := func(catID string) (int64, domain.Period, bool) {
		if sug, err := budgeting.SuggestLimit(catID, app.Transactions(), wEnd, 6, rates); err == nil && sug > 0 {
			return sug, domain.PeriodMonthly, true
		}
		return 0, "", false
	}

	cands := untracked.Scan(app.Transactions(), app.Categories(), app.Budgets(), from, wEnd, rates, base, hint)
	if len(cands) == 0 {
		return Div(css.Class("track-sheet"),
			P(css.Class("budget-sub"), Attr("data-testid", "track-empty"), uistate.T("track.empty")),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), OnClick(closeSheet), uistate.T("action.close"))))
	}

	edits := parseTrackRows(rowsAtom.Get())
	rows := make([]trackRow, 0, len(cands))
	for _, c := range cands {
		r := trackRow{Cand: c, Include: true, Amount: c.SuggestMinor, Raise: true}
		if e, ok := edits[c.CategoryID]; ok {
			r.Include, r.Amount, r.BudgetID, r.Raise = e.Include, e.Amount, e.BudgetID, e.Raise
		}
		rows = append(rows, r)
	}

	// Only budgets this member can see are offerable destinations.
	var dests []domain.Budget
	for _, b := range app.Budgets() {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			dests = append(dests, b)
		}
	}

	var choices []untracked.Choice
	for _, r := range rows {
		if !r.Include {
			continue
		}
		choices = append(choices, untracked.Choice{
			CategoryID: r.Cand.CategoryID, AmountMinor: r.Amount, BudgetID: r.BudgetID, Raise: r.Raise,
		})
	}
	effect := untracked.Impact(choices, v.BannerIncome+v.RolledOver, v.TotalLimit+v.SavingsAssigned)

	spentOf := map[string]int64{}
	limitOf := map[string]int64{}
	for _, st := range v.Statuses {
		spentOf[st.Budget.ID] = st.Spent.Amount
		limitOf[st.Budget.ID] = st.Limit.Amount
	}
	risky := untracked.OverspendRisk(choices,
		func(bid string) int64 { return spentOf[bid] },
		func(bid string) int64 { return limitOf[bid] })
	riskyNames := make([]string, 0, len(risky))
	for _, bid := range risky {
		for _, b := range dests {
			if b.ID == bid {
				riskyNames = append(riskyNames, budgetTitle(b.Name, v.CatName[b.CategoryID]))
			}
		}
	}

	apply := ui.UseEvent(Prevent(func() {
		// Re-resolve at click time so the write uses the state on screen.
		live := parseTrackRows(rowsAtom.Get())
		n := 0
		for _, c := range cands {
			r := trackRow{Include: true, Amount: c.SuggestMinor, Raise: true}
			if e, ok := live[c.CategoryID]; ok {
				r.Include, r.Amount, r.BudgetID, r.Raise = e.Include, e.Amount, e.BudgetID, e.Raise
			}
			if !r.Include || r.Amount <= 0 {
				continue
			}
			if r.BudgetID == "" {
				nb := domain.Budget{
					ID: id.New(), Name: c.Name, CategoryID: c.CategoryID,
					Limit: money.New(r.Amount, base), Period: c.Period,
				}
				if err := app.PutBudget(nb); err == nil {
					n++
				}
				continue
			}
			for _, b := range app.Budgets() {
				if b.ID != r.BudgetID {
					continue
				}
				// Extend the tracked LIST. TrackedCategoryIDs is either/or — a
				// non-empty CategoryIDs replaces CategoryID rather than extending
				// it — so seeding the list from the accessor keeps the budget
				// watching everything it already watched.
				nb := b
				nb.CategoryIDs = append(append([]string(nil), b.TrackedCategoryIDs()...), c.CategoryID)
				if r.Raise {
					nb.Limit = money.New(b.Limit.Amount+r.Amount, b.Limit.Currency)
					if nb.Limit.Currency == "" {
						nb.Limit = money.New(b.Limit.Amount+r.Amount, base)
					}
				}
				if err := app.PutBudget(nb); err == nil {
					n++
				}
				break
			}
		}
		if n > 0 {
			uistate.PostUndoable(uistate.T("track.applied", plural(n, "category")))
			uistate.RequestPersist()
			uistate.BumpDataRevision()
		}
		rowsAtom.Set("")
		openAtom.Set(false)
	}))

	body := MapKeyed(rows,
		func(r trackRow) any { return r.Cand.CategoryID },
		func(r trackRow) ui.Node {
			return ui.CreateElement(trackUntrackedRow, trackUntrackedRowProps{
				Row: r, Base: base, Dests: dests, CatName: v.CatName,
				OnChange: func(next trackRow) { rowsAtom.Set(putTrackRow(rowsAtom.Get(), r.Cand.CategoryID, next)) },
			})
		})

	// The footer is the point of the sheet: what this costs, stated before it is
	// possible to apply. In zero-based the To Assign line is the honest bad news.
	toAssign := Fragment()
	if v.Method == budgeting.MethodZeroBased {
		toAssign = P(css.Class("track-foot-line"), Attr("data-testid", "track-toassign"),
			uistate.T("track.toAssign",
				fmtMoney(money.New(effect.ToAssignBeforeMinor, base)),
				fmtMoney(money.New(effect.ToAssignAfterMinor, base))))
	}
	var riskNote ui.Node = Fragment()
	if len(riskyNames) > 0 {
		riskNote = P(css.Class("track-foot-risk"), Attr("data-testid", "track-risk"),
			uistate.T("track.overspendRisk", strings.Join(riskyNames, ", ")))
	}

	return Div(css.Class("track-sheet"),
		P(css.Class("budget-sub"), uistate.T("track.intro", trackScanMonths)),
		Div(css.Class("track-rows"), Attr("data-testid", "track-rows"), body),
		Div(css.Class("modal-foot track-foot"),
			Div(css.Class("track-foot-read"),
				P(css.Class("track-foot-line"), Attr("data-testid", "track-summary"),
					uistate.TN("track.summaryOne", "track.summaryMany", effect.Categories,
						fmtMoney(money.New(effect.TrackedMinor, base)),
						fmtMoney(money.New(effect.AssignedDeltaMinor, base)))),
				toAssign,
				riskNote,
			),
			Div(css.Class("track-foot-actions"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "track-cancel"),
					OnClick(closeSheet), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "track-apply"),
					disabledAttr(effect.Categories == 0), OnClick(apply),
					uistate.T("track.apply", effect.Categories)),
			),
		),
	)
}

type trackUntrackedRowProps struct {
	Row      trackRow
	Base     string
	Dests    []domain.Budget
	CatName  map[string]string
	OnChange func(trackRow)
}

// trackUntrackedRow is one category's row. Its own component so every On* hook
// sits at a stable call-site — the framework's hardest rule is that handlers must
// never be registered inside a variable-length loop.
func trackUntrackedRow(props trackUntrackedRowProps) ui.Node {
	r := props.Row
	c := r.Cand

	toggleInclude := ui.UseEvent(func() { next := r; next.Include = !r.Include; props.OnChange(next) })
	toggleRaise := ui.UseEvent(func() { next := r; next.Raise = !r.Raise; props.OnChange(next) })
	setAmount := ui.UseEvent(func(s string) {
		next := r
		if amt, err := money.ParseMinor(s, currency.Decimals(props.Base)); err == nil && amt >= 0 {
			next.Amount = amt
		}
		props.OnChange(next)
	})
	setDest := ui.UseEvent(func(s string) { next := r; next.BudgetID = s; props.OnChange(next) })

	opts := []ui.Node{Option(Value(""), uistate.T("track.destNew"))}
	for _, b := range props.Dests {
		name := budgetTitle(b.Name, props.CatName[b.CategoryID])
		opts = append(opts, Option(Value(b.ID), SelectedIf(b.ID == r.BudgetID), name))
	}

	// The raise toggle only means anything for an existing destination: a new
	// budget is CREATED at the row's amount, so there is no limit to raise.
	var raise ui.Node = Fragment()
	if r.BudgetID != "" {
		raise = Label(css.Class("track-raise"),
			Input(Type("checkbox"), Attr("data-testid", "track-raise-"+c.CategoryID),
				checkedAttr(r.Raise), OnChange(toggleRaise)),
			Span(uistate.T("track.raise")))
	}

	sub := uistate.T("track.rowSpent", fmtMoney(money.New(c.SpentMinor, props.Base)), fmtDate(c.LastSeen))
	if c.FromHint {
		sub += " · " + uistate.T("track.fromHistory")
	}

	return Div(ClassStr("track-row"+map[bool]string{true: "", false: " is-off"}[r.Include]),
		Attr("data-testid", "track-row-"+c.CategoryID),
		Label(css.Class("track-row-pick"),
			Input(Type("checkbox"), Attr("data-testid", "track-include-"+c.CategoryID),
				checkedAttr(r.Include), OnChange(toggleInclude)),
			Span(css.Class("track-row-name"), c.Name)),
		Span(css.Class("track-row-sub"), sub),
		Div(css.Class("track-row-controls"),
			Input(css.Class("fctrl-input track-row-amt"), Type("text"), Attr("inputmode", "decimal"),
				Attr("data-testid", "track-amount-"+c.CategoryID),
				Attr("aria-label", uistate.T("track.amountAria", c.Name)),
				Value(money.FormatMinor(r.Amount, currency.Decimals(props.Base))), OnInput(setAmount)),
			Select(css.Class("fctrl-select track-row-dest"),
				Attr("data-testid", "track-dest-"+c.CategoryID),
				Attr("aria-label", uistate.T("track.destAria", c.Name)),
				OnChange(setDest), opts),
			raise,
		),
	)
}

// fmtDate renders a candidate's last-seen date in the household's own format.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return dateutil.FormatDate(t)
}
