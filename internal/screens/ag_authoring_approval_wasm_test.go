// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func assistantAuthoringApp(t *testing.T) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	for _, category := range []domain.Category{
		{ID: "food", Name: "Food", Kind: domain.KindExpense},
		{ID: "food-other", Name: "Food delivery", Kind: domain.KindExpense},
	} {
		if err := app.PutCategory(category); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func assistantAuthoringTool(t *testing.T, app *appstate.App, name string) chatTool {
	t.Helper()
	for _, tool := range agToolsAuthoring(app, "USD", currency.Rates{Base: "USD"}) {
		if tool.spec.Function.Name == name {
			return tool
		}
	}
	t.Fatalf("missing authoring tool %s", name)
	return chatTool{}
}

func TestAssistantAuthoringRulePreviewAndReceiptPromiseOnlySave(t *testing.T) {
	app := assistantAuthoringApp(t)
	if err := app.PutAccount(domain.Account{ID: "cash", Name: "Cash", Currency: "USD", Class: domain.ClassAsset,
		Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "market", AccountID: "cash", Desc: "Market purchase", Payee: "Market",
		Amount: money.New(-1500, "USD"), Date: time.Now()}); err != nil {
		t.Fatal(err)
	}
	tool := assistantAuthoringTool(t, app, "create_rule")
	raw := json.RawMessage(`{"match":"Market","category":"Food","tags":["groceries"],"rename_desc":"Weekly shopping"}`)
	before := app.Transactions()
	preview := tool.preview(raw)
	for _, want := range []string{"Save rule ID", "category ID food", "Food", "groceries", "Weekly shopping", "Matches 1", "existing transactions stay unchanged"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview %q lacks %q", preview, want)
		}
	}
	if out := tool.run(raw); !strings.Contains(out, "Created the rule") || !strings.Contains(out, "Existing transactions stay unchanged") {
		t.Fatal(out)
	}
	if len(app.Rules()) != 1 || !strings.Contains(preview, app.Rules()[0].ID) || !reflect.DeepEqual(app.Transactions(), before) {
		t.Fatal("save changed existing transactions or used a different rule identity")
	}
	rev := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already saved") || app.Rev() != rev || len(app.Rules()) != 1 {
		t.Fatalf("retry duplicated or rewrote rule: %q", out)
	}
}

func TestAssistantAuthoringWorkflowPreviewNamesEveryPersistedEffect(t *testing.T) {
	app := assistantAuthoringApp(t)
	tool := assistantAuthoringTool(t, app, "create_workflow")
	notes := strings.Repeat("full notes ", 20)
	message := strings.Repeat("notification content ", 12)
	args := map[string]any{"name": "Review market", "trigger": "txn-added", "condition": "income > 5", "actions": []map[string]string{
		{"kind": "createTask", "title": "Review the receipt", "message": notes},
		{"kind": "notify", "message": message},
		{"kind": "setCategory", "category": "Food"},
		{"kind": "addTag", "tag": "weekly-review"},
		{"kind": "flagReview"},
	}}
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	preview := tool.preview(raw)
	for _, want := range []string{"enabled workflow", "Review market", "txn-added", "income > 5", "Review the receipt",
		strings.TrimSpace(notes), strings.TrimSpace(message), "Food", "category ID food", "weekly-review", "needs-review", "1.", "5."} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview omitted effect %q: %s", want, preview)
		}
	}
	if out := tool.run(raw); !strings.Contains(out, "Created the workflow") {
		t.Fatal(out)
	}
	wfs := app.Workflows()
	if len(wfs) != 1 || len(wfs[0].Actions) != 5 || !wfs[0].Enabled || !strings.Contains(preview, wfs[0].ID) {
		t.Fatalf("saved workflow disagrees with preview: %+v", wfs)
	}
	if wfs[0].Actions[0].Notes != strings.TrimSpace(notes) || wfs[0].Actions[1].Message != strings.TrimSpace(message) ||
		wfs[0].Actions[2].CategoryID != "food" || wfs[0].Actions[3].Tag != "weekly-review" {
		t.Fatal("saved action details differ from the full preview")
	}
	rev := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already saved") || app.Rev() != rev || len(app.Workflows()) != 1 {
		t.Fatalf("workflow retry duplicated/rewrote it: %q", out)
	}
}

