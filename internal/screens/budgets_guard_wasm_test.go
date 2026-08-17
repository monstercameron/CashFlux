// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The 2026-08-17 /budgets recheck found the page accepting money it should refuse
// and describing money it should have explained. These mount the real components:
//
//	C665 — a negative limit left Save live and was thrown away in silence.
//	C666 — the blank Add budget form offered an enabled commit button.
//	C667 — quick-fill history came from a different population AND a different
//	       period than the card's spent total.
//	C668 — every "What's driving this?" control had the same accessible name.
//	C670 — the compact-list toggle's name never said which view was on screen.

// guardTestApp installs an appstate with one expense category, so the add form
// takes its normal "pick an existing category" path rather than the forced
// create-one path a category-less household gets.
func guardTestApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	// A parent with a child, because that is the shape C667 was reported on: the
	// budget watches the parent, every charge lands on the child.
	for _, c := range []domain.Category{
		{ID: "c-transport", Name: "Transportation", Kind: domain.KindExpense},
		{ID: "c-gas", Name: "Gas", Kind: domain.KindExpense, ParentID: "c-transport"},
	} {
		if err := app.PutCategory(c); err != nil {
			t.Fatalf("PutCategory(%q): %v", c.Name, err)
		}
	}
	prev := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = prev })
	return app
}

// byTestID finds the single rendered node carrying data-testid=id, or nil.
func byTestID(f *render.Fixture, tag, id string) *render.QueryNode {
	for _, n := range f.AllByTag(tag) {
		if n.Attr("data-testid") == id {
			return n
		}
	}
	return nil
}

// isDisabled reports whether a commit control is actually dead — the property is
// what the browser honours, the attribute is what a static render carries.
func isDisabled(n *render.QueryNode) bool {
	if n == nil {
		return false
	}
	if v, ok := n.Property("disabled").(bool); ok && v {
		return true
	}
	return n.Attr("disabled") != ""
}

// ─── C666: the blank Add budget form ─────────────────────────────────────────

// TestAddBudgetBlockedUntilNameAndLimit is the C666 guard: opening the form with
// nothing typed must not offer a live commit, and it must say what it is waiting
// for rather than leaving the button to be pressed to find out.
func TestAddBudgetBlockedUntilNameAndLimit(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{}))

	submit := byTestID(f, "button", "budget-add-submit")
	if submit == nil {
		t.Fatal("Add budget button not rendered")
	}
	if !isDisabled(submit) {
		t.Error("Add budget is enabled on a blank form — C666 has regressed")
	}
	blockers := byTestID(f, "span", "budget-add-blockers")
	if blockers == nil || blockers.Text() == "" {
		t.Fatal("the blank form does not say what it still needs")
	}
	if want := uistate.T("budgets.addNeedsBoth"); blockers.Text() != want {
		t.Errorf("blocker line = %q, want %q", blockers.Text(), want)
	}
}

// TestAddBudgetRejectsNonPositiveLimit is the other half of C666 (and the add
// form's share of C665): a name alone is not a valid draft, and a zero or
// negative limit is refused under the field rather than on commit.
func TestAddBudgetRejectsNonPositiveLimit(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetAddForm, BudgetAddFormProps{}))

	f.InputByID("budget-add", "Transportation")
	f.Stabilize()
	if blockers := byTestID(f, "span", "budget-add-blockers"); blockers == nil ||
		blockers.Text() != uistate.T("budgets.addNeedsLimit") {
		t.Errorf("with a name but no limit, the form should ask for the limit; got %v", blockers)
	}

	limitField := byTestID(f, "input", "budget-add-limit")
	if limitField == nil {
		t.Fatal("the limit field did not render")
	}
	for _, bad := range []string{"-1", "0"} {
		limitField.Input(bad)
		f.Stabilize()
		if err := f.ByID("budget-limit-err"); !err.Exists() || err.Text() == "" {
			t.Errorf("limit %q produced no field-level error", bad)
		}
		if !isDisabled(byTestID(f, "button", "budget-add-submit")) {
			t.Errorf("limit %q left Add budget enabled — C665/C666 regression", bad)
		}
	}

	limitField.Input("1300.00")
	f.Stabilize()
	if err := f.ByID("budget-limit-err"); err.Exists() && err.Text() != "" {
		t.Errorf("a valid limit still shows an error: %q", err.Text())
	}
	if isDisabled(byTestID(f, "button", "budget-add-submit")) {
		t.Error("a complete draft still leaves Add budget disabled")
	}
	if b := byTestID(f, "span", "budget-add-blockers"); b != nil && b.Text() != "" {
		t.Errorf("a complete draft still lists blockers: %q", b.Text())
	}
}

// ─── C665: the full budget editor ────────────────────────────────────────────

