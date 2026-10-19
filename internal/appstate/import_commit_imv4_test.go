package appstate

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func imv4Batch(t *testing.T, a *App) StagedImport {
	t.Helper()
	seedAgreementAccount(t, a)
	batch, err := a.Stage(domain.DocCSV, []extract.Row{
		{ID: "one", Date: "2026-06-01", Description: "Coffee", Amount: "-3.50"},
		{ID: "two", Date: "2026-06-02", Description: "Coffee", Amount: "-5.00"},
	}, "chk")
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func TestIMV4BatchMatchesOrderedLocalWorkflows(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	seedAgreementAccount(t, a)
	if err := a.PutRule(rules.Rule{ID: "coffee", Match: "coffee", SetCategoryID: "food"}); err != nil {
		t.Fatal(err)
	}
	for _, w := range []workflow.Workflow{
		{ID: "flag", Name: "Flag", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "txn_abs > 4", Actions: []workflow.Action{{Kind: workflow.ActionFlagReview}}},
		{ID: "category", Name: "Category", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Actions: []workflow.Action{{Kind: workflow.ActionSetCategory, CategoryID: "other"}, {Kind: workflow.ActionCreateTask, Title: "Review imported rows"}, {Kind: workflow.ActionNotify, Message: "Imported {{txn_abs}}"}}},
	} {
		if err := a.PutWorkflow(w); err != nil {
			t.Fatal(err)
		}
	}
	goal := domain.Goal{ID: "fund", Name: "Food fund", IsSinkingFund: true, CategoryID: "food", CurrentAmount: money.New(1000, "USD"), TargetAmount: money.New(1000, "USD")}
	if err := a.Store().PutGoal(goal); err != nil {
		t.Fatal(err)
	}
	batch := imv4Batch(t, a)
	batch.ruleHits = map[string]int{"coffee": 2}
	_ = a.Transactions()
	beforeRev := a.Rev()
	notifications, notices := 0, 0
	a.OnTxnMutated(func() { notifications++ })
	a.Notifier = func(string) { notices++ }
	result, err := a.CommitImportBatch(batch, "history", "checkpoint")
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 || notifications != 1 || notices != 1 || a.Rev() != beforeRev+1 {
		t.Fatalf("result=%+v notifications=%d notices=%d revision=%d/%d", result, notifications, notices, a.Rev(), beforeRev)
	}
	rows := a.Transactions()
	if rows[0].CategoryID != "other" || rows[1].CategoryID != "other" {
		t.Fatal("workflow category was not applied")
	}
	for _, row := range rows {
		if row.Amount.Amount == -500 && !reflect.DeepEqual(row.Tags, []string{workflow.ReviewTag}) {
			t.Fatalf("flags=%v", row.Tags)
		}
	}
	if funds := a.Goals(); funds[0].CurrentAmount.Amount != 150 {
		t.Fatalf("drawdown used workflow category or repeated the batch: %+v", funds)
	}
	if len(a.Tasks()) != 1 || len(a.WorkflowRuns()) != 3 {
		t.Fatalf("tasks=%d workflow runs=%d", len(a.Tasks()), len(a.WorkflowRuns()))
	}
	if hits := a.Rules()[0].HitCount; hits != 2 {
		t.Fatalf("rule hits=%d", hits)
	}
	if doc := a.Documents()[0]; doc.CheckpointID != "checkpoint" || doc.RowCount != 2 {
		t.Fatalf("history=%+v", doc)
	}
}

func TestIMV4BatchDurabilityAndFailure(t *testing.T) {
	for _, mode := range []string{"unavailable", "pending", "failure", "success", "short revision"} {
		t.Run(mode, func(t *testing.T) {
			a := newApp(t, false)
			defer a.Store().Close()
			if err := a.PutWorkflow(workflow.Workflow{ID: "notice", Name: "Notice", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "Import complete"}}}); err != nil {
				t.Fatal(err)
			}
			notices := 0
			a.Notifier = func(string) { notices++ }
			batch := imv4Batch(t, a)
			writer := make(chan func(DurableResult), 1)
			if mode != "unavailable" {
				a.SetDurableWriter(func(_ uint64, done func(DurableResult)) { writer <- done })
			}
			finished := make(chan error, 1)
			progress := make(chan string, 10)
			a.CommitImportBatchDurable(batch, "history", "", func(p ImportCommitProgress) { progress <- p.Phase }, func(_ CSVImportResult, err error) { finished <- err })
			if mode == "unavailable" {
				if err := <-finished; !errors.Is(err, ErrDurabilityUnavailable) {
					t.Fatal(err)
				}
				if len(a.Transactions()) != 0 {
					t.Fatal("wrote without a durable writer")
				}
				return
			}
			var done func(DurableResult)
			select {
			case done = <-writer:
			case <-time.After(10 * time.Second):
				t.Fatal("writer not called")
			}
			select {
			case err := <-finished:
				t.Fatalf("acknowledged before durability: %v", err)
			default:
			}
			if notices != 0 {
				t.Fatal("workflow notice acknowledged the import before durability")
			}
			result := DurableResult{Revision: a.Rev()}
			if mode == "failure" {
				result.Err = errors.New("strict transaction aborted")
			}
			if mode == "short revision" {
				result.Revision = 0
			}
			done(result)
			err := <-finished
			if (err != nil) != (mode == "failure" || mode == "short revision") {
				t.Fatalf("completion error=%v", err)
			}
			if err != nil && notices != 0 || err == nil && notices != 1 {
				t.Fatalf("durable workflow notices=%d error=%v", notices, err)
			}
			if len(a.Transactions()) != 2 {
				t.Fatal("pending/failed durable save lost retryable rows")
			}
			if phase := <-progress; phase != "preparing" {
				t.Fatalf("initial phase=%s", phase)
			}
			done(result)
			select {
			case <-finished:
				t.Fatal("completed twice")
			default:
			}
		})
	}
}

