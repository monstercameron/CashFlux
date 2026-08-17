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