// TestBudgetEditRejectsNegativeLimit is the C665 guard on the editor the report
// was filed against: typing -1 must kill Save and say why, not leave the commit
// live for the handler to refuse in silence.
func TestBudgetEditRejectsNegativeLimit(t *testing.T) {
	app := guardTestApp(t)
	if err := app.PutBudget(domain.Budget{
		ID: "b-transport", Name: "Transportation", CategoryID: "c-transport",
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Period: domain.PeriodMonthly, Limit: money.New(130000, "USD"),
	}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetEditForm, BudgetEditFormProps{
		BudgetID: "b-transport", Mode: uistate.BudgetEditModeEdit,
	}))

	save := byTestID(f, "button", "budget-edit-save")
	if save == nil {
		t.Fatal("the editor rendered no Save button")
	}
	if isDisabled(save) {
		t.Fatal("Save is disabled on an untouched, valid budget")
	}

	limitField := byTestID(f, "input", "budget-edit-limit")
	if limitField == nil {
		t.Fatal("the limit field did not render")
	}
	limitField.Input("-1")
	f.Stabilize()

	if !isDisabled(byTestID(f, "button", "budget-edit-save")) {
		t.Error("a negative limit left Save enabled — C665 has regressed")
	}
	errNode := f.ByID("budget-edit-limit-err")
	if !errNode.Exists() || errNode.Text() == "" {
		t.Fatal("a negative limit produced no inline error")
	}
	if !strings.Contains(errNode.Text(), fmtMoney(money.Zero("USD"))) {
		t.Errorf("the error should name the floor it wants beaten; got %q", errNode.Text())
	}
	// The stored budget must be untouched — refusing to save is the point.
	for _, b := range app.Budgets() {
		if b.ID == "b-transport" && b.Limit.Amount != 130000 {
			t.Errorf("the budget's limit changed to %d while the draft was invalid", b.Limit.Amount)
		}
	}
}

// TestInlineLimitEditorRejectsNegative is the C665 guard on the OTHER limit
// editor — the one the report was filed against, which advertises min="0.01" and
// honoured it nowhere. Typing -1 used to leave ✓ live, and pressing it closed the
// editor without saving and without a word, so a refused edit and a saved one
// looked identical.
func TestInlineLimitEditorRejectsNegative(t *testing.T) {
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	if err := app.PutBudget(b); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(BudgetRow, budgetRowProps{
		Status: budgeting.Status{
			Budget: b, Limit: b.Limit, Spent: money.Zero("USD"), Remaining: b.Limit,
		},
		Category: "Transportation",
		OnDelete: func(string) {},
	}))

	// Click the limit figure to open the inline editor.
	open := byTestID(f, "button", "budget-limit-btn-"+b.ID)
	if open == nil {
		t.Fatal("the limit figure is not an edit affordance")
	}
	open.Click()
	f.Stabilize()

	field := byTestID(f, "input", "budget-limit-input-"+b.ID)
	if field == nil {
		t.Fatal("the inline limit editor did not open")
	}
	field.Input("-1")
	f.Stabilize()

	save := byTestID(f, "button", "budget-limit-save-"+b.ID)
	if save == nil {
		t.Fatal("the inline editor has no save control")
	}
	if !isDisabled(save) {
		t.Error("a negative limit left ✓ live — C665 has regressed on the card editor")
	}
	if err := f.ByID("budget-limit-err-" + b.ID); !err.Exists() || err.Text() == "" {
		t.Error("a negative limit produced no inline reason")
	}
	// Submitting anyway (Enter in the field routes here, past the dead button)
	// must neither write nor silently close: the typed text has to survive so it
	// can be corrected.
	var editForm *render.QueryNode
	for _, n := range f.AllByTag("form") {
		if strings.Contains(n.Attr("class"), "budget-limit-editform") {
			editForm = n
		}
	}
	if editForm == nil {
		t.Fatal("the inline editor is not a form, so Enter cannot be intercepted")
	}
	editForm.Submit()
	f.Stabilize()
	if byTestID(f, "input", "budget-limit-input-"+b.ID) == nil {
		t.Error("the editor closed on a refused value, discarding the edit in silence")
	}
	for _, stored := range app.Budgets() {
		if stored.ID == b.ID && stored.Limit.Amount != 130000 {
			t.Errorf("the stored limit changed to %d — a negative limit was persisted", stored.Limit.Amount)
		}
	}
}

// ─── C669: the top-up dialog's arithmetic ────────────────────────────────────

// TestTopupNamesBaseLimitAndEffectiveCap is the C669 guard: the dialog must show
// the base limit, the carry/boost arithmetic and the resulting cap, and say which
// of the two figures the typed amount lands on. Exercised through the copy builder
// rather than a mount — the dialog's duration control is a Segmented, whose effect
// reaches for a real DOM node the fixture does not provide.
func TestTopupNamesBaseLimitAndEffectiveCap(t *testing.T) {
	base := fmtMoney(money.New(50000, "USD"))
	effCap := fmtMoney(money.New(72171, "USD"))
	capMath := uistate.T("budgets.capMathLimit", base) + " " +
		uistate.T("budgets.capMathCarryPlus", fmtMoney(money.New(22171, "USD")))

	// Rollover in play: the arithmetic and BOTH figures have to appear, and the
	// this-period line must say the base limit is left alone.
	capLine, changes := budgetTopupCapCopy(base, effCap, capMath, false)
	for _, want := range []string{base, effCap} {
		if !strings.Contains(capLine, want) {
			t.Errorf("cap line %q omits %s", capLine, want)
		}
	}
	if !strings.Contains(capLine, capMath) {
		t.Errorf("cap line %q omits the arithmetic that produced the cap", capLine)
	}
	if !strings.Contains(changes, effCap) || !strings.Contains(changes, base) {
		t.Errorf("the this-period line must name both the cap it raises and the base limit it leaves; got %q", changes)
	}

	// Permanent: the amount lands on the base limit, so that is the figure named.
	_, permChanges := budgetTopupCapCopy(base, effCap, capMath, true)
	if !strings.Contains(permChanges, base) {
		t.Errorf("the permanent line does not name the base limit it raises; got %q", permChanges)
	}
	if strings.Contains(permChanges, effCap) {
		t.Errorf("the permanent line names the effective cap, which it does not change; got %q", permChanges)
	}

	// No rollover and no boost: one figure, said once, still explicitly.
	plainCap, plainChanges := budgetTopupCapCopy(base, "", "", false)
	if !strings.Contains(plainCap, base) {
		t.Errorf("the plain cap line omits the base limit; got %q", plainCap)
	}
	if !strings.Contains(plainChanges, base) {
		t.Errorf("the plain this-period line omits the base limit; got %q", plainChanges)
	}
}

