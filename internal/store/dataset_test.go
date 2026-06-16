package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func sampleDataset() Dataset {
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	return Dataset{
		Members: []domain.Member{{ID: "m1", Name: "Alice", IsDefault: true}},
		Accounts: []domain.Account{{
			ID: "a1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
			OpeningBalance: money.New(100000, "USD"), BalanceAsOf: asOf,
			Custom: map[string]any{"nickname": "main"},
		}},
		Categories:   []domain.Category{{ID: "c1", Name: "Food", Kind: domain.KindExpense}},
		Transactions: []domain.Transaction{{ID: "t1", AccountID: "a1", Date: asOf, Desc: "Groceries", CategoryID: "c1", Amount: money.New(-5000, "USD")}},
		Budgets:      []domain.Budget{{ID: "b1", Name: "Food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}},
		Goals:        []domain.Goal{{ID: "g1", Name: "Trip", Scope: domain.ScopeIndividual, OwnerID: "m1", TargetAmount: money.New(200000, "USD"), CurrentAmount: money.New(50000, "USD"), TargetDate: asOf}},
		Tasks:        []domain.Task{{ID: "k1", Title: "Pay rent", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Source: domain.SourceManual}},
		Rules:        []rules.Rule{{ID: "r1", Match: "coffee", SetCategoryID: "c1", SetTags: []string{"treats"}}},
		Documents: []domain.Document{{
			ID: "d1", Filename: "june.csv", Kind: domain.DocCSV, UploadedAt: asOf, AccountID: "a1",
			Status: domain.DocImported, Extracted: []domain.DocumentRow{{Date: "2026-06-01", Description: "Coffee", Amount: "-4.50", Category: "Food"}},
		}},
		SavedInsights: []domain.SavedInsight{{ID: "si1", Text: "You saved 18% this month.", CreatedAt: asOf}},
		Recurring: []domain.Recurring{{
			ID: "rec1", Label: "Salary", Amount: money.New(420000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: asOf, AccountID: "a1", CategoryID: "c1",
		}},
		AllocProfiles: []domain.AllocationProfile{{ID: "ap1", Name: "Aggressive", Returns: 3, Stability: 1, Liquidity: 1, DebtReduction: 2}},
		Formulas:      []domain.Formula{{ID: "f1", Name: "Savings rate", Expr: "(income - expense) / income * 100", Enabled: true}},
		Settings: Settings{
			BaseCurrency:       "USD",
			FXRates:            map[string]float64{"EUR": 1.1},
			OpenAIModel:        "gpt-x",
			FreshnessOverrides: map[string]int{"savings": 60},
		},
	}
}

func TestExportStampsSchemaVersion(t *testing.T) {
	out, err := Export(Dataset{})
	if err != nil {
		t.Fatalf("export error: %v", err)
	}
	ds, err := Import(out)
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if ds.SchemaVersion != SchemaVersion {
		t.Errorf("schema version = %d, want %d", ds.SchemaVersion, SchemaVersion)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	original := sampleDataset()

	first, err := Export(original)
	if err != nil {
		t.Fatalf("export error: %v", err)
	}
	imported, err := Import(first)
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	second, err := Export(imported)
	if err != nil {
		t.Fatalf("re-export error: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("round trip not lossless:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	// Spot-check a few decoded values.
	if imported.Accounts[0].OpeningBalance.Amount != 100000 {
		t.Errorf("opening balance = %d, want 100000", imported.Accounts[0].OpeningBalance.Amount)
	}
	if imported.Accounts[0].Custom["nickname"] != "main" {
		t.Errorf("custom field lost: %v", imported.Accounts[0].Custom)
	}
	if imported.Settings.BaseCurrency != "USD" || imported.Settings.FXRates["EUR"] != 1.1 {
		t.Errorf("settings lost: %+v", imported.Settings)
	}
	if len(imported.Rules) != 1 || imported.Rules[0].Match != "coffee" || len(imported.Rules[0].SetTags) != 1 {
		t.Errorf("rules lost: %+v", imported.Rules)
	}
	if len(imported.Documents) != 1 || imported.Documents[0].Kind != domain.DocCSV || len(imported.Documents[0].Extracted) != 1 {
		t.Errorf("documents lost: %+v", imported.Documents)
	}
	if len(imported.SavedInsights) != 1 || imported.SavedInsights[0].Text != "You saved 18% this month." {
		t.Errorf("saved insights lost: %+v", imported.SavedInsights)
	}
	if len(imported.Recurring) != 1 || imported.Recurring[0].Cadence != domain.CadenceMonthly || imported.Recurring[0].Amount.Amount != 420000 {
		t.Errorf("recurring lost: %+v", imported.Recurring)
	}
	if len(imported.AllocProfiles) != 1 || imported.AllocProfiles[0].Name != "Aggressive" || imported.AllocProfiles[0].Returns != 3 {
		t.Errorf("alloc profiles lost: %+v", imported.AllocProfiles)
	}
	if len(imported.Formulas) != 1 || imported.Formulas[0].Name != "Savings rate" || !imported.Formulas[0].Enabled {
		t.Errorf("formulas lost: %+v", imported.Formulas)
	}
}

func TestImportRejectsNewerSchema(t *testing.T) {
	future := []byte(`{"schemaVersion": 9999}`)
	if _, err := Import(future); err == nil {
		t.Error("expected error importing a newer schema version")
	}
}

func TestImportTreatsUnversionedAsCurrent(t *testing.T) {
	unversioned := []byte(`{"members":[{"id":"m1","name":"Bob"}]}`)
	ds, err := Import(unversioned)
	if err != nil {
		t.Fatalf("import error: %v", err)
	}
	if ds.SchemaVersion != SchemaVersion {
		t.Errorf("schema version = %d, want %d", ds.SchemaVersion, SchemaVersion)
	}
	if len(ds.Members) != 1 || ds.Members[0].Name != "Bob" {
		t.Errorf("members not parsed: %+v", ds.Members)
	}
}

func TestImportRejectsGarbage(t *testing.T) {
	if _, err := Import([]byte("not json")); err == nil {
		t.Error("expected error on invalid JSON")
	}
}
