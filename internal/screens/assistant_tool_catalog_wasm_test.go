// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/toolperm"
)

func TestAssistantEnabledToolCatalogIsCompleteAndUnambiguous(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	tools := buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull)
	// The relay accepts at most 64 tools and 32 KiB per argument schema.
	// Registering a capability that cannot reach that transport is a regression.
	if len(tools) > 64 {
		t.Fatalf("enabled catalog has %d tools, exceeding the relay limit of 64", len(tools))
	}
	t.Logf("enabled assistant tool catalog: %d tools", len(tools))
	seen := map[string]bool{}
	for _, tool := range tools {
		name := tool.spec.Function.Name
		if name == "" || seen[name] || tool.run == nil || tool.spec.Function.Description == "" {
			t.Fatalf("incomplete or duplicate enabled tool %q", name)
		}
		seen[name] = true
		if len(tool.spec.Function.Parameters) > 32<<10 {
			t.Fatalf("%s argument schema exceeds the relay limit of 32 KiB", name)
		}
		var schema struct {
			Type       string                     `json:"type"`
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
		}
		if err := json.Unmarshal(tool.spec.Function.Parameters, &schema); err != nil || schema.Type != "object" {
			t.Fatalf("%s does not advertise an object argument schema: %v", name, err)
		}
		for _, required := range schema.Required {
			if _, ok := schema.Properties[required]; !ok {
				t.Errorf("%s requires undefined argument %q", name, required)
			}
		}
		if tool.mutates {
			permission := toolperm.For(name, json.RawMessage(`{}`))
			if tool.preview == nil || !permission.Known || permission.ReadOnly() {
				t.Errorf("enabled mutation %s lacks a preview or truthful write permission", name)
			}
		}
		if !aicontext.ToolAllowed(name, aicontext.TierFull) {
			t.Errorf("full-detail tool %s unexpectedly withheld", name)
		}
	}
	for _, name := range []string{
		"data_manifest", "query_data", "read_record", "list_formula_metrics", "evaluate_formula",
		"financial_summary", "spending_by_category", "spending_breakdown", "account_balances", "list_budgets", "check_affordability",
		"get_account_details", "project_account", "list_goals", "goal_funding_plan", "list_goal_reservations", "list_recurring",
		"list_report_sections", "report_section", "trace_report_row", "complete_task", "create_rule", "create_workflow",
		"add_goal_contribution", "add_transfer", "delete_transaction", "merge_duplicate_transactions", "dismiss_flagged_activity",
	} {
		if !seen[name] {
			t.Errorf("Assistant capability %s is not registered", name)
		}
	}
	for _, name := range []string{"query_data", "read_record", "search_documents", "gather_tax_records", "list_recurring", "project_account", "trace_report_row"} {
		if !seen[name] {
			t.Errorf("detail gate test refers to unregistered tool %s", name)
		} else if aicontext.ToolAllowed(name, aicontext.TierAggregatesOnly) {
			t.Errorf("aggregate-only conversation exposes detail tool %s", name)
		}
	}
}