// ─── C667: quick-fill history follows the card ───────────────────────────────

// quickFillFixtureBudget returns the budget the C667 tests edit: a parent
// category whose spend all lands in sub-categories, which is the shape that
// produced "$0.00 of history" beside a four-figure spent total.
func quickFillFixtureBudget() domain.Budget {
	return domain.Budget{
		ID: "b-transport", Name: "Transportation", CategoryID: "c-transport",
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Period: domain.PeriodMonthly, Limit: money.New(130000, "USD"),
	}
}

// TestQuickFillRowWalksBackFromItsAnchor is the time-axis half of C667: the
// chips describe history behind the period being VIEWED, so the caption's window
// has to move with the anchor rather than sit on the wall clock.
func TestQuickFillRowWalksBackFromItsAnchor(t *testing.T) {
	guardTestApp(t)
	b := quickFillFixtureBudget()
	pr := uistate.LoadPrefs()

	// The render fixture is process-global on js/wasm, so each anchor gets its own
	// fixture and releases it before the next one is taken.
	explainFor := func(anchor time.Time) string {
		f := render.New(t)
		defer f.Cleanup()
		f.Render(budgetQuickFillRow(appstate.Default, b, budgeting.Status{Budget: b}, anchor,
			budgetTargetDraft{Decimals: 2, Currency: "USD"}, func(string) {}))
		node := byTestID(f, "span", "budget-quickfill-explain")
		if node == nil {
			t.Fatal("the quick-fill row states no window")
		}
		return node.Text()
	}

	// March 2026: the six whole months behind it are Sep 2025 .. Feb 2026.
	march := explainFor(time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC))
	for _, want := range []string{
		pr.FormatDate(time.Date(2025, time.September, 1, 0, 0, 0, 0, time.UTC)),
		pr.FormatDate(time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)),
	} {
		if !strings.Contains(march, want) {
			t.Errorf("the March view's window omits %s; got %q", want, march)
		}
	}

	// August 2026: a different anchor must produce a different window, or the
	// row is quoting the clock rather than the view.
	august := explainFor(time.Date(2026, time.August, 15, 0, 0, 0, 0, time.UTC))
	if august == march {
		t.Fatalf("two different anchors produced the same window: %q", march)
	}
	if want := pr.FormatDate(time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)); !strings.Contains(august, want) {
		t.Errorf("the August view's window omits %s; got %q", want, august)
	}
}

