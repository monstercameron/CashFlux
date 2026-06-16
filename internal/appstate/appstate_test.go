package appstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func newApp(t *testing.T, seed bool) *App {
	t.Helper()
	a, err := New(&bytes.Buffer{}, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestNewSeedsSampleData(t *testing.T) {
	a := newApp(t, true)
	if len(a.Accounts()) == 0 {
		t.Error("expected seeded accounts")
	}
	if len(a.Transactions()) == 0 {
		t.Error("expected seeded transactions")
	}
	if a.Settings().BaseCurrency != "USD" {
		t.Errorf("base currency = %q, want USD", a.Settings().BaseCurrency)
	}
}

func TestNewEmpty(t *testing.T) {
	a := newApp(t, false)
	if len(a.Accounts()) != 0 {
		t.Errorf("expected empty store, got %d accounts", len(a.Accounts()))
	}
}

func TestPutAccountValidatesCustomFields(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCustomFieldDef(customfields.Def{
		ID: "cf1", EntityType: "account", Key: "branch", Label: "Branch", Type: customfields.TypeText, Required: true,
	}); err != nil {
		t.Fatalf("PutCustomFieldDef: %v", err)
	}

	acc := domain.Account{
		ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}

	// Missing the required custom field → rejected.
	if err := a.PutAccount(acc); err == nil {
		t.Error("expected error for missing required custom field")
	}

	// Wrong type for the custom field → rejected.
	acc.Custom = map[string]any{"branch": 42}
	if err := a.PutAccount(acc); err == nil {
		t.Error("expected error for wrong-typed custom field")
	}

	// Correct value → accepted and persisted.
	acc.Custom = map[string]any{"branch": "Downtown"}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("expected valid account to save, got %v", err)
	}
	got := a.Accounts()
	if len(got) != 1 || got[0].Custom["branch"] != "Downtown" {
		t.Errorf("custom value not persisted: %+v", got)
	}
}

func TestReassignCategory(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCategory(domain.Category{ID: "old", Name: "Old", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory old: %v", err)
	}
	if err := a.PutCategory(domain.Category{ID: "new", Name: "New", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory new: %v", err)
	}
	if err := a.PutAccount(domain.Account{
		ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutTransaction(domain.Transaction{
		ID: "t1", AccountID: "a1", CategoryID: "old", Desc: "Lunch",
		Date: time.Now(), Amount: money.New(-500, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	if err := a.PutBudget(domain.Budget{
		ID: "b1", Name: "Food", CategoryID: "old", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(10000, "USD"),
	}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}

	moved, err := a.ReassignCategory("old", "new")
	if err != nil {
		t.Fatalf("ReassignCategory: %v", err)
	}
	if moved != 2 {
		t.Errorf("moved = %d, want 2", moved)
	}
	for _, tr := range a.Transactions() {
		if tr.ID == "t1" && tr.CategoryID != "new" {
			t.Errorf("transaction not reassigned: %q", tr.CategoryID)
		}
	}
	for _, b := range a.Budgets() {
		if b.ID == "b1" && b.CategoryID != "new" {
			t.Errorf("budget not reassigned: %q", b.CategoryID)
		}
	}
}

func TestReassignOwner(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutMember(domain.Member{ID: "m1", Name: "Alex"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	if err := a.PutAccount(domain.Account{
		ID: "a1", Name: "Alex Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: "m1", Scope: domain.ScopeIndividual,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutGoal(domain.Goal{
		ID: "g1", Name: "Trip", OwnerID: "m1", Scope: domain.ScopeIndividual,
		TargetAmount: money.New(100000, "USD"),
	}); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	// Reassign to the group owner: scope becomes shared.
	moved, err := a.ReassignOwner("m1", domain.GroupOwnerID)
	if err != nil {
		t.Fatalf("ReassignOwner: %v", err)
	}
	if moved != 2 {
		t.Errorf("moved = %d, want 2", moved)
	}
	for _, ac := range a.Accounts() {
		if ac.ID == "a1" && (ac.OwnerID != domain.GroupOwnerID || ac.Scope != domain.ScopeShared) {
			t.Errorf("account not reassigned: owner=%q scope=%v", ac.OwnerID, ac.Scope)
		}
	}
	for _, g := range a.Goals() {
		if g.ID == "g1" && g.OwnerID != domain.GroupOwnerID {
			t.Errorf("goal not reassigned: owner=%q", g.OwnerID)
		}
	}
}

func TestPutValidatesAndPersists(t *testing.T) {
	a := newApp(t, false)

	// Invalid account is rejected.
	if err := a.PutAccount(domain.Account{ID: "x"}); err == nil {
		t.Error("expected validation error for incomplete account")
	}
	if len(a.Accounts()) != 0 {
		t.Error("invalid account should not be stored")
	}

	// Valid account persists.
	ok := domain.Account{
		ID: "a1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(1000, "USD"),
	}
	if err := a.PutAccount(ok); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if len(a.Accounts()) != 1 {
		t.Fatalf("expected 1 account, got %d", len(a.Accounts()))
	}

	if err := a.DeleteAccount("a1"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if len(a.Accounts()) != 0 {
		t.Error("account not deleted")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	a := newApp(t, true)
	data, err := a.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	b := newApp(t, false)
	if err := b.ImportJSON(data); err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if len(b.Accounts()) != len(a.Accounts()) {
		t.Errorf("imported accounts = %d, want %d", len(b.Accounts()), len(a.Accounts()))
	}

	again, _ := b.ExportJSON()
	if !bytes.Equal(data, again) {
		t.Error("export/import not lossless across apps")
	}
}

func TestLoadSampleAndWipe(t *testing.T) {
	a := newApp(t, false)
	if len(a.Accounts()) != 0 {
		t.Fatalf("expected empty store, got %d accounts", len(a.Accounts()))
	}

	if err := a.LoadSample(); err != nil {
		t.Fatalf("LoadSample: %v", err)
	}
	if len(a.Accounts()) == 0 || len(a.Transactions()) == 0 {
		t.Error("LoadSample should populate accounts and transactions")
	}

	if err := a.Wipe(); err != nil {
		t.Fatalf("Wipe: %v", err)
	}
	if len(a.Accounts()) != 0 || len(a.Transactions()) != 0 || len(a.Members()) != 0 {
		t.Error("Wipe should leave the store empty")
	}
}

func TestExportCSV(t *testing.T) {
	a := newApp(t, true)
	data, err := a.ExportCSV()
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if len(data) == 0 {
		t.Error("ExportCSV should produce output for seeded data")
	}
}

func TestInitSetsDefault(t *testing.T) {
	if err := Init(&bytes.Buffer{}, true); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Default == nil || len(Default.Accounts()) == 0 {
		t.Error("Init should set a seeded Default app")
	}
}
