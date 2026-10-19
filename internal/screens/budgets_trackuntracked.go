// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetview"
	"github.com/monstercameron/CashFlux/internal/currency"
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

const trackScanMonths = budgetview.TrackScanMonths

type trackScanMemo struct {
	Candidates []untracked.Candidate
	Err        error
}

// trackRow is one resolved row: the scanned candidate plus the user's edits.
type trackRow = budgetview.TrackRow

// parseTrackRows decodes the per-row edit atom. Unknown categories are ignored, so
// a stale entry (a category tracked since the sheet was last open) cannot resurrect
// a row that no longer exists.
func parseTrackRows(s string) map[string]trackRow {
	return budgetview.ParseTrackRows(s)
}

// putTrackRow replaces one category's entry in the encoded atom value.
func putTrackRow(encoded, catID string, r trackRow) string {
	return budgetview.PutTrackRow(encoded, catID, r)
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
	dataRevision := uistate.UseDataRevision().Get()
	app := appstate.Default
	openAtom := uistate.UseTrackUntrackedOpen()
	rowsAtom := uistate.UseTrackUntrackedRows()

	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()

	closeSheet := ui.UseEvent(Prevent(func() { openAtom.Set(false) }))
	var applyFn func()
	closeOnClick := OnClick(closeSheet)

	base := budgetBaseCurrency("")
	rates := currency.Rates{Base: base}
	appRevision := uint64(0)
	if app != nil {
		settings := app.Settings()
		if settings.BaseCurrency != "" {
			base = settings.BaseCurrency
		}
		rates = currency.Rates{Base: base, Rates: settings.FXRates}
		appRevision = app.Rev()
	}
	wStart, wEnd := vw.Range()
	from := budgetview.TrackScanRange(wEnd)
	payAnchor := budgetview.ParsePayAnchor(pr.PayCycleAnchor)
	scan := ui.UseMemo(func() trackScanMemo {
		if app == nil {
			return trackScanMemo{}
		}
		candidates, err := budgetview.ScanUntracked(app.Transactions(), budgetLedgerForApp(app), app.Categories(), app.Budgets(), wStart, wEnd, rates, base, pr.WeekStartWeekday(), payAnchor)
		return trackScanMemo{Candidates: candidates, Err: err}
	}, app, appRevision, dataRevision, from.Unix(), wStart.Unix(), wEnd.Unix(), base, pr.WeekStartWeekday(), payAnchor.Format("2006-01-02"))
	if app == nil {
		return Fragment()
	}
	v := computeBudgetView(app, activeMemberID, vw, pr, false)
	base = v.Base
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	if scan.Err != nil {
		return Div(css.Class("track-sheet"),
			P(css.Class("budget-sub is-error"), Attr("role", "status"), uistate.T("track.scanFailed")),
			Div(css.Class("modal-foot"), Button(css.Class("btn"), Type("button"), closeOnClick, uistate.T("action.close"))))
	}
	cands := scan.Candidates
	if len(cands) == 0 {
		return Div(css.Class("track-sheet"),
			P(css.Class("budget-sub"), Attr("data-testid", "track-empty"), uistate.T("track.empty")),
			Div(css.Class("modal-foot"),
				Button(css.Class("btn"), Type("button"), closeOnClick, uistate.T("action.close"))))
	}

	rows := budgetview.ResolveTrackRows(cands, parseTrackRows(rowsAtom.Get()))
	dests := budgetview.VisibleDestinations(app.Budgets(), activeMemberID)
	preview, err := budgetview.PreviewTrack(rows, v.BannerIncome, v.RolledOver, v.TotalLimit, v.SavingsAssigned, v.Statuses, dests, v.CatName)
	if err != nil {
		return trackUntrackedCalculationFailure(closeOnClick)
	}
	applyFn = func() {
		if err := applyTrackUntracked(app, cands, base, rates, activeMemberID, wStart, rowsAtom.Get, rowsAtom.Set, func() { openAtom.Set(false) }); err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
		}
	}

	return ui.CreateElement(trackUntrackedContent, trackUntrackedContentProps{
		Rows: rows, Dests: dests, CatNames: v.CatName, Preview: preview, Base: base, Method: v.Method,
		Close: func() { openAtom.Set(false) }, Apply: applyFn,
		Change: func(catID string, next trackRow) { rowsAtom.Set(putTrackRow(rowsAtom.Get(), catID, next)) },
	})
}

type trackUntrackedContentProps struct {
	Rows         []trackRow
	Dests        []domain.Budget
	CatNames     map[string]string
	Change       func(string, trackRow)
	Preview      budgetview.TrackPreview
	Base         string
	Method       budgeting.Methodology
	Close, Apply func()
}

func trackUntrackedContent(props trackUntrackedContentProps) ui.Node {
	effect, base := props.Preview.Effect, props.Base
	presentation := budgetview.PresentTrackFooter(props.Preview, base, props.Method, uistate.T)
	closeOnClick := OnClick(Prevent(props.Close))
	applyOnClick := OnClick(Prevent(props.Apply))
	toAssign := Fragment()
	if presentation.ToAssign != "" {
		toAssign = P(css.Class("track-foot-line"), Attr("data-testid", "track-toassign"), presentation.ToAssign)
	}
	var riskNote ui.Node = Fragment()
	if presentation.Risk != "" {
		riskNote = P(css.Class("track-foot-risk"), Attr("data-testid", "track-risk"), presentation.Risk)
	}

	body := renderTrackUntrackedRows(props.Rows, base, props.Dests, props.CatNames, props.Change)
	return renderTrackUntrackedSheet(body, effect, base, toAssign, riskNote, closeOnClick, applyOnClick)
}