// TestQuickFillRowCountsSubcategorySpend is the category-axis half of C667, at
// the seam where it actually broke: the row has to hand the pure layer the SAME
// rollup set the card's bar counts. Without it a parent-category budget whose
// charges all sit in sub-categories is offered a history of zero beside a card
// reporting four figures — the reported symptom exactly.
func TestQuickFillRowCountsSubcategorySpend(t *testing.T) {
	app := guardTestApp(t)
	b := quickFillFixtureBudget()
	// Every charge lands on the CHILD category, in the month before the anchor.
	if err := app.PutAccount(domain.Account{
		ID: "a-1", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", BalanceAsOf: time.Now(),
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "t-gas", AccountID: "a-1", Desc: "Fuel", CategoryID: "c-gas",
		Date:   time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC),
		Amount: money.New(-90000, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	f := render.New(t)
	f.Render(budgetQuickFillRow(app, b, budgeting.Status{Budget: b},
		time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC),
		budgetTargetDraft{Decimals: 2, Currency: "USD"}, func(string) {}))

	chip := byTestID(f, "button", "budget-fill-"+budgeting.QuickFillLastPeriod)
	if chip == nil {
		t.Fatal("the last-period-spend chip did not render")
	}
	want := fmtMoney(money.New(90000, "USD"))
	if !strings.Contains(chip.Text(), want) {
		t.Errorf("last-period spend chip = %q, want %s — the sub-category spend is not being rolled up", chip.Text(), want)
	}
	// And the chip must say it is spend, not a plan, so it can't be confused with
	// the prior-limit chip sitting beside it.
	if name := chip.Name(); !strings.Contains(name, uistate.T("budgets.fillKindSpend")) {
		t.Errorf("chip accessible name = %q — it must say the figure is actual spend", name)
	}
	prior := byTestID(f, "button", "budget-fill-"+budgeting.QuickFillPriorLimit)
	if prior == nil {
		t.Fatal("the prior-limit chip did not render")
	}
	if name := prior.Name(); !strings.Contains(name, uistate.T("budgets.fillKindLimit")) {
		t.Errorf("prior-limit chip accessible name = %q — it must not read as spending", name)
	}
}

// seedViewedPeriod points the dashboard window atom at the month containing when.
//
// The id is the one uistate.UsePeriod() addresses. If it ever drifts the seed
// stops applying silently — the callers below catch that, because the caption
// would then describe today's window instead of the seeded one.
func seedViewedPeriod(when time.Time) {
	state.NewGlobalAtom("dashboard:period", period.Window{}).
		Set(period.NewWindow(period.Month, when, uistate.LoadPrefs().WeekStartWeekday()))
}

// TestBudgetEditQuickFillUsesTheViewedPeriod is the wiring guard for the same
// defect: the editor must hand the row the VIEW's anchor, not time.Now(). Paging
// back to a past period and opening the editor was offering history counted from
// today beside a card reporting the past period — C667 arriving down the time
// axis after its category axis was fixed.
func TestBudgetEditQuickFillUsesTheViewedPeriod(t *testing.T) {
	app := guardTestApp(t)
	if err := app.PutBudget(quickFillFixtureBudget()); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	pr := uistate.LoadPrefs()
	// Derived from the clock, not hardcoded: a fixed calendar date would stop
	// discriminating in the month it names, and the assertion has to keep its
	// teeth on every day this test is ever run.
	viewedStart := dateutil.MonthStart(dateutil.AddMonths(time.Now(), -6))

	f := render.New(t)
	seedViewedPeriod(viewedStart)
	f.Render(ui.CreateElement(BudgetEditForm, BudgetEditFormProps{
		BudgetID: "b-transport", Mode: uistate.BudgetEditModeEdit,
	}))

	node := byTestID(f, "span", "budget-quickfill-explain")
	if node == nil {
		t.Fatal("the editor's quick-fill row states no window")
	}
	// Viewing a past month, the history behind it ends the day before that month.
	if want := pr.FormatDate(viewedStart.AddDate(0, 0, -1)); !strings.Contains(node.Text(), want) {
		t.Errorf("the editor quoted a window ending elsewhere than %s — it is walking back from the clock, not the view; got %q", want, node.Text())
	}
	// The prior-limit chip must still be flagged as a plan rather than spending.
	if note := byTestID(f, "span", "budget-quickfill-limit-note"); note == nil || note.Text() == "" {
		t.Error("nothing distinguishes the prior-limit chip from the spend chips")
	}
}

// TestBudgetEditQuickFillAnchorsToTodayInTheCurrentView guards the OTHER branch
// of budgetViewAnchor, which the past-period test above cannot see.
//
// When the viewed window contains today, the anchor is today — not the window's
// start. For a monthly budget the two coincide, so the distinction only shows on
// a budget whose cadence is finer than the view: a WEEKLY budget viewed in the
// current month must offer the week behind TODAY, not the week behind the 1st.
// Substituting `vw.From` for the anchor is a silent regression everywhere else.
//
// On the handful of days where today's week already contains the 1st, the two
// anchors coincide and this test cannot tell them apart; the anchor function's
// own test below covers the branch unconditionally.
func TestBudgetEditQuickFillAnchorsToTodayInTheCurrentView(t *testing.T) {
	app := guardTestApp(t)
	weekly := quickFillFixtureBudget()
	weekly.ID, weekly.Name, weekly.Period = "b-weekly", "Coffee", domain.PeriodWeekly
	if err := app.PutBudget(weekly); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	pr := uistate.LoadPrefs()
	now := time.Now()

	f := render.New(t)
	seedViewedPeriod(now) // the current month — so budgetViewAnchor must return today
	f.Render(ui.CreateElement(BudgetEditForm, BudgetEditFormProps{
		BudgetID: "b-weekly", Mode: uistate.BudgetEditModeEdit,
	}))

	node := byTestID(f, "span", "budget-quickfill-explain")
	if node == nil {
		t.Fatal("the editor's quick-fill row states no window")
	}
	// Whole weeks behind the week containing today: the window ends the day
	// before this week starts.
	thisWeek := dateutil.WeekStart(now, pr.WeekStartWeekday())
	if want := pr.FormatDate(thisWeek.AddDate(0, 0, -1)); !strings.Contains(node.Text(), want) {
		t.Errorf("viewing the current month, the window should end %s (the day before this week) — got %q", want, node.Text())
	}
}

// TestBudgetViewAnchorPicksTodayOnlyInsideTheWindow pins budgetViewAnchor itself,
// the function both the card and the editor derive their anchor from. It had no
// test of its own, so either branch could be dropped without a failure.
func TestBudgetViewAnchorPicksTodayOnlyInsideTheWindow(t *testing.T) {
	now := time.Date(2026, time.August, 17, 9, 30, 0, 0, time.UTC)
	weekStart := time.Sunday

	// The window containing now: the anchor is now, so period math lands in the
	// part of the window that has actually happened.
	current := period.NewWindow(period.Month, now, weekStart)
	if got := budgetViewAnchor(current, now); !got.Equal(now) {
		t.Errorf("anchor inside the viewed window = %v, want today (%v)", got, now)
	}

	// A window that does not contain now: the anchor is its start, because
	// "today" is not a moment that window describes.
	past := period.NewWindow(period.Month, dateutil.AddMonths(now, -3), weekStart)
	from, _ := past.Range()
	if got := budgetViewAnchor(past, now); !got.Equal(from) {
		t.Errorf("anchor outside the viewed window = %v, want the window start (%v)", got, from)
	}
	future := period.NewWindow(period.Month, dateutil.AddMonths(now, 3), weekStart)
	from, _ = future.Range()
	if got := budgetViewAnchor(future, now); !got.Equal(from) {
		t.Errorf("anchor ahead of the viewed window = %v, want the window start (%v)", got, from)
	}
}

// ─── C668: driver controls scoped by budget ──────────────────────────────────

// TestDriversToggleNamesItsBudget is the C668 guard: several budgets can offer
// this control at once, so its accessible name has to identify which one.
func TestDriversToggleNamesItsBudget(t *testing.T) {
	guardTestApp(t)
	f := render.New(t)
	f.Render(ui.CreateElement(budgetDriversPanel, budgetDriversPanelProps{
		Budget: domain.Budget{ID: "b-transport", Name: "Transportation"},
		Title:  "Transportation",
	}))

	toggle := byTestID(f, "button", "budget-drivers-toggle-b-transport")
	if toggle == nil {
		t.Fatal("the drivers toggle did not render")
	}
	name := toggle.Name()
	if !strings.Contains(name, "Transportation") {
		t.Errorf("accessible name = %q — it must identify the budget it expands", name)
	}
	if !strings.Contains(name, uistate.T("budgets.driversShow")) {
		t.Errorf("accessible name = %q — it must still say what the control does", name)
	}
	if got := toggle.Attr("aria-controls"); got != "budget-drivers-list-b-transport" {
		t.Errorf("aria-controls = %q, want the panel's own region", got)
	}
}

// ─── C670: the compact-list toggle ───────────────────────────────────────────

// TestDensityStateKeyTracksTheView is the C670 guard: the screen-reader suffix
// must name the CURRENT view and what a click produces, in both states, while the
// visible label and aria-pressed stay as C596 left them.
func TestDensityStateKeyTracksTheView(t *testing.T) {
	on := uistate.T(budgetDensityStateKey(uistate.BudgetDensityCompact))
	off := uistate.T(budgetDensityStateKey(""))
	if on == off {
		t.Fatalf("both density states read the same: %q", on)
	}
	compact := strings.ToLower(uistate.T("budgets.densityCompact"))
	// Off: the current view is cards, and a click produces the compact list.
	if !strings.Contains(off, "currently off") || !strings.Contains(off, compact) {
		t.Errorf("the off-state suffix must say it is off and promise the compact list; got %q", off)
	}
	// On: the current view is the compact list, and a click brings the cards back.
	if !strings.Contains(on, "currently on") || !strings.Contains(on, "full cards") {
		t.Errorf("the on-state suffix must say it is on and promise full cards; got %q", on)
	}
}

// ─── C671: the reconcile action's reach ──────────────────────────────────────

// TestReconcileDisclosesItsScopeBeforeOpening is the C671 guard on the
// originating action: "Bring the plan down to what arrived" read as a fix for one
// underfunded month and pre-filled a permanent rewrite of every budget, which the
// user learned only after the form was open.
func TestReconcileDisclosesItsScopeBeforeOpening(t *testing.T) {
	f := render.New(t)
	// Assigned well past what arrived, so the callout appears with a real cut.
	f.Render(ui.CreateElement(budgetFundedCallout, budgetFundedProps{
		Funding:       budgeting.FundingRead{Expected: 1000000, Received: 600000, Assigned: 1000000},
		Base:          "USD",
		ScalableCount: 12,
	}))

	btn := byTestID(f, "button", "budgets-funded-reconcile")
	if btn == nil {
		t.Fatal("the reconcile action did not render")
	}
	// The label itself has to carry the reach; a reader who never opens the form
	// should not be able to mistake it for a permanent plan change.
	if name := btn.Name(); !strings.Contains(strings.ToLower(name), "this period") {
		t.Errorf("reconcile label = %q — it must say which periods it changes", name)
	}
	summary := byTestID(f, "span", "budgets-funded-reconcile-summary")
	if summary == nil || summary.Text() == "" {
		t.Fatal("the action states neither its size nor its reach before it is pressed")
	}
	for _, want := range []string{"12", "this period"} {
		if !strings.Contains(strings.ToLower(summary.Text()), want) {
			t.Errorf("the summary omits %q; got %q", want, summary.Text())
		}
	}
}

// TestReconcileSeedsThisPeriodScope pins the handover: the button promises this
// period, so the form it opens must OPEN on this period. A seed that carried only
// a percentage is what let the promise and the pre-filled operation disagree.
func TestReconcileSeedsThisPeriodScope(t *testing.T) {
	t.Cleanup(func() { uistate.TakeBudgetAdjustSeed() })
	f := render.New(t)
	f.Render(ui.CreateElement(budgetFundedCallout, budgetFundedProps{
		Funding:       budgeting.FundingRead{Expected: 1000000, Received: 600000, Assigned: 1000000},
		Base:          "USD",
		ScalableCount: 12,
	}))
	btn := byTestID(f, "button", "budgets-funded-reconcile")
	if btn == nil {
		t.Fatal("the reconcile action did not render")
	}
	btn.Click()
	f.Stabilize()

	pct, scope := uistate.TakeBudgetAdjustSeed()
	if scope != string(budgeting.AdjustThisPeriod) {
		t.Errorf("seeded scope = %q, want %q — the form would open on a reach the button never mentioned",
			scope, budgeting.AdjustThisPeriod)
	}
	if pct == "" || !strings.HasPrefix(pct, "-") {
		t.Errorf("seeded percentage = %q, want a reduction", pct)
	}
	// And the form must honour it rather than fall back to its own default.
	if got := defaultAdjustScope(scope); got != string(budgeting.AdjustThisPeriod) {
		t.Errorf("the form resolved the seeded scope to %q, want %q", got, budgeting.AdjustThisPeriod)
	}
}

// TestAdjustScopeCopyTracksTheScope pins the three moments that have to agree on
// how long a change lasts: the hint under the control, the commit button, and the
// undo banner. The toolbar's own default is unchanged — only the seeded path moves.
func TestAdjustScopeCopyTracksTheScope(t *testing.T) {
	if got := defaultAdjustScope(""); got != string(budgeting.AdjustEveryPeriod) {
		t.Errorf("an unseeded open resolved to %q — the toolbar's historical default must not move", got)
	}
	if got := defaultAdjustScope("nonsense"); got != string(budgeting.AdjustEveryPeriod) {
		t.Errorf("an unparseable seed resolved to %q, want the safe default", got)
	}

	this := budgeting.AdjustThisPeriod
	every := budgeting.AdjustEveryPeriod
	for _, pair := range []struct{ a, b string }{
		{adjustScopeHintKey(this), adjustScopeHintKey(every)},
		{adjustApplyKey(this), adjustApplyKey(every)},
		{adjustAppliedKey(this), adjustAppliedKey(every)},
	} {
		if pair.a == pair.b {
			t.Errorf("both scopes resolve to the same copy key %q", pair.a)
		}
		if uistate.T(pair.a) == pair.a || uistate.T(pair.b) == pair.b {
			t.Errorf("missing catalog entry for %q / %q", pair.a, pair.b)
		}
	}
	// The permanent hint must name the reach; the this-period one must promise the
	// revert. Those are the two facts the ticket says were missing.
	// It must say the change is unbounded in time. It must NOT say the change
	// begins at this period: a limit has no history, so past periods report
	// against the new number too.
	h := strings.ToLower(uistate.T(adjustScopeHintKey(every)))
	if !strings.Contains(h, "every period") {
		t.Errorf("the permanent hint does not say it reaches every period; got %q", h)
	}
	if strings.Contains(h, "every period after") {
		t.Errorf("the permanent hint draws a future-only boundary the data model does not have; got %q", h)
	}
	if h := strings.ToLower(uistate.T(adjustScopeHintKey(this))); !strings.Contains(h, "next period") {
		t.Errorf("the this-period hint does not promise the revert; got %q", h)
	}
}

// TestReconcileOnAClosedPeriodSaysSo is the C671 disclosure guard for a past
// view. "This period" is honest but incomplete once the period has ended: the
// change alters what the app reports about a month that is over, which is a
// different act from correcting the one you are living in. The summary has to
// name the month and say the change is retroactive.
func TestReconcileOnAClosedPeriodSaysSo(t *testing.T) {
	funding := budgeting.FundingRead{Expected: 1000000, Received: 600000, Assigned: 1000000}

	live := render.New(t)
	live.Render(ui.CreateElement(budgetFundedCallout, budgetFundedProps{
		Funding: funding, Base: "USD", ScalableCount: 12, PeriodLabel: "Jul 2026",
	}))
	liveText := byTestID(live, "span", "budgets-funded-reconcile-summary").Text()
	live.Cleanup()

	past := render.New(t)
	past.Render(ui.CreateElement(budgetFundedCallout, budgetFundedProps{
		Funding: funding, Base: "USD", ScalableCount: 12, PeriodLabel: "Jul 2026", Historical: true,
	}))
	pastNode := byTestID(past, "span", "budgets-funded-reconcile-summary")
	if pastNode == nil || pastNode.Text() == "" {
		t.Fatal("a closed period states nothing about the reconcile action's reach")
	}
	pastText := pastNode.Text()

	if pastText == liveText {
		t.Fatalf("a closed period reads exactly like a live one: %q", pastText)
	}
	if !strings.Contains(pastText, "Jul 2026") {
		t.Errorf("the closed-period summary does not name the month it would change; got %q", pastText)
	}
	if !strings.Contains(strings.ToLower(pastText), "already ended") {
		t.Errorf("the closed-period summary does not say the period is over; got %q", pastText)
	}
	// And the disclosure must reach the CONTROL, not only the caption under it —
	// a reader who acts on the prominent thing and skips the grey line is exactly
	// the reader this ticket is about.
	pastBtn := byTestID(past, "button", "budgets-funded-reconcile")
	if pastBtn == nil {
		t.Fatal("the reconcile action did not render on a closed period")
	}
	if !strings.Contains(pastBtn.Name(), "Jul 2026") {
		t.Errorf("the button does not name the month it would change; got %q", pastBtn.Name())
	}
	if !strings.Contains(strings.ToLower(pastBtn.Name()), "has ended") {
		t.Errorf("the button does not say the period is closed; got %q", pastBtn.Name())
	}
	if strings.Contains(pastBtn.Name(), "%!") {
		t.Errorf("the button label mis-formats its argument: %q", pastBtn.Name())
	}
	// Both still state the size, which is the disclosure the ticket asked for.
	for _, text := range []string{liveText, pastText} {
		if !strings.Contains(text, "12") {
			t.Errorf("summary omits the affected budget count; got %q", text)
		}
	}
}

// TestAdjustIntroFollowsTheScope: the form's opening sentence used to promise a
// change to "every budget's limit" whichever scope was selected — false of a
// this-period change, which never touches a limit.
func TestAdjustIntroFollowsTheScope(t *testing.T) {
	permKey := adjustIntroKey(budgeting.AdjustEveryPeriod)
	thisKey := adjustIntroKey(budgeting.AdjustThisPeriod)
	if permKey == thisKey {
		t.Fatalf("both scopes share the intro key %q", permKey)
	}
	perm, thisPeriod := uistate.T(permKey), uistate.T(thisKey)
	if perm == permKey || thisPeriod == thisKey {
		t.Fatalf("missing catalog entry for %q / %q", permKey, thisKey)
	}
	if !strings.Contains(strings.ToLower(perm), "limit") {
		t.Errorf("the permanent intro should still say it changes limits; got %q", perm)
	}
	if !strings.Contains(strings.ToLower(thisPeriod), "left as it is") {
		t.Errorf("the this-period intro must say the limits are left alone; got %q", thisPeriod)
	}
}

// TestEffectiveCapsAreTheCardsCaps pins the six lines the whole C671 preview fix
// rests on: the caps a this-period adjustment scales must be the caps the CARDS
// score against — base plus rollover carry-in plus any one-off change already
// recorded — not the stored base limit.
func TestEffectiveCapsAreTheCardsCaps(t *testing.T) {
	app := guardTestApp(t)
	now := time.Now()
	pr := uistate.LoadPrefs()
	periodStart, _ := budgeting.PeriodRange(domain.PeriodMonthly, now, pr.WeekStartWeekday())

	// A budget already lowered by $300 for this period only: base $1,300, cap $1,000.
	b := quickFillFixtureBudget()
	if err := app.PutBudget(b.WithPeriodBoost(periodStart, -30000)); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	// computeBudgetView memoizes on the data revision, which the app bumps after
	// every real mutation — a test writing straight to the store has to say so, or
	// it reads a sibling test's cached view.
	uistate.BumpDataRevision()

	// No fixture needed: computeBudgetView is a plain function, and the prefs and
	// window it wants can be built without hooks.
	vw := period.NewWindow(period.Month, now, pr.WeekStartWeekday())
	caps := budgetEffectiveCaps(computeBudgetView(app, "", vw, pr, false))

	got, ok := caps[b.ID]
	if !ok {
		t.Fatalf("the budget is missing from the caps map: %v", caps)
	}
	if got == b.Limit.Amount {
		t.Fatalf("cap = %d, which is the STORED base limit — a this-period adjustment scaled against this would promise a figure the write cannot deliver", got)
	}
	if got != 100000 {
		t.Errorf("cap = %d, want 100000 (base 130000 with the −30000 recorded for this period)", got)
	}
}

// TestAdjustPeriodDisclosureFollowsTheWindow is the round-3 guard for the second
// door: the toolbar's own "Adjust all" is available at any time and carries none
// of the funding callout's disclosures, so the form itself has to say WHICH
// period a this-period change lands on — and say it louder when that period has
// already closed.
func TestAdjustPeriodDisclosureFollowsTheWindow(t *testing.T) {
	liveKey, pastKey := adjustPeriodKey(false), adjustPeriodKey(true)
	if liveKey == pastKey {
		t.Fatalf("both window states share the copy key %q", liveKey)
	}
	live, past := uistate.T(liveKey, "Jul 2026"), uistate.T(pastKey, "Jul 2026")
	for _, text := range []string{live, past} {
		if !strings.Contains(text, "Jul 2026") {
			t.Errorf("the period line does not name the period; got %q", text)
		}
		if strings.Contains(text, "%!") {
			t.Errorf("the period line mis-formats its argument: %q", text)
		}
	}
	if !strings.Contains(strings.ToLower(past), "already ended") {
		t.Errorf("the closed-window line does not say the period is over; got %q", past)
	}
	if !strings.Contains(strings.ToLower(past), "reports about") {
		t.Errorf("the closed-window line does not say what applying there does; got %q", past)
	}
	// It must not assert that the viewed window IS every budget's period — a
	// quarterly budget viewed in a closed month is still in a live quarter.
	if strings.HasPrefix(past, "This period is") {
		t.Errorf("the line claims the viewed window is each budget's period, which is false for a coarser cadence; got %q", past)
	}
	// Tone follows the same fact, so the warning reads as one.
	if adjustPeriodClass(true) == adjustPeriodClass(false) {
		t.Error("a closed period is toned the same as a live one")
	}
}

// The skip explanation must not prescribe a direction: the inversion guard fires
// on raises too (a small raise cannot clear a deep negative overlay), where
// "choose a smaller one" would be backwards.
func TestAdjustSkipCopyIsDirectionNeutral(t *testing.T) {
	keys := map[budgeting.SkipReason]string{
		budgeting.SkipWouldInvert:    adjustSkipKey(budgeting.SkipWouldInvert),
		budgeting.SkipNothingToScale: adjustSkipKey(budgeting.SkipNothingToScale),
		budgeting.SkipUnknownOverlay: adjustSkipKey(budgeting.SkipUnknownOverlay),
	}
	seen := map[string]bool{}
	for reason, key := range keys {
		if seen[key] {
			t.Errorf("%q reuses another reason's copy key %q", reason, key)
		}
		seen[key] = true
		if text := uistate.T(key, "2 budgets", "Groceries, Fuel"); text == key {
			t.Errorf("missing catalog entry for %q", key)
		}
	}
	invert := strings.ToLower(uistate.T(keys[budgeting.SkipWouldInvert], "2 budgets", "Groceries, Fuel"))
	if strings.Contains(invert, "smaller") {
		t.Errorf("the inversion message prescribes a direction, but the guard fires on raises too; got %q", invert)
	}
	if !strings.Contains(invert, "at or below zero") {
		t.Errorf("the inversion message does not say what it is protecting; got %q", invert)
	}
}

// TestOverlaysSurviveAnUnevaluableBudget is the round-4 regression guard, for the
// INVERSE of the inversion bug.
//
// Requiring cap information under permanent scope closed an unsafe write, but a
// cap is only knowable where a budget's whole status could be evaluated — and one
// transaction in a currency with no exchange rate is enough to fail that. Coupling
// a permanent base-limit rewrite to spend and FX data would drop a budget carrying
// nothing at all out of the bulk tool for a reason that has nothing to do with it.
// An overlay of zero is knowable without any of that, and must be supplied.
func TestOverlaysSurviveAnUnevaluableBudget(t *testing.T) {
	plain := domain.Budget{
		ID: "b-plain", Name: "Travel", CategoryID: "c-transport",
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
		Period: domain.PeriodMonthly, Limit: money.New(100000, "USD"),
	}
	carrying := plain
	carrying.ID, carrying.Name, carrying.Rollover = "b-carrying", "Groceries", true

	// An empty view stands in for "no budget could be evaluated" — the shape an FX
	// gap produces, without needing to break the FX table to get there.
	overlays := budgetAdjustOverlays(budgetView{}, []domain.Budget{plain, carrying},
		func(domain.Budget) time.Time { return time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC) })

	if _, ok := overlays[plain.ID]; !ok {
		t.Error("a budget carrying nothing was dropped because an unrelated evaluation failed — a permanent rewrite does not depend on spend data")
	}
	if got := overlays[plain.ID]; got != 0 {
		t.Errorf("overlay for a budget with no rollover and no boost = %d, want 0", got)
	}
	if _, ok := overlays[carrying.ID]; ok {
		t.Error("a rollover budget's carry-in cannot be known without the evaluation that failed — it must be left out, not guessed")
	}

	// And the preview follows: the plain budget is adjustable, the rollover one is
	// reported rather than silently missing.
	p := budgeting.AdjustAllPreviewFor([]domain.Budget{plain, carrying}, -10, budgeting.AdjustEveryPeriod, overlays)
	if p.Count() != 1 || p.Lines[0].Budget.ID != plain.ID {
		t.Fatalf("permanent preview covered %d budgets (%+v), want just the plain one", p.Count(), p.Lines)
	}
	if got := p.SkippedFor(budgeting.SkipUnknownOverlay); len(got) != 1 || got[0].Budget.ID != carrying.ID {
		t.Errorf("the unknowable budget was not reported: %+v", p.Skipped)
	}

	// A boost the budget carries itself is still counted, evaluation or no.
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	boosted := plain.WithPeriodBoost(start, -30000)
	got := budgetAdjustOverlays(budgetView{}, []domain.Budget{boosted}, func(domain.Budget) time.Time { return start })
	if got[boosted.ID] != -30000 {
		t.Errorf("overlay for a boosted budget = %d, want -30000", got[boosted.ID])
	}
}

