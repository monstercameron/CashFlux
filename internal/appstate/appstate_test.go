package appstate

import (
	"bytes"
	"testing"

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
