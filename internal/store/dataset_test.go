// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/workflow"
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
		Categories: []domain.Category{{ID: "c1", Name: "Food", Kind: domain.KindExpense}},
		Transactions: []domain.Transaction{{
			ID: "t1", AccountID: "a1", Date: asOf, Desc: "Groceries", CategoryID: "c1", Amount: money.New(-5000, "USD"),
			Attachments: []domain.AttachmentRef{{ArtifactID: "art1", Name: "Receipt", Kind: "image", MIME: "image/png"}},
		}},
		Budgets: []domain.Budget{{ID: "b1", Name: "Food", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, CategoryID: "c1", Period: domain.PeriodMonthly, Limit: money.New(50000, "USD")}},
		Goals:   []domain.Goal{{ID: "g1", Name: "Trip", Scope: domain.ScopeIndividual, OwnerID: "m1", TargetAmount: money.New(200000, "USD"), CurrentAmount: money.New(50000, "USD"), TargetDate: asOf, GoalImageArtifactID: "art1", PausedUntil: asOf}},
		Tasks:   []domain.Task{{ID: "k1", Title: "Pay rent", Status: domain.StatusOpen, Priority: domain.PriorityHigh, Source: domain.SourceManual}},
		CustomFields: []customfields.Def{{
			ID: "cf1", EntityType: "account", Key: "branch", Label: "Branch",
			Type: customfields.TypeSelect, Options: []string{"east", "west"}, Required: true,
		}},
		Rules: []rules.Rule{{ID: "r1", Match: "coffee", SetCategoryID: "c1", SetTags: []string{"treats"}}},
		Documents: []domain.Document{{
			ID: "d1", Filename: "june.csv", Kind: domain.DocCSV, UploadedAt: asOf, AccountID: "a1",
			Status: domain.DocImported, Extracted: []domain.DocumentRow{{Date: "2026-06-01", Description: "Coffee", Amount: "-4.50", Category: "Food"}},
		}},
		SavedInsights: []domain.SavedInsight{{ID: "si1", Text: "You saved 18% this month.", CreatedAt: asOf}},
		Conversations: []domain.Conversation{{ID: "cv1", Title: "Groceries", CreatedAt: asOf, UpdatedAt: asOf,
			Messages: []domain.ChatMessage{{ID: "m1", Role: "user", Text: "How much on groceries?", CreatedAt: asOf}}}},
		Recurring: []domain.Recurring{{
			ID: "rec1", Label: "Salary", Amount: money.New(420000, "USD"), Cadence: domain.CadenceMonthly,
			NextDue: asOf, AccountID: "a1", CategoryID: "c1",
		}},
		AllocProfiles: []domain.AllocationProfile{{ID: "ap1", Name: "Aggressive", Returns: 3, Stability: 1, Liquidity: 1, DebtReduction: 2, GoalProgress: 1.5}},
		Formulas:      []domain.Formula{{ID: "f1", Name: "Savings rate", Expr: "(income - expense) / income * 100", Enabled: true}},
		Plans: []domain.Plan{{ID: "pl1", Name: "Runway", HorizonMonths: 6, StartBalance: 300000,
			Items: []domain.PlanItem{{ID: "pi1", Label: "Burn", Kind: domain.PlanItemRecurring, Amount: -40000}}}},
		CustomPages: []domain.CustomPage{{
			ID: "cp1", Slug: "my-money", Name: "My Money", Icon: "page", Order: 0, CreatedAt: asOf,
			Layout: []dashlayout.Item{{ID: "w1", ColSpan: 2, RowSpan: 1}},
			Widgets: []domain.PageWidget{{
				ID: "w1", Type: "kpi", Title: "Savings rate",
				Config:  widgetcfg.Config{"format": "percent"},
				Binding: domain.WidgetBinding{Expr: "(income - expense) / income * 100"},
			}},
		}},
		Artifacts: []domain.Artifact{{
			ID: "art1", Name: "Receipt", Kind: "image", MIME: "image/png",
			Bytes: []byte{0x89, 0x50, 0x4e, 0x47}, Size: 4, CreatedAt: asOf,
		}, {
			ID: "art2", Name: "Import", Kind: "csv",
			Columns: []string{"date", "amount"}, Rows: [][]string{{"2026-06-01", "12.50"}},
		}, {
			ID: "art3", Name: "Remote receipt", Kind: "image", MIME: "image/png",
			BlobRef: &domain.BlobRef{Hash: "abcd", MIME: "image/png", Size: 2048}, Size: 2048,
		}},
		Workflows: []workflow.Workflow{{
			ID: "wf1", Name: "Overspend alert", Enabled: true,
			Trigger: workflow.Trigger{Kind: workflow.TriggerTxnAdded}, Condition: "expense > income",
			Actions: []workflow.Action{{Kind: workflow.ActionCreateTask, Title: "Review spending"}},
		}},
		WorkflowRuns: []workflow.Run{{
			ID: "run1", WorkflowID: "wf1", At: "2026-06-01T00:00:00Z", Matched: true,
			Effects: []workflow.Effect{{Kind: workflow.ActionCreateTask, Summary: "Create task: Review spending", Title: "Review spending"}},
		}},
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
	if len(imported.Transactions) != 1 || len(imported.Transactions[0].Attachments) != 1 ||
		imported.Transactions[0].Attachments[0].ArtifactID != "art1" ||
		imported.Transactions[0].Attachments[0].MIME != "image/png" {
		t.Errorf("transaction attachments lost: %+v", imported.Transactions)
	}
	if len(imported.Goals) != 1 || imported.Goals[0].GoalImageArtifactID != "art1" ||
		imported.Goals[0].PausedUntil.IsZero() {
		t.Errorf("goal vision image / pause lost: %+v", imported.Goals)
	}
	if len(imported.Rules) != 1 || imported.Rules[0].Match != "coffee" || len(imported.Rules[0].SetTags) != 1 {
		t.Errorf("rules lost: %+v", imported.Rules)
	}
	if len(imported.CustomFields) != 1 || imported.CustomFields[0].Key != "branch" ||
		imported.CustomFields[0].Type != customfields.TypeSelect || len(imported.CustomFields[0].Options) != 2 ||
		!imported.CustomFields[0].Required {
		t.Errorf("custom field defs lost: %+v", imported.CustomFields)
	}
	if len(imported.Documents) != 1 || imported.Documents[0].Kind != domain.DocCSV || len(imported.Documents[0].Extracted) != 1 {
		t.Errorf("documents lost: %+v", imported.Documents)
	}
	if len(imported.SavedInsights) != 1 || imported.SavedInsights[0].Text != "You saved 18% this month." {
		t.Errorf("saved insights lost: %+v", imported.SavedInsights)
	}
	if len(imported.Conversations) != 1 || imported.Conversations[0].Title != "Groceries" || len(imported.Conversations[0].Messages) != 1 {
		t.Errorf("conversations lost: %+v", imported.Conversations)
	}
	if len(imported.Recurring) != 1 || imported.Recurring[0].Cadence != domain.CadenceMonthly || imported.Recurring[0].Amount.Amount != 420000 {
		t.Errorf("recurring lost: %+v", imported.Recurring)
	}
	if len(imported.AllocProfiles) != 1 || imported.AllocProfiles[0].Name != "Aggressive" ||
		imported.AllocProfiles[0].Returns != 3 || imported.AllocProfiles[0].GoalProgress != 1.5 {
		t.Errorf("alloc profiles lost: %+v", imported.AllocProfiles)
	}
	if len(imported.Formulas) != 1 || imported.Formulas[0].Name != "Savings rate" || !imported.Formulas[0].Enabled {
		t.Errorf("formulas lost: %+v", imported.Formulas)
	}
	if len(imported.Plans) != 1 || imported.Plans[0].Name != "Runway" || imported.Plans[0].HorizonMonths != 6 ||
		len(imported.Plans[0].Items) != 1 || imported.Plans[0].Items[0].Amount != -40000 {
		t.Errorf("plans lost: %+v", imported.Plans)
	}
	if len(imported.CustomPages) != 1 {
		t.Fatalf("custom pages lost: %+v", imported.CustomPages)
	}
	cp := imported.CustomPages[0]
	if cp.Slug != "my-money" || cp.Name != "My Money" ||
		len(cp.Layout) != 1 || cp.Layout[0].ID != "w1" || cp.Layout[0].ColSpan != 2 ||
		len(cp.Widgets) != 1 || cp.Widgets[0].Type != "kpi" ||
		cp.Widgets[0].Config["format"] != "percent" ||
		cp.Widgets[0].Binding.Expr != "(income - expense) / income * 100" {
		t.Errorf("custom page lost: %+v", cp)
	}
	if len(imported.Artifacts) != 3 {
		t.Fatalf("artifacts lost: %+v", imported.Artifacts)
	}
	if imported.Artifacts[0].Kind != "image" || len(imported.Artifacts[0].Bytes) != 4 ||
		imported.Artifacts[1].Kind != "csv" || len(imported.Artifacts[1].Rows) != 1 ||
		imported.Artifacts[1].Rows[0][1] != "12.50" {
		t.Errorf("artifact content lost: %+v", imported.Artifacts)
	}
	if imported.Artifacts[2].BlobRef == nil || imported.Artifacts[2].BlobRef.Hash != "abcd" ||
		imported.Artifacts[2].BlobRef.Size != 2048 || len(imported.Artifacts[2].Bytes) != 0 {
		t.Errorf("artifact blob ref lost: %+v", imported.Artifacts[2])
	}
	if len(imported.Workflows) != 1 || imported.Workflows[0].Name != "Overspend alert" ||
		!imported.Workflows[0].Enabled || imported.Workflows[0].Trigger.Kind != workflow.TriggerTxnAdded ||
		len(imported.Workflows[0].Actions) != 1 || imported.Workflows[0].Actions[0].Title != "Review spending" {
		t.Errorf("workflow lost: %+v", imported.Workflows)
	}
	if len(imported.WorkflowRuns) != 1 || !imported.WorkflowRuns[0].Matched ||
		len(imported.WorkflowRuns[0].Effects) != 1 {
		t.Errorf("workflow run lost: %+v", imported.WorkflowRuns)
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