// The skip explanation must not claim a scope. SkipUnknownOverlay fires under
// both, so wording it as a this-period limitation misdescribes the permanent case.
func TestUnknownOverlayCopyIsScopeNeutral(t *testing.T) {
	text := uistate.T(adjustSkipKey(budgeting.SkipUnknownOverlay), "2 budgets", "Travel, Groceries")
	if text == adjustSkipKey(budgeting.SkipUnknownOverlay) {
		t.Fatal("missing catalog entry for the unknown-overlay skip")
	}
	if strings.Contains(strings.ToLower(text), "this-period change") {
		t.Errorf("the message claims a scope it does not have; got %q", text)
	}
}

// The permanent acknowledgement must not draw a boundary the data model lacks: a
// budget's limit has no history, so "this period and every period after it" is
// not where the change stops.
func TestPermanentAckDoesNotClaimAFutureOnlyBoundary(t *testing.T) {
	for _, key := range []string{"budgets.adjustAllAckFutureLower", "budgets.adjustAllAckFutureRaise"} {
		text := uistate.T(key, "12 budgets", "40.7")
		if text == key {
			t.Fatalf("missing catalog entry for %q", key)
		}
		if strings.Contains(strings.ToLower(text), "every period after it") {
			t.Errorf("%s claims the change starts at this period; a limit has no history, so past periods report against it too: %q", key, text)
		}
		if !strings.Contains(strings.ToLower(text), "every period") {
			t.Errorf("%s no longer says the change is unbounded: %q", key, text)
		}
	}
}

