// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAssistantAuthoringTransactionConditionRunsAtCanonicalThreshold(t *testing.T) {
	app := assistantAuthoringApp(t)
	if err := app.PutAccount(domain.Account{ID: "card", Name: "Card", Currency: "USD", Class: domain.ClassAsset,
		Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}); err != nil {
		t.Fatal(err)
	}
	tool := assistantAuthoringTool(t, app, "create_workflow")
	raw := json.RawMessage(`{"name":"Review large charge","trigger":"txn-added","condition":"txn_abs > 500","actions":[{"kind":"createTask","title":"Review large charge"}]}`)
	if preview := tool.preview(raw); !strings.Contains(preview, "enabled workflow") || !strings.Contains(preview, "txn_abs > 500") {
		t.Fatalf("advertised condition cannot be previewed: %q", preview)
	}
	if out := tool.run(raw); !strings.HasPrefix(out, "Created the workflow") {
		t.Fatal(out)
	}
	if workflows := app.Workflows(); len(workflows) != 1 || workflows[0].Condition != "txn_abs > 500" {
		t.Fatalf("saved condition changed: %+v", workflows)
	}
	for i, amount := range []int64{-49999, -50000, -50001} {
		if err := app.PutTransaction(domain.Transaction{ID: fmt.Sprintf("threshold-%d", i), AccountID: "card", Desc: "Card charge",
			Payee: "Market", Date: time.Now(), Amount: money.New(amount, "USD")}); err != nil {
			t.Fatal(err)
		}
		want := 0
		if i == 2 {
			want = 1
		}
		tasks := app.Tasks()
		if len(tasks) != want {
			t.Fatalf("amount %d produced %d tasks, want %d; txn_abs must use major units", amount, len(tasks), want)
		}
		if want == 1 && (tasks[0].Title != "Review large charge" || tasks[0].Status != domain.StatusOpen) {
			t.Fatalf("wrong workflow effect: %+v", tasks[0])
		}
	}
}

func TestAssistantAuthoringTriggerConditionsRejectUnavailableNames(t *testing.T) {
	app := assistantAuthoringApp(t)
	for _, trigger := range []string{"manual", "scheduled", "budget-exceeded", "goal-reached", "bill-due", "txn-added"} {
		conditions := []string{"txn_abs > 500", `contains(txn_payee, "Market")`, "txn_payee_typical > 1"}
		if trigger == "txn-added" {
			conditions = []string{"txn_not_a_real_field > 0", "not_a_real_variable > 0"}
		}
		for _, condition := range conditions {
			t.Run(trigger+"/"+condition, func(t *testing.T) {
				tool := assistantAuthoringTool(t, app, "create_workflow")
				raw, err := json.Marshal(map[string]any{"name": "Invalid condition", "trigger": trigger,
					"condition": condition, "actions": []map[string]string{{"kind": "createTask", "title": "Never created"}}})
				if err != nil {
					t.Fatal(err)
				}
				rev := app.Rev()
				if preview := tool.preview(raw); !strings.HasPrefix(preview, "Can't create") {
					t.Fatalf("unavailable variable got actionable preview: %q", preview)
				}
				if out := tool.run(raw); !strings.HasPrefix(out, "Can't create") || app.Rev() != rev || len(app.Workflows()) != 0 || len(app.Tasks()) != 0 {
					t.Fatalf("unavailable/unknown variable was saved: %q", out)
				}
			})
		}
	}
}