func TestIMV4BatchRejectsEditDuringYield(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	batch := imv4Batch(t, a)
	if err := a.PutAccount(domain.Account{ID: "other", Name: "Other", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CommitImportBatch(batch, "history", ""); !errors.Is(err, ErrStaleImport) {
		t.Fatalf("error=%v", err)
	}
	if len(a.Transactions()) != 0 || len(a.Documents()) != 0 {
		t.Fatal("stale import wrote rows")
	}
}

func TestIMV4BulkLedgerSnapshotOwnsNestedValues(t *testing.T) {
	a := newApp(t, false)
	defer a.Store().Close()
	batch := imv4Batch(t, a)
	// Seal a draft that contains a nested custom value (as a trusted stage does).
	batch.Rows[0].Transaction.Custom = map[string]any{"nested": map[string]any{"value": "original"}}
	batch.seal, _ = importSeal(batch)
	before := a.Transactions()
	if _, err := a.CommitImportBatch(batch, "", ""); err != nil {
		t.Fatal(err)
	}
	batch.Rows[0].Transaction.Custom["nested"].(map[string]any)["value"] = "changed"
	row, _ := a.TransactionByID(batch.Rows[0].Transaction.ID)
	if row.Custom["nested"].(map[string]any)["value"] != "original" || len(before) != 0 {
		t.Fatal("imported snapshot shares mutable draft state")
	}
}

func TestIMV4LocalWorkflowProof(t *testing.T) {
	for _, test := range []struct {
		name, condition string
		action          workflow.Action
		local           bool
	}{
		{"row amount", "txn_abs > 2", workflow.Action{Kind: workflow.ActionFlagReview}, true},
		{"quoted aggregate name", `contains(txn_desc, "safe_to_spend")`, workflow.Action{Kind: workflow.ActionAddTag, Tag: "review"}, true},
		{"custom row field", "cf_txn_tip > 2", workflow.Action{Kind: workflow.ActionFlagReview}, true},
		{"aggregate condition", "safe_to_spend > 2", workflow.Action{Kind: workflow.ActionFlagReview}, false},
		{"aggregate template", "txn_abs > 2", workflow.Action{Kind: workflow.ActionNotify, Message: "Remaining {{safe_to_spend}}"}, false},
		{"merchant history", "txn_vs_typical > 2", workflow.Action{Kind: workflow.ActionFlagReview}, false},
		{"dataset action", "txn_abs > 2", workflow.Action{Kind: workflow.ActionApplyRules}, false},
		{"new resolving task", "", workflow.Action{Kind: workflow.ActionCreateTask, Title: "Check", ResolveCondition: "txn_abs > 2"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := localImportWorkflow(workflow.Workflow{Condition: test.condition, Actions: []workflow.Action{test.action}}); got != test.local {
				t.Fatalf("local=%t, want %t", got, test.local)
			}
		})
	}
}