// TestReconcileCountMatchesTheFormsOwnCount is the round-5 guard for the
// invariant the callout claims: the budget count promised on the button is the
// count the form will show.
//
// It broke once already, silently — the preview's fourth argument changed meaning
// from effective CAPS to OVERLAYS and one of the two call sites was not migrated,
// so the button counted budgets against base+cap while the form counted them
// against base+overlay. Both are map[string]int64, so nothing complained. The
// types are now distinct (compile error), and this pins the behaviour as well.
func TestReconcileCountMatchesTheFormsOwnCount(t *testing.T) {
	app := guardTestApp(t)
	pr := uistate.LoadPrefs()
	now := time.Now()
	periodStart, _ := budgeting.PeriodRange(domain.PeriodMonthly, now, pr.WeekStartWeekday())

	// One ordinary budget, and one already pulled below zero for this period —
	// the case where counting against the wrong basis diverges.
	plain := quickFillFixtureBudget()
	drained := plain
	drained.ID, drained.Name = "b-drained", "Drained"
	if err := app.PutBudget(plain); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	if err := app.PutBudget(drained.WithPeriodBoost(periodStart, -drained.Limit.Amount-100)); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	uistate.BumpDataRevision()

	vw := period.NewWindow(period.Month, now, pr.WeekStartWeekday())
	v := computeBudgetView(app, "", vw, pr, false)

	// What the button promises.
	promised := budgetReconcileCount(app, "", v, vw, pr, -40)

	// What the form itself will compute, by the form's own route.
	scoped := adjustAllScope(app, "")
	overlays := budgetAdjustOverlays(v, scoped, budgetPeriodStarts(vw, pr))
	shown := budgeting.AdjustAllPreviewFor(scoped, -40, budgeting.AdjustThisPeriod, overlays).Count()

	if promised != shown {
		t.Errorf("the button promises %d budgets and the form shows %d", promised, shown)
	}
	// The drained budget has nothing left to scale, so neither should count it.
	if promised != 1 {
		t.Errorf("count = %d, want 1 — the budget already pulled below zero has no amount to take a percentage of", promised)
	}
}
