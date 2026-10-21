// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestINT3NonRolloverTargetUsesViewedStart(t *testing.T) {
	app := br2sugTestApp(t)
	old := appstate.Default
	appstate.Default = app
	defer func() { appstate.Default = old }()
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "int3-target", Name: "Target", CategoryID: "transport-cat", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(10000, "USD"), TargetKind: domain.TargetByDate, TargetAmount: money.New(130000, "USD"), TargetDate: start.AddDate(0, 12, 0), TargetSnoozedUntil: start.AddDate(0, -1, 0)}
	input := budgeting.ViewInput{Period: budgeting.ViewPeriod{Anchor: start, Now: start}, Base: "USD", Rates: currency.Rates{Base: "USD"}}
	v := budgeting.EvaluateViewBudget(b, input)
	need, err := budgetTargetNeed(app, b, v.Status, v.Status.Start)
	if err != nil || need.Snoozed || need.Needed.Amount != 10000 {
		t.Fatalf("need=%+v err=%v", need, err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node { return budgetTargetLine(v.Status) }))
	if byTestID(f, "span", "budget-target-snoozed-"+b.ID) != nil {
		t.Fatal("expired snooze still shown")
	}
	node := byTestID(f, "span", "budget-target-"+b.ID)
	if node == nil || !strings.Contains(node.Text(), "100.00") {
		t.Fatalf("target line omits viewed-period pace: %v", node)
	}
}

func TestINT3HistoricalPlanAndEditorUsePeriodBase(t *testing.T) {
	app := br2sugTestApp(t)
	old := appstate.Default
	appstate.Default = app
	defer func() { appstate.Default = old }()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "int3-history", Name: "History", CategoryID: "transport-cat", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(60000, "USD"), LimitHistory: map[string]int64{"2026-01-01": 50000, "2026-10-01": 60000}}
	if err := app.Store().PutBudget(b); err != nil {
		t.Fatal(err)
	}
	window := period.NewWindow(period.Month, start, time.Monday)
	v := computeBudgetView(app, "", window, prefs.Prefs{}, false)
	plan := budgetSummaryPlan(v, app, window)
	if plan.normalizedBudgetTotal != 50000 {
		t.Fatalf("January plan=%d", plan.normalizedBudgetTotal)
	}
	var draft budgetRowLimitDraftResult
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		draft = useBudgetRowLimitDraft(v.Statuses[0])
		return Button(Attr("id", "edit-int3"), OnClick(draft.startLimitEdit), "Edit")
	}))
	f.ByID("edit-int3").Click()
	if draft.limitDraft.Get() != "500.00" {
		t.Fatalf("historical draft=%q", draft.limitDraft.Get())
	}
}

func TestINT3InformationalIssueIsNotFailure(t *testing.T) {
	var log bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	defer slog.SetDefault(old)
	issue := budgeting.ViewIssue{BudgetID: "b", Operation: "rollover start", Kind: budgeting.ViewIssueRolloverStartAssumed, Informational: true}
	builder := newBudgetViewBuilder(budgeting.ViewInput{}, false)
	builder.addIssues([]budgeting.ViewIssue{issue})
	if strings.Contains(log.String(), "level=ERROR") {
		t.Fatalf("note logged as error: %s", log.String())
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node { return budgetSummaryDiagnosticLines([]budgeting.ViewIssue{issue}) }))
	if byTestID(f, "p", "budgets-status-issues") != nil {
		t.Fatal("information rendered in failure strip")
	}
	note := byTestID(f, "p", "budgets-status-notes")
	if note == nil || !strings.Contains(note.Text(), "estimated rollover start") {
		t.Fatal("missing rollover explanation")
	}
}

func TestINT3ReviewCompletionWaitsForDurability(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			app := newReviewApp(t)
			txn := putCharge(t, app, "int3-review", "Coffee", 5)
			txn.Tags = []string{reviewqueue.ReviewTag, "keep", reviewqueue.ReviewTag}
			if err := app.PutTransaction(txn); err != nil {
				t.Fatal(err)
			}
			var complete func(appstate.DurableResult)
			var rev uint64
			app.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { rev, complete = r, done })
			f := render.New(t)
			defer f.Cleanup()
			var notice state.Atom[uistate.Notice]
			f.Render(ui.CreateElement(func() ui.Node { notice = uistate.UseNotice(); return Div() }))
			notice.Set(uistate.Notice{})
			calls := 0
			success := false
			if !applyReviewChoice(app, []string{txn.ID}, "cat-food", func(ok bool) { calls++; success = ok }) {
				t.Fatal("review was not accepted")
			}
			if complete == nil || calls != 0 || notice.Get().Undoable {
				t.Fatal("review acknowledged before commit")
			}
			got, _ := app.TransactionByID(txn.ID)
			if !got.Reviewed || got.HasSplits() || len(got.Tags) != 1 || got.Tags[0] != "keep" {
				t.Fatalf("confirmation semantics: %+v", got)
			}
			result := appstate.DurableResult{Revision: rev}
			if fail {
				result.Err = errors.New("quota")
			}
			complete(result)
			if calls != 1 || success == fail || notice.Get().Undoable == fail {
				t.Fatalf("completion calls=%d success=%v notice=%+v", calls, success, notice.Get())
			}
		})
	}
}

