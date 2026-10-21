// SPDX-License-Identifier: MIT

package toolcite

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestForNamesTheToolAndItsScope(t *testing.T) {
	s := For("spending_by_category", json.RawMessage(`{"category":"Groceries","period":"last_month"}`))
	if s.Label != "Spending by category" {
		t.Fatalf("label = %q", s.Label)
	}
	if s.Scope != "category=Groceries · period=last_month" {
		t.Fatalf("scope = %q", s.Scope)
	}
	if got := s.Title(); got != "Spending by category · category=Groceries · period=last_month" {
		t.Fatalf("title = %q", got)
	}
}

func TestScopeJoinsSuppliedArgumentsInReadingOrder(t *testing.T) {
	s := For("list_transactions", json.RawMessage(`{"category":"Groceries","match":"trader","limit":20}`))
	if s.Scope != "category=Groceries · limit=20" {
		t.Fatalf("scope = %q, want match then category then limit", s.Scope)
	}
}

func TestScopeSkipsArgumentsTheCallDidNotSupply(t *testing.T) {
	s := For("list_transactions", json.RawMessage(`{"category":"coffee"}`))
	if s.Scope != "category=coffee" {
		t.Fatalf("scope = %q, want just the one supplied argument", s.Scope)
	}
	if strings.Contains(s.Title(), "··") {
		t.Fatalf("empty arguments left a gap in the title: %q", s.Title())
	}
}

func TestAToolWithNoScopeArgumentsTitlesCleanly(t *testing.T) {
	s := For("account_balances", json.RawMessage(`{}`))
	if s.Scope != "" {
		t.Fatalf("scope = %q, want empty", s.Scope)
	}
	if got := s.Title(); got != "Account balances" {
		t.Fatalf("title = %q", got)
	}
}

func TestAnUntabledToolStillCites(t *testing.T) {
	s := For("some_new_tool", json.RawMessage(`{}`))
	if s.Label == "" {
		t.Fatal("an untabled tool produced no label, so it would vanish from the citation list")
	}
	if s.Label != "Some new tool" {
		t.Fatalf("label = %q", s.Label)
	}
}

func TestMalformedArgumentsLoseTheScopeNotTheCitation(t *testing.T) {
	s := For("spending_by_category", json.RawMessage(`{"month":`))
	if s.Label != "Spending by category" {
		t.Fatalf("label = %q", s.Label)
	}
	if s.Scope != "" {
		t.Fatalf("scope = %q, want empty when the arguments could not be read", s.Scope)
	}
}

func TestNumbersInScopePrintWithoutFloatNoise(t *testing.T) {
	s := For("check_affordability", json.RawMessage(`{"amount":1200,"item":"a laptop"}`))
	if !strings.Contains(s.Scope, "amount=1200") || strings.Contains(s.Scope, ".00") {
		t.Fatalf("scope = %q", s.Scope)
	}
	frac := For("check_affordability", json.RawMessage(`{"amount":12.5}`))
	if frac.Scope != "amount=12.5" {
		t.Fatalf("fractional scope = %q", frac.Scope)
	}
}

func TestNewReadAndPlanningToolsCiteTheirActualSelectors(t *testing.T) {
	for _, tc := range []struct{ tool, args, want string }{
		{"data_manifest", `{"collection":"accounts"}`, "collection=accounts"},
		{"query_data", `{"collection":"transactions","ids":["tx-1"],"equals":{"amount":12345678901234567890.12345},"dateField":"date","from":"2026-01-01","through":"2026-01-31"}`, `collection=transactions · ids=["tx-1"] · equals={"amount":12345678901234567890.12345} · dateField=date · from=2026-01-01 · through=2026-01-31`},
		{"read_record", `{"collection":"transactions","id":"tx-7","field":"amount","snapshot":"opaque-secret-token","offset":10}`, "collection=transactions · id=tx-7 · field=amount · offset=10 · snapshot=continuation snapshot"},
		{"get_account_details", `{"account_id":"acct-1"}`, "account_id=acct-1"},
		{"project_account", `{"account_id":"acct-1","horizon_days":90,"as_of":"2026-08-01"}`, "account_id=acct-1 · horizon_days=90 · as_of=2026-08-01"},
		{"goal_funding_plan", `{"goal_id":"goal-1","offset":20,"limit":10}`, "goal_id=goal-1 · offset=20 · limit=10"},
		{"list_goal_reservations", `{"account_id":"acct-1"}`, "account_id=acct-1"},
		{"list_recurring", `{"recurring_id":"rec-1","account_id":"acct-1","from":"2026-01-01","before":"2026-02-01","include_paused":true}`, "recurring_id=rec-1 · account_id=acct-1 · include_paused=yes · from=2026-01-01 · before=2026-02-01"},
		{"evaluate_formula", `{"expression":"net_worth * 0.04","snapshot":"opaque-secret-token"}`, "expression=net_worth * 0.04 · snapshot=continuation snapshot"},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			got := For(tc.tool, json.RawMessage(tc.args))
			if got.Scope != tc.want {
				t.Fatalf("scope = %q, want %q", got.Scope, tc.want)
			}
			if strings.Contains(got.Scope, "opaque-secret-token") {
				t.Fatal("opaque snapshot token leaked into citation scope")
			}
		})
	}
}

func TestScopePreservesLargeExactJSONNumbers(t *testing.T) {
	got := For("check_affordability", json.RawMessage(`{"amount":90071992547409931234567890.123456789}`))
	want := "amount=90071992547409931234567890.123456789"
	if got.Scope != want {
		t.Fatalf("scope = %q, want exact decimal %q", got.Scope, want)
	}
}

func TestNumericSpotsAClaimWorthChecking(t *testing.T) {
	for _, tc := range []struct {
		reply string
		want  bool
	}{
		{"You spent $312 on groceries.", true},
		{"Your budgets all look fine — nothing over.", false},
		{"", false},
		{"Try setting one up under Budgets.", false},
	} {
		if got := Numeric(tc.reply); got != tc.want {
			t.Errorf("Numeric(%q) = %v, want %v", tc.reply, got, tc.want)
		}
	}
}
