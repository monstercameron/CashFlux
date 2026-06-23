package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func openTasksWithTitle(a *App, title string) int {
	n := 0
	for _, tk := range a.Tasks() {
		if tk.Status == domain.StatusOpen && tk.Title == title {
			n++
		}
	}
	return n
}

// M1: the txn-added trigger fires from PutTransaction on a NEW transaction, and
// NOT on an edit of an existing one.
func TestTriggerFiresOnNewNotEdit(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	_ = a.PutWorkflow(workflow.Workflow{
		ID: "wf", Name: "Tag", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, // no condition → always
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "New txn"}},
	})

	tx := domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Desc: "Buy", Amount: money.New(-100, "USD")}
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("add: %v", err)
	}
	if got := len(a.WorkflowRuns()); got != 1 {
		t.Fatalf("new add should fire once: runs=%d", got)
	}

	// Edit the same transaction (same ID) — must NOT fire again.
	tx.Desc = "Buy (edited)"
	if err := a.PutTransaction(tx); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got := len(a.WorkflowRuns()); got != 1 {
		t.Errorf("edit should not fire the trigger: runs=%d, want 1", got)
	}
}

// M2 + bulk: importing many rows fires the trigger ONCE (not per row), and a
// createTask action doesn't pile up duplicate open tasks.
func TestBulkImportFiresOnceNoDuplicateTasks(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	_ = a.PutWorkflow(workflow.Workflow{
		ID: "wf", Name: "Note", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Imported batch"}},
	})
	csv := "date,account_id,desc,amount\n2026-06-10,Checking,Row one,-10\n2026-06-11,Checking,Row two,-20\n2026-06-12,Checking,Row three,-30\n"
	n, _, err := a.ImportTransactionsCSV([]byte(csv), "")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if n != 3 {
		t.Fatalf("imported %d rows, want 3", n)
	}
	if runs := len(a.WorkflowRuns()); runs != 1 {
		t.Errorf("bulk import should fire once, got %d runs", runs)
	}
	if got := openTasksWithTitle(a, "Imported batch"); got != 1 {
		t.Errorf("createTask should be idempotent, got %d tasks", got)
	}
}

// M2: a createTask workflow run twice produces only one open task.
func TestCreateTaskDedup(t *testing.T) {
	a := newApp(t, false)
	wf := workflow.Workflow{
		ID: "wf", Name: "Dup", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerManual},
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Only once"}},
	}
	_ = a.PutWorkflow(wf)
	_, _ = a.RunWorkflow(wf, false)
	_, _ = a.RunWorkflow(wf, false)
	if got := openTasksWithTitle(a, "Only once"); got != 1 {
		t.Errorf("expected 1 task after two runs, got %d", got)
	}
}

// A disabled workflow does not fire on its trigger.
func TestDisabledWorkflowNotTriggered(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	_ = a.PutWorkflow(workflow.Workflow{
		ID: "wf", Name: "Off", Enabled: false,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded},
		Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Should not appear"}},
	})
	_ = a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Desc: "Buy", Amount: money.New(-100, "USD")})
	if openTasksWithTitle(a, "Should not appear") != 0 {
		t.Error("disabled workflow created a task")
	}
	if len(a.WorkflowRuns()) != 0 {
		t.Error("disabled workflow recorded a run")
	}
}

// A multi-action run applies every action (create task, apply rules, notify) and
// the recorded Run carries each effect, including the notify summary.
func TestMultiActionApplyAndNotify(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	_ = a.PutCategory(domain.Category{ID: "c1", Name: "Coffee", Kind: domain.KindExpense})
	_ = a.PutRule(rules.Rule{ID: "r1", Match: "starbucks", SetCategoryID: "c1"})
	_ = a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Payee: "Starbucks", Desc: "coffee", Amount: money.New(-500, "USD")})

	wf := workflow.Workflow{
		ID: "wf", Name: "Everything", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerManual},
		Actions: []workflow.Action{
			{Kind: workflow.ActionCreateTask, Title: "Did it"},
			{Kind: workflow.ActionApplyRules},
			{Kind: workflow.ActionNotify, Message: "all done"},
		},
	}
	_ = a.PutWorkflow(wf)
	run, err := a.RunWorkflow(wf, false)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(run.Effects) != 3 {
		t.Fatalf("want 3 effects, got %d", len(run.Effects))
	}
	if run.Effects[2].Kind != workflow.ActionNotify || run.Effects[2].Summary != "Notify: all done" {
		t.Errorf("notify effect wrong: %+v", run.Effects[2])
	}
	if openTasksWithTitle(a, "Did it") != 1 {
		t.Error("createTask action did not create the task")
	}
	if tx, _, _ := a.store.GetTransaction("t1"); tx.CategoryID != "c1" {
		t.Errorf("applyRules action did not categorize: %q", tx.CategoryID)
	}
}

