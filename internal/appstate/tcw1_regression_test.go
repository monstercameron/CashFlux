package appstate

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ruleapply"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func tcw1App(t *testing.T) *App {
	t.Helper()
	a := newApp(t, false)
	t.Cleanup(func() { _ = a.store.Close() })
	seedAgreementAccount(t, a)
	a.now = func() time.Time { return tcw1Date(t, "2026-10-19") }
	return a
}

func tcw1Date(t *testing.T, text string) time.Time {
	t.Helper()
	date, err := dateutil.ParseDate(text)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

func TestTCW1PostingCalendarAndPause(t *testing.T) {
	for _, tc := range []struct {
		name      string
		paused    bool
		due, asOf time.Time
		want      int
	}{
		{"paused", true, tcw1Date(t, "2026-10-19"), tcw1Date(t, "2026-10-19"), 0},
		{"future local calendar day", false, tcw1Date(t, "2026-10-20"), time.Date(2026, 10, 19, 23, 30, 0, 0, time.FixedZone("west", -4*3600)), 0},
		{"same day non-midnight", false, time.Date(2026, 10, 19, 18, 0, 0, 0, time.UTC), tcw1Date(t, "2026-10-19"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := tcw1App(t)
			r := domain.Recurring{ID: "rent", Label: "Rent", AccountID: "chk", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: tc.due, Autopost: true, Paused: tc.paused}
			if err := a.PutRecurring(r); err != nil {
				t.Fatal(err)
			}
			posted, err := a.PostDueRecurring(tc.asOf)
			if err != nil || posted != tc.want {
				t.Fatalf("posted=%d err=%v", posted, err)
			}
			if tc.want == 0 && !a.Recurring()[0].NextDue.Equal(tc.due) {
				t.Fatal("skipped occurrence advanced")
			}
		})
	}
}

func TestTCW1PostingFailureDoesNotStallOtherRecurring(t *testing.T) {
	a := tcw1App(t)
	card := domain.Account{ID: "card", Name: "Rewards Card", Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(-10000, "USD"), MinPayment: money.New(100, "USD")}
	source := domain.Account{ID: "source", Name: "Euro checking", Currency: "EUR", Class: domain.ClassAsset, Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(0, "EUR")}
	if err := a.PutAccount(source); err != nil {
		t.Fatal(err)
	}
	if err := a.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	due := tcw1Date(t, "2026-10-19")
	for _, r := range []domain.Recurring{
		{ID: "a-card", Label: "Rewards payment", AccountID: "source", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: due, Autopost: true},
		{ID: "z-rent", Label: "Rent", AccountID: "chk", Amount: money.New(-200, "USD"), Cadence: domain.CadenceMonthly, NextDue: due, Autopost: true},
	} {
		if err := a.PutRecurring(r); err != nil {
			t.Fatal(err)
		}
	}
	var notices []string
	a.Notifier = func(s string) { notices = append(notices, s) }
	posted, err := a.PostDueRecurring(due)
	if posted != 1 || err == nil || len(notices) != 1 {
		t.Fatalf("posted=%d err=%v notices=%v", posted, err, notices)
	}
	for _, r := range a.Recurring() {
		if r.ID == "a-card" && !r.NextDue.Equal(due) {
			t.Fatal("failed occurrence advanced")
		}
	}
	settings := a.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = map[string]float64{"EUR": 0.9}
	if err := a.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if posted, err = a.PostDueRecurring(due); err != nil || posted != 1 {
		t.Fatalf("retry posted=%d err=%v", posted, err)
	}
	if len(a.Transactions()) != 3 {
		t.Fatal("retry duplicated rent or omitted transfer leg")
	}
	if posted, err = a.PostDueRecurring(due); err != nil || posted != 0 {
		t.Fatalf("second retry posted=%d err=%v", posted, err)
	}
}

func TestTCW1OccurrenceReceiptSurvivesHydrationAndRowDeletion(t *testing.T) {
	for _, transfer := range []bool{false, true} {
		name := "expense"
		if transfer {
			name = "transfer"
		}
		t.Run(name, func(t *testing.T) {
			a := tcw1App(t)
			label := "Rent"
			if transfer {
				label = "Rewards payment"
				if err := a.PutAccount(domain.Account{ID: "card", Name: "Rewards Card", Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(-10000, "USD"), MinPayment: money.New(100, "USD")}); err != nil {
					t.Fatal(err)
				}
			}
			r := domain.Recurring{ID: "r", Label: label, AccountID: "chk", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: tcw1Date(t, "2026-10-19"), Autopost: true}
			if err := a.PutRecurring(r); err != nil {
				t.Fatal(err)
			}
			if n, err := a.PostDueRecurring(r.NextDue); err != nil || n != 1 {
				t.Fatalf("post=%d %v", n, err)
			}
			for _, txn := range a.Transactions() {
				if _, err := a.store.DeleteTransaction(txn.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.store.PutRecurring(r); err != nil {
				t.Fatal(err)
			} // stale schedule replay
			data, err := a.ExportJSON()
			if err != nil {
				t.Fatal(err)
			}
			b := tcw1App(t)
			if err := b.ImportJSON(data); err != nil {
				t.Fatal(err)
			}
			if n, err := b.PostDueRecurring(r.NextDue); err != nil || n != 0 || len(b.Transactions()) != 0 {
				t.Fatalf("receipt replay posted=%d err=%v rows=%d", n, err, len(b.Transactions()))
			}
			if !b.Recurring()[0].NextDue.After(r.NextDue) {
				t.Fatal("receipt did not recover the stale schedule")
			}
		})
	}
}

func TestTCW1RecurringOverflowReturnsError(t *testing.T) {
	a := tcw1App(t)
	if err := a.PutAccount(domain.Account{ID: "card", Name: "Rewards Card", Currency: "USD", Class: domain.ClassLiability, Type: domain.TypeCreditCard, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	r := domain.Recurring{ID: "r", Label: "Rewards payment", AccountID: "chk", Amount: money.New(math.MinInt64, "USD"), Cadence: domain.CadenceMonthly, NextDue: tcw1Date(t, "2026-10-19"), Autopost: true}
	if err := a.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	if n, err := a.PostDueRecurring(r.NextDue); n != 0 || err == nil {
		t.Fatalf("post=%d %v", n, err)
	}
}

func TestTCW1ImportIDCollisionIsIsolated(t *testing.T) {
	a := tcw1App(t)
	prior := domain.Transaction{ID: "collision", AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "Original", Amount: money.New(-100, "USD")}
	if err := a.store.PutTransaction(prior); err != nil {
		t.Fatal(err)
	}
	prior, _, _ = a.store.GetTransaction(prior.ID)
	result, err := a.ImportTransactionsCSV([]byte("id,date,desc,amount\ncollision,2026-10-02,Other,-2\ngood,2026-10-03,Good,-3\n"), "chk", "")
	if err != nil || result.Imported != 1 || len(result.Failed) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	current, _, _ := a.store.GetTransaction("collision")
	if !reflect.DeepEqual(prior, current) {
		t.Fatal("colliding row overwritten")
	}
}

func TestTCW1ImportAutomationSingleAtomicBatch(t *testing.T) {
	for _, entry := range []string{"Commit", "CommitImportBatch"} {
		t.Run(entry, func(t *testing.T) {
			a := tcw1App(t)
			if err := a.PutWorkflow(workflow.Workflow{ID: "tag", Name: "Tag", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "safe_to_spend < 999999999", Actions: []workflow.Action{{Kind: workflow.ActionAddTag, Tag: "batch"}, {Kind: workflow.ActionCreateTask, Title: "Check import"}}}); err != nil {
				t.Fatal(err)
			}
			if err := a.PutTask(domain.Task{ID: "resolve", Title: "Resolve once", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Resolve: &domain.TaskResolve{Condition: "txn_abs > 1"}}); err != nil {
				t.Fatal(err)
			}
			batch, err := a.StageCSV([]byte("date,desc,amount\n2026-10-01,One,-2\n2026-10-02,Two,-3\n"), "chk")
			if err != nil {
				t.Fatal(err)
			}
			rev := a.Rev()
			calls := 0
			a.OnTxnMutated(func() { calls++ })
			var result CSVImportResult
			if entry == "Commit" {
				result, err = a.Commit(batch, "history", "")
			} else {
				result, err = a.CommitImportBatch(batch, "history", "")
			}
			if err != nil || result.Imported != 2 || a.Rev() != rev+1 || calls != 1 {
				t.Fatalf("result=%+v err=%v revision delta=%d calls=%d", result, err, a.Rev()-rev, calls)
			}
			if len(a.WorkflowRuns()) != 2 || len(a.Tasks()) != 2 {
				t.Fatal("workflow/task effects missing or repeated")
			}
			for _, txn := range a.Transactions() {
				if !reflect.DeepEqual(txn.Tags, []string{"batch"}) {
					t.Fatalf("tags=%v", txn.Tags)
				}
			}
			for _, task := range a.Tasks() {
				if task.ID == "resolve" && task.Status != domain.StatusDone {
					t.Fatal("task did not resolve")
				}
			}
		})
	}
}

func TestTCW1MerchantHistoryIsCurrencyScopedAndExcludesCurrentRow(t *testing.T) {
	a := tcw1App(t)
	for i, amount := range []int64{-100, -100, -300} {
		if err := a.store.PutTransaction(domain.Transaction{ID: string(rune('a' + i)), AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "Coffee", Amount: money.New(amount, "USD")}); err != nil {
			t.Fatal(err)
		}
	}
	// Same merchant in another currency cannot set a USD baseline.
	for i := 0; i < 5; i++ {
		if err := a.store.PutTransaction(domain.Transaction{ID: string(rune('j' + i)), AccountID: "foreign", Date: tcw1Date(t, "2026-10-01"), Desc: "Coffee", Amount: money.New(-100000, "JPY")}); err != nil {
			t.Fatal(err)
		}
	}

	if err := a.PutWorkflow(workflow.Workflow{ID: "history", Name: "History", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "and(txn_payee_typical == 1, txn_vs_typical == 5)", Actions: []workflow.Action{{Kind: workflow.ActionAddTag, Tag: "price rise"}}}); err != nil {
		t.Fatal(err)
	}
	result, err := a.ImportTransactionsCSV([]byte("date,desc,amount\n2026-10-19,Coffee,-5\n"), "chk", "")
	if err != nil || result.Imported != 1 {
		t.Fatalf("%+v %v", result, err)
	}
	if len(a.WorkflowRuns()) != 1 {
		t.Fatal("merchant median included the target or was not available")
	}
}

func TestTCW1RulesEntryBackfillAndUndo(t *testing.T) {
	a := tcw1App(t)
	for _, r := range []rules.Rule{{ID: "bill", Order: 0, Match: "Coffee", SetBillAccountID: "bill-account"}, {ID: "tag", Order: 1, Match: "Coffee", SetTags: []string{"coffee"}}} {
		if err := a.PutRule(r); err != nil {
			t.Fatal(err)
		}
	}
	txn := domain.Transaction{ID: "t", AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "Coffee", Amount: money.New(-100, "USD")}
	entry := a.AutoCategorizeTransaction(txn)
	if err := a.store.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	if n, _ := a.PreviewApplyRules(); n != 1 {
		t.Fatalf("preview=%d", n)
	}
	if n, _, err := a.ApplyRulesWithCounts(); err != nil || n != 1 {
		t.Fatalf("apply=%d %v", n, err)
	}
	got, _, _ := a.store.GetTransaction(txn.ID)
	if !reflect.DeepEqual(entry.Tags, got.Tags) {
		t.Fatalf("entry=%v backfill=%v", entry.Tags, got.Tags)
	}
	got.CategoryID = "later manual edit"
	if err := a.store.PutTransaction(got); err != nil {
		t.Fatal(err)
	}
	if err := a.UndoRuleChanges([]ruleapply.Change{{TxnID: got.ID, FromCategoryID: "old", ToCategoryID: "rule"}}); err == nil {
		t.Fatal("undo overwrote intervening category edit")
	}
	if auditview.SessionActor() != "" {
		t.Fatal("rule actor leaked")
	}
}

func TestTCW1DocumentRepeatsRenameStatusAndRuleCredits(t *testing.T) {
	a := tcw1App(t)
	if err := a.PutRule(rules.Rule{ID: "member", Match: "RAW COFFEE", SetMemberID: "member", RenameDesc: "Coffee"}); err != nil {
		t.Fatal(err)
	}
	rows := []extract.Row{{Date: "2026-10-01", Description: "RAW COFFEE", Amount: "-1", Status: "posted"}, {Date: "2026-10-01", Description: "RAW COFFEE", Amount: "-1", Status: "posted"}}
	first, err := a.ImportReviewedDocumentRows(domain.DocImage, "chk", rows)
	if err != nil || first.Imported != 2 {
		t.Fatalf("first=%+v %v", first, err)
	}
	second, err := a.ImportReviewedDocumentRows(domain.DocImage, "chk", rows)
	if err != nil || second.Imported != 0 || second.Skipped != 2 {
		t.Fatalf("second=%+v %v", second, err)
	}
	if a.Rules()[0].HitCount != 2 {
		t.Fatalf("hits=%d", a.Rules()[0].HitCount)
	}
	for _, txn := range a.Transactions() {
		if txn.Desc != "Coffee" || !reflect.DeepEqual(txn.Owners(), []string{"member"}) {
			t.Fatalf("txn=%+v", txn)
		}
	}
}

func TestTCW1WriteSeamRejectsMissingTransferTarget(t *testing.T) {
	a := tcw1App(t)
	err := a.PutTransaction(domain.Transaction{ID: "t", AccountID: "chk", TransferAccountID: "missing", Date: tcw1Date(t, "2026-10-01"), Desc: "Transfer", Amount: money.New(-100, "USD")})
	if err == nil || !strings.Contains(err.Error(), "transferAccountId") {
		t.Fatalf("err=%v", err)
	}
}

func TestTCW1RuleUndoPreservesLaterEditsAtomicallyAndResetsActor(t *testing.T) {
	a := tcw1App(t)
	for _, row := range []domain.Transaction{
		{ID: "first", AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "One", CategoryID: "rule", Amount: money.New(-100, "USD")},
		{ID: "second", AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "Two", CategoryID: "manual", Amount: money.New(-100, "USD")},
	} {
		if err := a.store.PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	err := a.UndoRuleChanges([]ruleapply.Change{{TxnID: "first", FromCategoryID: "old", ToCategoryID: "rule"}, {TxnID: "second", FromCategoryID: "old", ToCategoryID: "rule"}})
	if err == nil {
		t.Fatal("undo accepted an intervening edit")
	}
	first, _, _ := a.store.GetTransaction("first")
	second, _, _ := a.store.GetTransaction("second")
	if first.CategoryID != "rule" || second.CategoryID != "manual" {
		t.Fatal("undo partially changed the ledger")
	}
	if auditview.SessionActor() != "" {
		t.Fatal("rule actor leaked after failed undo")
	}
	if err := a.UndoRuleChanges([]ruleapply.Change{{TxnID: "first", FromCategoryID: "old", ToCategoryID: "rule"}}); err != nil {
		t.Fatal(err)
	}
	if auditview.SessionActor() != "" {
		t.Fatal("rule actor leaked after successful undo")
	}
}

func TestTCW1ResolvingTaskUndoKeepsImportedAndLaterTransactions(t *testing.T) {
	a := tcw1App(t)
	if err := a.PutTask(domain.Task{ID: "resolve", Title: "Resolve", Status: domain.StatusOpen, Priority: domain.PriorityMedium, Resolve: &domain.TaskResolve{Condition: "txn_abs > 1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ImportTransactionsCSV([]byte("date,desc,amount\n2026-10-01,Coffee,-2\n"), "chk", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.store.PutTransaction(domain.Transaction{ID: "later", AccountID: "chk", Date: tcw1Date(t, "2026-10-02"), Desc: "Later", Amount: money.New(-100, "USD")}); err != nil {
		t.Fatal(err)
	}
	other := tcw1App(t)
	if _, ok := other.UndoLastAutoResolve(); ok {
		t.Fatal("a different scope consumed the undo")
	}
	if titles, ok := a.UndoLastAutoResolve(); !ok || !reflect.DeepEqual(titles, []string{"Resolve"}) {
		t.Fatalf("undo=%v %t", titles, ok)
	}
	if len(a.Transactions()) != 2 || a.Tasks()[0].Status != domain.StatusOpen {
		t.Fatal("undo lost ledger rows or failed to reopen task")
	}
}

func TestTCW1CommitDurableHoldsAutomationNotices(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			a := tcw1App(t)
			if err := a.PutWorkflow(workflow.Workflow{ID: "notice", Name: "Notice", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "safe_to_spend < 999999999", Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "Imported"}}}); err != nil {
				t.Fatal(err)
			}
			batch, err := a.StageCSV([]byte("date,desc,amount\n2026-10-01,Coffee,-2\n"), "chk")
			if err != nil {
				t.Fatal(err)
			}
			var save func(DurableResult)
			a.SetDurableWriter(func(_ uint64, done func(DurableResult)) { save = done })
			notices, completed := 0, 0
			a.Notifier = func(string) { notices++ }
			a.CommitDurable(batch, "history", "", func(_ CSVImportResult, err error) {
				completed++
				if (err != nil) != failed {
					t.Fatalf("err=%v", err)
				}
			})
			if save == nil || completed != 0 || notices != 0 {
				t.Fatal("automation acknowledged before durability")
			}
			durable := DurableResult{Revision: a.Rev()}
			if failed {
				durable.Err = errors.New("strict write failed")
			}
			save(durable)
			wantNotices := 1
			if failed {
				wantNotices = 0
			}
			if completed != 1 || notices != wantNotices {
				t.Fatalf("completed=%d notices=%d", completed, notices)
			}
		})
	}
}

func TestTCW1AmountRuleCurrencyAtEveryCaller(t *testing.T) {
	a := tcw1App(t)
	r := rules.Rule{ID: "usd", Conditions: []rules.RuleCondition{{Field: "amount", Op: ">=", Value: "100", Currency: "USD"}}, SetCategoryID: "usd-only"}
	if err := a.PutRule(r); err != nil {
		t.Fatal(err)
	}
	for i, code := range []string{"USD", "JPY"} {
		txn := domain.Transaction{ID: string(rune('a' + i)), AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "Buy", Amount: money.New(-500, code)}
		entry := a.AutoCategorizeTransaction(txn)
		if (entry.CategoryID != "") != (code == "USD") {
			t.Fatalf("entry %s category=%s", code, entry.CategoryID)
		}
		// Bypass the write seam solely to exercise legacy mixed-currency history.
		if err := a.store.PutTransaction(txn); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := a.PreviewApplyRules(); n != 1 {
		t.Fatalf("preview=%d", n)
	}
	if n, _, err := a.ApplyRulesWithCounts(); err != nil || n != 1 {
		t.Fatalf("backfill=%d %v", n, err)
	}
}

func TestTCW1OneRuleHonorsPrecedenceForTagsAndIndependentActions(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		first, second          rules.Rule
		target                 string
		wantTags               []string
		wantCategory, wantBill string
	}{
		{"tags respect the earlier winner", rules.Rule{SetTags: []string{"first"}}, rules.Rule{SetTags: []string{"second"}}, "second", nil, "", ""},
		{"category can follow tag-only rule", rules.Rule{SetTags: []string{"first"}}, rules.Rule{SetCategoryID: "category"}, "second", nil, "category", ""},
		{"bill-only action is independent", rules.Rule{SetTags: []string{"first"}}, rules.Rule{SetBillAccountID: "bill"}, "second", nil, "", "bill"},
		{"rename does not hide bill match", rules.Rule{RenameDesc: "Renamed", SetBillAccountID: "bill"}, rules.Rule{}, "first", nil, "", "bill"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := tcw1App(t)
			tc.first.ID = "first"
			tc.first.Order = 0
			tc.first.Match = "Coffee"
			tc.second.ID = "second"
			tc.second.Order = 1
			tc.second.Match = "Coffee"
			for _, r := range []rules.Rule{tc.first, tc.second} {
				if r.SetCategoryID == "" && len(r.SetTags) == 0 && r.RenameDesc == "" && r.SetBillAccountID == "" {
					continue
				}
				if err := a.PutRule(r); err != nil {
					t.Fatal(err)
				}
			}
			txn := domain.Transaction{ID: "t", AccountID: "chk", Date: tcw1Date(t, "2026-10-01"), Desc: "Coffee", Amount: money.New(-100, "USD")}
			if err := a.store.PutTransaction(txn); err != nil {
				t.Fatal(err)
			}
			if _, err := a.ApplyOneRule(tc.target); err != nil {
				t.Fatal(err)
			}
			got, _, _ := a.store.GetTransaction("t")
			if !reflect.DeepEqual(got.Tags, tc.wantTags) || got.CategoryID != tc.wantCategory || got.BillAccountID != tc.wantBill {
				t.Fatalf("txn=%+v", got)
			}
		})
	}
}

func TestTCW1PausedRecurringDoesNotFireBillWorkflow(t *testing.T) {
	a := tcw1App(t)
	if err := a.PutRecurring(domain.Recurring{ID: "r", Label: "Rent", AccountID: "chk", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: tcw1Date(t, "2026-10-01"), Paused: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutWorkflow(workflow.Workflow{ID: "bill", Name: "Bill", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerBillDue}, Actions: []workflow.Action{{Kind: workflow.ActionNotify, Message: "Due"}}}); err != nil {
		t.Fatal(err)
	}
	a.FireBillDueTrigger(tcw1Date(t, "2026-10-19"))
	if len(a.WorkflowRuns()) != 0 {
		t.Fatal("paused recurring fired bill-due automation")
	}
}

func TestTCW1DatasetWorkflowEffectsRunOnceAndNotifyAfterBatch(t *testing.T) {
	a := tcw1App(t)
	if err := a.PutRecurring(domain.Recurring{ID: "r", Label: "Rent", AccountID: "chk", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: tcw1Date(t, "2026-10-01"), Autopost: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.PutWorkflow(workflow.Workflow{ID: "post", Name: "Post", Enabled: true, Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Actions: []workflow.Action{{Kind: workflow.ActionPostRecurring}}}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	a.OnTxnMutated(func() {
		calls++
		if len(a.Transactions()) != 3 {
			t.Fatal("observer saw an incomplete import")
		}
	})
	result, err := a.ImportTransactionsCSV([]byte("date,desc,amount\n2026-10-01,One,-2\n2026-10-02,Two,-3\n"), "chk", "")
	if err != nil || result.Imported != 2 || calls != 1 || len(a.WorkflowRuns()) != 2 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
	}
}