func TestINT3ScopedLedgerResolvesRefundBeforeFiltering(t *testing.T) {
	app := br2sugTestApp(t)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	br2sugPutExpense(t, app, "purchase", start, -10000)
	br2sugPutExpense(t, app, "refund", start.AddDate(0, 1, 0), 6000)
	if err := app.PutTxnLink(domain.TxnLink{ID: "int3-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(6000, "USD")}); err != nil {
		t.Fatal(err)
	}
	purchase, _ := app.TransactionByID("purchase")
	scoped := budgetLedgerForTransactions(app, []domain.Transaction{purchase})
	if len(scoped) != 1 || scoped[0].Amount.Amount != -4000 {
		t.Fatalf("scoped net=%+v", scoped)
	}
}

func TestINT3EnvelopeCardUsesSameSmoothingInputs(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "int3-envelope", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(100000, "USD"), RolloverStart: start}
	ledger := budgeting.NewNettedLedger([]domain.Transaction{{ID: "annual-posted", CategoryID: "food", Amount: money.New(-60000, "USD"), Date: start.AddDate(0, 0, 9)}})
	input := budgeting.ViewInput{Ledger: &ledger, Period: budgeting.ViewPeriod{Anchor: start.AddDate(0, 0, 14), Now: start.AddDate(0, 0, 14)}, Base: "USD", Rates: currency.Rates{Base: "USD"}, Recurring: []domain.Recurring{{ID: "annual", Label: "Annual", CategoryID: "food", Amount: money.New(-60000, "USD"), Cadence: domain.CadenceYearly, NextDue: start.AddDate(0, 0, 14), SmoothIntoBudgets: true}}}
	view := budgeting.EvaluateViewBudget(b, input)
	builder := newBudgetViewBuilder(input, false)
	builder.addBudget(view)
	builder.addEnvelopes([]domain.Budget{b}, string(budgeting.MethodEnvelope))
	if !view.Valid || view.Status.Remaining.Amount != 100000 {
		t.Fatalf("card=%+v", view.Status)
	}
	if got, want := builder.view.EnvAvail[b.ID], fmtMoney(view.Status.Remaining); got != want {
		t.Fatalf("envelope=%s card=%s", got, want)
	}
}

func TestINT3KeyboardClearsAlreadyReviewedFlagsAfterCommit(t *testing.T) {
	app := newReviewApp(t)
	txn := putCharge(t, app, "int3-keyboard", "Coffee", 5)
	txn.Reviewed = true
	txn.CategoryID = "cat-food"
	txn.Tags = []string{reviewqueue.ReviewTag, "keep", reviewqueue.ReviewTag}
	txn.Splits = []domain.CategorySplit{{CategoryID: "cat-food", Amount: txn.Amount}}
	if err := app.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	var complete func(appstate.DurableResult)
	var rev uint64
	app.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { rev, complete = r, done })
	f := render.New(t)
	defer f.Cleanup()
	var notice state.Atom[uistate.Notice]
	f.Render(ui.CreateElement(func() ui.Node { notice = uistate.UseNotice(); return Div() }))
	notice.Set(uistate.Notice{})
	markTxnReviewedFromKeyboard(txn.ID)
	if complete == nil || notice.Get().Undoable {
		t.Fatal("keyboard review omitted durable boundary")
	}
	got, _ := app.TransactionByID(txn.ID)
	if !got.Reviewed || got.HasSplits() || len(got.Tags) != 1 || got.Tags[0] != "keep" {
		t.Fatalf("keyboard confirmation=%+v", got)
	}
	complete(appstate.DurableResult{Revision: rev})
	if !notice.Get().Undoable {
		t.Fatal("durable keyboard confirmation has no Undo")
	}
}

func TestINT3ExplicitReviewAppDoesNotPublishIntoActiveScope(t *testing.T) {
	app := newReviewApp(t)
	txn := putCharge(t, app, "int3-isolated", "Coffee", 5)
	// Keep another valid scope active; earlier fixture trees may still observe
	// shared atoms while this isolated App finishes.
	_ = newReviewApp(t)
	var complete func(appstate.DurableResult)
	var rev uint64
	app.SetDurableWriter(func(r uint64, done func(appstate.DurableResult)) { rev, complete = r, done })
	f := render.New(t)
	defer f.Cleanup()
	var notice state.Atom[uistate.Notice]
	f.Render(ui.CreateElement(func() ui.Node { notice = uistate.UseNotice(); return Div() }))
	notice.Set(uistate.Notice{})
	completed := false
	if !applyReviewChoice(app, []string{txn.ID}, "cat-food", func(ok bool) { completed = ok }) {
		t.Fatal("explicit review was not accepted")
	}
	complete(appstate.DurableResult{Revision: rev})
	if !completed || notice.Get().Undoable || notice.Get().Text != "" {
		t.Fatalf("isolated completion crossed scope: %+v", notice.Get())
	}
}

func TestINT3WeeklyPlanUsesViewedBaseAfterMidmonthChange(t *testing.T) {
	app := br2sugTestApp(t)
	old := appstate.Default
	appstate.Default = app
	defer func() { appstate.Default = old }()
	start := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	b := domain.Budget{ID: "int3-week-history", Name: "Weekly", CategoryID: "transport-cat", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodWeekly, Limit: money.New(60000, "USD"), LimitHistory: map[string]int64{"2026-01-01": 50000, "2026-10-19": 60000}}
	if err := app.Store().PutBudget(b); err != nil {
		t.Fatal(err)
	}
	window := period.NewWindow(period.Week, start, time.Monday)
	v := computeBudgetView(app, "", window, prefs.Prefs{WeekStart: prefs.WeekMonday}, false)
	got := budgetSummaryPlan(v, app, window)
	// $600/week is $2,600/month; seven of October's 31 days rounds to $587.10.
	if got.normalizedBudgetTotal != 58710 {
		t.Fatalf("viewed weekly plan=%d want 58710", got.normalizedBudgetTotal)
	}
}