func TestAssistantAuthoringInvalidArgumentsNeverWrite(t *testing.T) {
	for _, tc := range []struct{ tool, raw string }{
		{"create_rule", `{"match":`},
		{"create_rule", `null`},
		{"create_rule", `[]`},
		{"create_rule", `{"match":"Market","tags":["x"],"unexpected":true}`},
		{"create_rule", `{"match":"Market","tags":[42]}`},
		{"create_rule", `{"match":"Market","tags":["x"],"category":null}`},
		{"create_rule", `{"MATCH":"Market","tags":["x"]}`},
		{"create_rule", `{"match":"Market","tags":["x"]} {}`},
		{"create_rule", `{"match":"Market"}`},
		{"create_rule", `{"category":"Food"}`},
		{"create_rule", `{"match":"Market","category":"missing"}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"notify","message":"Hello","extra":true}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"createTask","title":"Review","message":null}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"notify","Message":"Hello"}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"applyRules"}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"notify","message":"Hello","tag":"ignored"}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"notify","message":" "}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"createTask"}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[{"kind":"addTag","tag":"x"}]}`},
		{"create_workflow", `{"name":"X","trigger":"txn-added","actions":[{"kind":"addTag","tag":" "}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","actions":[]}`},
		{"create_workflow", `{"name":"X","trigger":"unknown","actions":[{"kind":"notify","message":"Hi"}]}`},
		{"create_workflow", `{"name":"X","trigger":"manual","condition":"unknown_variable > 0","actions":[{"kind":"notify","message":"Hi"}]}`},
	} {
		t.Run(tc.tool+"/"+tc.raw, func(t *testing.T) {
			app := assistantAuthoringApp(t)
			tool := assistantAuthoringTool(t, app, tc.tool)
			raw := json.RawMessage(tc.raw)
			rev := app.Rev()
			if out := tool.preview(raw); !strings.HasPrefix(out, "Can't create") {
				t.Fatalf("invalid arguments offered an actionable preview: %q", out)
			}
			if out := tool.run(raw); !strings.HasPrefix(out, "Can't create") || len(app.Rules()) != 0 || len(app.Workflows()) != 0 || app.Rev() != rev {
				t.Fatalf("invalid input wrote or hid failure: %q", out)
			}
		})
	}
}

func TestAssistantAuthoringLiveCategoryBindingAndFailureRetry(t *testing.T) {
	for _, toolName := range []string{"create_rule", "create_workflow"} {
		t.Run(toolName, func(t *testing.T) {
			app := assistantAuthoringApp(t)
			tool := assistantAuthoringTool(t, app, toolName)
			raw := json.RawMessage(`{"match":"Market","category":"Food"}`)
			if toolName == "create_workflow" {
				raw = json.RawMessage(`{"name":"Review","trigger":"txn-added","actions":[{"kind":"setCategory","category":"Food"}]}`)
			}
			tool.preview(raw)
			// Reassign the same name to another category while the card is open.
			if err := app.PutCategory(domain.Category{ID: "food", Name: "Previous food", Kind: domain.KindExpense}); err != nil {
				t.Fatal(err)
			}
			if err := app.PutCategory(domain.Category{ID: "replacement", Name: "Food", Kind: domain.KindExpense}); err != nil {
				t.Fatal(err)
			}
			rev := app.Rev()
			if out := tool.run(raw); !strings.Contains(out, "changed after the preview") || app.Rev() != rev || len(app.Rules()) != 0 || len(app.Workflows()) != 0 {
				t.Fatalf("live validation silently retargeted the category: %q", out)
			}
			// A new approval can accept the live target; a write failure stays retryable.
			if out := tool.preview(raw); !strings.Contains(out, "replacement") {
				t.Fatal("new preview did not resolve live category identity")
			}
			app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
			if out := tool.run(raw); !strings.HasPrefix(out, "Couldn't save") || app.Rev() != rev {
				t.Fatalf("failed write changed the store or hid failure: %q", out)
			}
			app.SetActiveRoleFunc(nil)
			if out := tool.run(raw); !strings.HasPrefix(out, "Created") {
				t.Fatal(out)
			}
			if len(app.Rules())+len(app.Workflows()) != 1 {
				t.Fatal("retry did not save exactly one record")
			}
		})
	}
}

func TestAssistantAuthoringRefusesAmbiguousCategoryNames(t *testing.T) {
	app := assistantAuthoringApp(t)
	if err := app.PutCategory(domain.Category{ID: "household", Name: "Household", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	// Identical leaf names are valid under distinct parents. The Assistant's
	// plain-name selector must refuse their ambiguity rather than pick a row.
	if err := app.PutCategory(domain.Category{ID: "duplicate", Name: "Food", ParentID: "household", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"create_rule", "create_workflow"} {
		tool := assistantAuthoringTool(t, app, name)
		raw := json.RawMessage(`{"match":"Market","category":"Food"}`)
		if name == "create_workflow" {
			raw = json.RawMessage(`{"name":"Review","trigger":"txn-added","actions":[{"kind":"setCategory","category":"Food"}]}`)
		}
		rev := app.Rev()
		if out := tool.run(raw); !strings.Contains(out, "unique category") || app.Rev() != rev {
			t.Fatalf("ambiguous name selected the first category: %q", out)
		}
	}
}