// The keystone value: a txn-added workflow can see the triggering transaction and
// act on it — categorize by payee, flag by amount.
func TestTransactionRouting(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	_ = a.PutCategory(domain.Category{ID: "dining", Name: "Dining", Kind: domain.KindExpense})
	// Route: anything from a bistro → Dining category.
	_ = a.PutWorkflow(workflow.Workflow{
		ID: "route", Name: "Dining router", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: `contains(txn_payee, "bistro")`,
		Actions: []workflow.Action{{Kind: workflow.ActionSetCategory, CategoryID: "dining"}},
	})
	// Flag big spends for review.
	_ = a.PutWorkflow(workflow.Workflow{
		ID: "flag", Name: "Big spend flag", Enabled: true,
		Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "txn_abs > 200",
		Actions: []workflow.Action{{Kind: workflow.ActionFlagReview}},
	})

	// A small bistro charge: categorized, not flagged.
	_ = a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "acc1", Date: thisMonth(), Payee: "Bistro Roma", Desc: "dinner", Amount: money.New(-3000, "USD")})
	t1, _, _ := a.store.GetTransaction("t1")
	if t1.CategoryID != "dining" {
		t.Errorf("bistro txn not routed to Dining: %q", t1.CategoryID)
	}
	if hasTag(t1.Tags, workflow.ReviewTag) {
		t.Error("small txn should not be flagged for review")
	}

	// A large non-bistro charge: flagged, not categorized by the router.
	_ = a.PutTransaction(domain.Transaction{ID: "t2", AccountID: "acc1", Date: thisMonth(), Payee: "Electronics Store", Desc: "laptop", Amount: money.New(-150000, "USD")})
	t2, _, _ := a.store.GetTransaction("t2")
	if !hasTag(t2.Tags, workflow.ReviewTag) {
		t.Errorf("big txn not flagged: tags=%v", t2.Tags)
	}
	if t2.CategoryID == "dining" {
		t.Error("non-bistro txn wrongly routed to Dining")
	}
}

func hasTag(tags []string, tag string) bool {
	for _, x := range tags {
		if x == tag {
			return true
		}
	}
	return false
}

// notify fires the Notifier hook so the message reaches the user.
func TestNotifyHook(t *testing.T) {
	a := newApp(t, false)
	var got string
	a.Notifier = func(m string) { got = m }
	wf := workflow.Workflow{ID: "n", Name: "Ping", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerManual},
		Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "hello"}}}
	_, _ = a.RunWorkflow(wf, false)
	if got != "hello" {
		t.Errorf("notifier got %q, want hello", got)
	}
}

// m1: the clock seam makes month-scoped figures deterministic.
func TestClockSeamDrivesMonthScope(t *testing.T) {
	a := newApp(t, false)
	seedAccount(t, a, 0)
	// Pin "now" to January 2020.
	a.now = func() time.Time { return time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC) }
	// Income inside the pinned month counts; income outside it does not.
	_ = a.PutTransaction(domain.Transaction{ID: "in", AccountID: "acc1", Date: time.Date(2020, 1, 10, 0, 0, 0, 0, time.UTC), Desc: "Jan pay", Amount: money.New(300000, "USD")})
	_ = a.PutTransaction(domain.Transaction{ID: "out", AccountID: "acc1", Date: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), Desc: "Other", Amount: money.New(999999, "USD")})
	vars := a.engineVars()
	if vars["income"] != 3000 {
		t.Errorf("income = %v, want 3000 (only the pinned-month txn)", vars["income"])
	}
}