func trackUntrackedCalculationFailure(closeOnClick any) ui.Node {
	return Div(css.Class("track-sheet"),
		P(css.Class("budget-sub is-error"), Attr("role", "status"), uistate.T("track.calculationFailed")),
		Div(css.Class("modal-foot"), Button(css.Class("btn"), Type("button"), closeOnClick, uistate.T("action.close"))))
}

func renderTrackUntrackedRows(rows []trackRow, base string, dests []domain.Budget, catNames map[string]string, onChange func(string, trackRow)) []ui.Node {
	return MapKeyed(rows,
		func(r trackRow) any { return r.Cand.CategoryID },
		func(r trackRow) ui.Node {
			return ui.CreateElement(trackUntrackedRow, trackUntrackedRowProps{
				Row: r, Base: base, Dests: dests, CatName: catNames,
				OnChange: func(next trackRow) { onChange(r.Cand.CategoryID, next) },
			})
		})
}

func renderTrackUntrackedSheet(body []ui.Node, effect untracked.Effect, base string, toAssign, riskNote ui.Node, closeOnClick, applyOnClick any) ui.Node {
	return Div(css.Class("track-sheet"),
		P(css.Class("budget-sub"), uistate.T("track.intro", trackScanMonths)),
		Div(css.Class("track-rows"), Attr("data-testid", "track-rows"), body),
		Div(css.Class("modal-foot track-foot"),
			Div(css.Class("track-foot-read"),
				P(css.Class("track-foot-line"), Attr("data-testid", "track-summary"),
					uistate.TN("track.summaryOne", "track.summaryMany", effect.Categories,
						fmtMoney(money.New(effect.TrackedMinor, base)), fmtMoney(money.New(effect.AssignedDeltaMinor, base)))),
				toAssign, riskNote),
			Div(css.Class("track-foot-actions"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "track-cancel"), closeOnClick, uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "track-apply"),
					disabledAttr(budgetview.ApplyDisabled(effect)), applyOnClick, uistate.T("track.apply", effect.Categories))),
		),
	)
}

func applyTrackUntracked(app *appstate.App, cands []untracked.Candidate, base string, rates currency.Rates, activeMemberID string, ref time.Time, readRows func() string, clearRows func(string), close func()) error {
	live := parseTrackRows(readRows())
	creates, updates, err := planTrackUntracked(cands, live, app.Budgets(), base, rates, activeMemberID)
	if err != nil {
		return err
	}
	writes := append(updates, creates...)
	n := len(writes)
	if n > 0 {
		if err := app.PutBudgetsAtomicAt(writes, ref); err != nil {
			return fmt.Errorf("track untracked: save budgets: %w", err)
		}
		uistate.PostUndoable(uistate.TN("track.appliedOne", "track.appliedOther", n))
		uistate.RequestPersist()
		uistate.BumpDataRevision()
	}
	clearRows("")
	close()
	return nil
}

func planTrackUntracked(cands []untracked.Candidate, rows map[string]trackRow, budgets []domain.Budget, base string, rates currency.Rates, activeMemberID string) ([]domain.Budget, []domain.Budget, error) {
	return budgetview.PlanTrackUntracked(cands, rows, budgets, base, rates, activeMemberID, id.New)
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

	setInclude := ui.UseEvent(func(e ui.ChangeEvent) {
		props.OnChange(r.WithIncluded(e.IsChecked()))
	})
	toggleRaise := ui.UseEvent(func() { props.OnChange(r.ToggleRaise()) })
	raiseOnChange := OnChange(toggleRaise)
	setAmount := ui.UseEvent(func(s string) {
		props.OnChange(budgetview.TrackAmountEdit(r, s, props.Base))
	})
	setDest := ui.UseEvent(func(s string) { props.OnChange(r.WithDestination(s)) })

	opts := []ui.Node{Option(Value(""), uistate.T("track.destNew"))}
	for _, b := range props.Dests {
		name := budgetview.BudgetTitle(b.Name, props.CatName[b.CategoryID])
		opts = append(opts, Option(Value(b.ID), SelectedIf(b.ID == r.BudgetID), name))
	}

	// The raise toggle only means anything for an existing destination: a new
	// budget is CREATED at the row's amount, so there is no limit to raise.
	presentation := budgetview.PresentTrackRow(r, props.Base, uistate.T)
	var raise ui.Node = Fragment()
	if presentation.ShowRaise {
		raise = Label(css.Class("track-raise"),
			Input(Type("checkbox"), Attr("data-testid", "track-raise-"+c.CategoryID),
				checkedAttr(r.Raise), raiseOnChange),
			Span(uistate.T("track.raise")))
	}

	return Div(ClassStr(presentation.Class),
		Attr("data-testid", "track-row-"+c.CategoryID),
		Label(css.Class("track-row-pick"),
			Input(Type("checkbox"), Attr("data-testid", "track-include-"+c.CategoryID),
				Checked(r.Include), OnChange(setInclude)),
			Span(css.Class("track-row-name"), c.Name)),
		Span(css.Class("track-row-sub"), presentation.Subtitle),
		Div(css.Class("track-row-controls"),
			Input(css.Class("fctrl-input track-row-amt"), Type("text"), Attr("inputmode", "decimal"),
				Attr("data-testid", "track-amount-"+c.CategoryID),
				Attr("aria-label", uistate.T("track.amountAria", c.Name)),
				Value(presentation.Amount), OnInput(setAmount)),
			Select(css.Class("fctrl-select track-row-dest"),
				Attr("data-testid", "track-dest-"+c.CategoryID),
				Attr("aria-label", uistate.T("track.destAria", c.Name)),
				OnChange(setDest), opts),
			raise,
		),
	)
}

// fmtDate renders a candidate's last-seen date in the household's own format.
func fmtDate(t time.Time) string { return budgetview.CandidateDate(t) }
