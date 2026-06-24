// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAccessorsAndHandles(t *testing.T) {
	a := newApp(t, false)
	if len(a.Categories()) != 0 || len(a.Tasks()) != 0 || len(a.CustomFieldDefs()) != 0 {
		t.Error("empty app should have no categories/tasks/custom defs")
	}
	if len(a.CustomFieldDefsFor("account")) != 0 {
		t.Error("no account custom defs expected")
	}
	if len(a.FreshnessWindows()) == 0 {
		t.Error("freshness windows should include the built-in defaults")
	}
	if a.Store() == nil || a.Log() == nil || a.LogRing() == nil {
		t.Error("Store/Log/LogRing handles should be non-nil")
	}
}

func TestTaskPutDelete(t *testing.T) {
	a := newApp(t, false)
	task := domain.Task{ID: "k1", Title: "Pay rent", Status: domain.StatusOpen, Priority: domain.PriorityMedium}
	if err := a.PutTask(task); err != nil {
		t.Fatalf("PutTask: %v", err)
	}
	if len(a.Tasks()) != 1 {
		t.Fatalf("tasks = %d, want 1", len(a.Tasks()))
	}
	if err := a.DeleteTask("k1"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if len(a.Tasks()) != 0 {
		t.Error("task not deleted")
	}
	if err := a.PutTask(domain.Task{ID: "bad", Title: ""}); err == nil {
		t.Error("a task with no title should fail validation")
	}
}

func TestCreateFreshnessReminderTask(t *testing.T) {
	a := newApp(t, false)
	task, err := a.CreateFreshnessReminderTask("Update stale account balances")
	if err != nil {
		t.Fatalf("CreateFreshnessReminderTask: %v", err)
	}
	if task.ID == "" {
		t.Fatal("task ID should be generated")
	}
	if task.Title != "Update stale account balances" || task.Status != domain.StatusOpen ||
		task.Priority != domain.PriorityMedium || task.Source != domain.SourceNudge {
		t.Fatalf("task = %+v, want open medium nudge reminder", task)
	}
	tasks := a.Tasks()
	if len(tasks) != 1 || tasks[0].ID != task.ID || tasks[0].Source != domain.SourceNudge {
		t.Fatalf("persisted tasks = %+v, want generated nudge task", tasks)
	}
}

func TestDeleteEntities(t *testing.T) {
	a := newApp(t, false)

	if err := a.PutMember(domain.Member{ID: "m1", Name: "Alice"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	if err := a.DeleteMember("m1"); err != nil {
		t.Fatalf("DeleteMember: %v", err)
	}

	if err := a.PutCategory(domain.Category{ID: "c1", Name: "Food", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	if err := a.DeleteCategory("c1"); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	if err := a.PutAccount(domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", Desc: "Coffee", Amount: money.New(-100, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	if err := a.DeleteTransaction("t1"); err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}

	if err := a.PutBudget(domain.Budget{ID: "b1", Name: "Food", CategoryID: "c1", Period: domain.PeriodMonthly, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(10000, "USD")}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}
	if err := a.DeleteBudget("b1"); err != nil {
		t.Fatalf("DeleteBudget: %v", err)
	}

	if err := a.PutGoal(domain.Goal{ID: "g1", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(100000, "USD")}); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}
	if err := a.DeleteGoal("g1"); err != nil {
		t.Fatalf("DeleteGoal: %v", err)
	}

	if err := a.PutCustomFieldDef(customfields.Def{ID: "d1", EntityType: "account", Key: "branch", Label: "Branch", Type: customfields.TypeText}); err != nil {
		t.Fatalf("PutCustomFieldDef: %v", err)
	}
	if err := a.DeleteCustomFieldDef("d1"); err != nil {
		t.Fatalf("DeleteCustomFieldDef: %v", err)
	}
}

func TestDeleteTransactionWithTransferPair(t *testing.T) {
	a := newApp(t, false)
	for _, account := range []domain.Account{
		{ID: "checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
		{ID: "savings", Name: "Savings", Currency: "USD", Type: domain.TypeSavings, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared},
	} {
		if err := a.PutAccount(account); err != nil {
			t.Fatalf("PutAccount(%s): %v", account.ID, err)
		}
	}
	day := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", Desc: "Transfer", Amount: money.New(-5000, "USD"), Date: day},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", Desc: "Transfer", Amount: money.New(5000, "USD"), Date: day},
		{ID: "decoy", AccountID: "savings", TransferAccountID: "checking", Desc: "Other day", Amount: money.New(5000, "USD"), Date: day.AddDate(0, 0, 1)},
		{ID: "solo", AccountID: "checking", Desc: "Coffee", Amount: money.New(-700, "USD"), Date: day},
	}
	for _, tx := range txns {
		if err := a.PutTransaction(tx); err != nil {
			t.Fatalf("PutTransaction(%s): %v", tx.ID, err)
		}
	}

	if err := a.DeleteTransactionWithTransferPair("out"); err != nil {
		t.Fatalf("DeleteTransactionWithTransferPair: %v", err)
	}
	remaining := map[string]bool{}
	for _, tx := range a.Transactions() {
		remaining[tx.ID] = true
	}
	if remaining["out"] || remaining["in"] {
		t.Fatalf("transfer pair still present after delete: %v", remaining)
	}
	if !remaining["decoy"] || !remaining["solo"] {
		t.Fatalf("unrelated transactions should remain: %v", remaining)
	}

	if err := a.DeleteTransactionWithTransferPair("solo"); err != nil {
		t.Fatalf("DeleteTransactionWithTransferPair(solo): %v", err)
	}
	for _, tx := range a.Transactions() {
		if tx.ID == "solo" {
			t.Fatal("solo transaction still present after delete")
		}
	}
}

func TestPutSettingsAndRedactedExport(t *testing.T) {
	a := newApp(t, false)
	s := a.Settings()
	s.OpenAIKey = "sk-secret"
	s.BaseCurrency = "USD"
	if err := a.PutSettings(s); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	if a.Settings().OpenAIKey != "sk-secret" {
		t.Error("settings did not persist the key")
	}

	full, err := a.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	if !bytes.Contains(full, []byte("sk-secret")) {
		t.Error("the manual export should keep the key")
	}
	red, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatalf("ExportJSONRedacted: %v", err)
	}
	if bytes.Contains(red, []byte("sk-secret")) {
		t.Error("the redacted export must not contain the key")
	}
}

func TestTransactionsCSVAndImport(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAccount(domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutTransaction(domain.Transaction{ID: "t1", AccountID: "a1", Desc: "Coffee", Amount: money.New(-500, "USD"), Date: time.Now()}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}

	csv, err := a.TransactionsCSV(a.Transactions())
	if err != nil {
		t.Fatalf("TransactionsCSV: %v", err)
	}
	if len(csv) == 0 {
		t.Fatal("empty CSV")
	}

	// Round-trip: importing the exported CSV into a fresh app (with the same
	// account) restores the row through the validated write path.
	b := newApp(t, false)
	if err := b.PutAccount(domain.Account{ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatalf("PutAccount(b): %v", err)
	}
	n, _, err := b.ImportTransactionsCSV(csv, "")
	if err != nil {
		t.Fatalf("ImportTransactionsCSV: %v", err)
	}
	if n != 1 {
		t.Errorf("imported %d, want 1", n)
	}

	if err := b.PutCategory(domain.Category{ID: "food", Name: "Dining", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory(b): %v", err)
	}
	if err := b.PutMember(domain.Member{ID: "m1", Name: "Alex"}); err != nil {
		t.Fatalf("PutMember(b): %v", err)
	}
	reordered := []byte("amount,member,category,account,date,payee,desc,tags,cleared\n-12.34,alex,dining,checking,2026-06-18,Bistro,Dinner,meal;work,true\n")
	n, _, err = b.ImportTransactionsCSV(reordered, "")
	if err != nil {
		t.Fatalf("ImportTransactionsCSV reordered: %v", err)
	}
	if n != 1 {
		t.Errorf("friendly-name import count = %d, want 1", n)
	}
	var imported domain.Transaction
	for _, tx := range b.Transactions() {
		if tx.Payee == "Bistro" {
			imported = tx
			break
		}
	}
	if imported.AccountID != "a1" || imported.CategoryID != "food" || imported.MemberID != "m1" {
		t.Errorf("friendly columns resolved to account/category/member = %q/%q/%q", imported.AccountID, imported.CategoryID, imported.MemberID)
	}
	if imported.Amount.Amount != -1234 || imported.Amount.Currency != "USD" {
		t.Errorf("amount = %+v, want -12.34 USD", imported.Amount)
	}
	if imported.Date.Format("2006-01-02") != "2026-06-18" || !imported.Cleared || len(imported.Tags) != 2 || imported.Tags[1] != "work" {
		t.Errorf("date/cleared/tags not mapped: %+v", imported)
	}
}
