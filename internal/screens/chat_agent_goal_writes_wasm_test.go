// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func assistantGoalWriteFixture(t *testing.T, code string) (*appstate.App, domain.Goal, chatTool) {
	t.Helper()
	app := primaryAccountsGoalsApp(t)
	g := domain.Goal{ID: "write-goal", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(100000, code), CurrentAmount: money.Zero(code), TargetDate: time.Now().AddDate(0, 6, 0), Notes: "Keep these notes"}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	return app, g, agToolsGoalWrites(app, "USD", currency.Rates{Base: "USD"})[0]
}

func assistantStoredGoal(t *testing.T, app *appstate.App, id string) domain.Goal {
	t.Helper()
	all, err := app.Store().ListGoals()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range all {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("goal %s missing", id)
	return domain.Goal{}
}

func TestAssistantGoalContributionRegisteredOnceAndRequiresApproval(t *testing.T) {
	app, _, tool := assistantGoalWriteFixture(t, "USD")
	count := 0
	for _, got := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		if got.spec.Function.Name == "add_goal_contribution" {
			count++
			if !got.mutates || got.preview == nil || !strings.Contains(string(got.spec.Function.Parameters), `"goal_id"`) {
				t.Fatal("contribution lost approval gating or exact goal ID")
			}
		}
	}
	if count != 1 {
		t.Fatalf("contribution registrations=%d, want1", count)
	}
	revision := app.Rev()
	if out := tool.run(json.RawMessage(`{"goal_id":"write-goal","amount":1}`)); !strings.Contains(out, "Review and approve") || app.Rev() != revision {
		t.Fatalf("unreviewed contribution executed: %s", out)
	}
}

func TestAssistantGoalContributionCanonicalHistoryUndoAndRetry(t *testing.T) {
	app, g, tool := assistantGoalWriteFixture(t, "EUR")
	raw := json.RawMessage(`{"goal_id":"write-goal","amount":"12.34","currency":"EUR"}`)
	preview := tool.preview(raw)
	for _, want := range []string{"12.34 EUR", g.Name, g.ID, "memo-only", "target uses EUR"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("approval %q lacks %q", preview, want)
		}
	}
	if out := tool.run(raw); !strings.Contains(out, "Recorded") || !strings.Contains(out, "12.34 EUR") {
		t.Fatal(out)
	}
	updated := assistantStoredGoal(t, app, g.ID)
	if updated.CurrentAmount != money.New(1234, "EUR") || len(updated.Contributions) != 1 || updated.Contributions[0].Amount != money.New(1234, "EUR") || updated.Contributions[0].TxnID != "" || updated.LastReviewedAt.IsZero() || updated.Notes != g.Notes || len(app.Transactions()) != 0 {
		t.Fatalf("canonical contribution/history missing: %+v", updated)
	}
	revision := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already recorded") || app.Rev() != revision {
		t.Fatalf("same approval duplicated contribution: %s", out)
	}
	undone, ok, err := app.UndoLastContribution(updated)
	if err != nil || !ok || undone != money.New(1234, "EUR") {
		t.Fatalf("canonical undo failed: %v %v %v", undone, ok, err)
	}
	if after := assistantStoredGoal(t, app, g.ID); after.CurrentAmount != g.CurrentAmount || len(after.Contributions) != 0 {
		t.Fatalf("undo did not restore savings/history: %+v", after)
	}
	_ = tool.preview(raw)
	if out := tool.run(raw); !strings.Contains(out, "Recorded") {
		t.Fatalf("new explicit approval could not record a contribution: %s", out)
	}
}

func TestAssistantGoalContributionExactMajorParsingAndForeignGoalCurrency(t *testing.T) {
	app, g, tool := assistantGoalWriteFixture(t, "EUR")
	s := app.Settings()
	s.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	g.CurrentAmount = money.New(100, "EUR")
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"goal_id":"write-goal","amount":0.01,"currency":"EUR"}`)
	if preview := tool.preview(raw); !strings.Contains(preview, "0.01 EUR") || !strings.Contains(preview, "target uses EUR") {
		t.Fatalf("foreign goal currency relabeled: %s", preview)
	}
	if out := tool.run(raw); !strings.Contains(out, "Recorded") {
		t.Fatal(out)
	}
	if got := assistantStoredGoal(t, app, g.ID); got.CurrentAmount != money.New(101, "EUR") || got.Contributions[0].Amount.Currency != "EUR" {
		t.Fatalf("saved contribution currency changed: %+v", got)
	}
	large := domain.Goal{ID: "large", Name: "Large", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(math.MaxInt64, "USD"), CurrentAmount: money.Zero("USD")}
	if err := app.PutGoal(large); err != nil {
		t.Fatal(err)
	}
	raw = json.RawMessage(`{"goal_id":"large","amount":90071992547409.93}`)
	_ = tool.preview(raw)
	if out := tool.run(raw); !strings.Contains(out, "Recorded") {
		t.Fatal(out)
	}
	if got := assistantStoredGoal(t, app, large.ID); got.CurrentAmount.Amount != 9007199254740993 {
		t.Fatalf("numeric JSON passed through float: %d", got.CurrentAmount.Amount)
	}
	g.CurrentAmount = money.New(100, "USD")
	if err := app.Store().PutGoal(g); err != nil {
		t.Fatal(err)
	}
	revision := app.Rev()
	raw = json.RawMessage(`{"goal_id":"write-goal","amount":1,"currency":"USD"}`)
	if preview := tool.preview(raw); !strings.Contains(preview, "Can't record") {
		t.Fatalf("unsupported legacy currency mismatch was offered: %s", preview)
	}
	if out := tool.run(raw); !strings.Contains(out, "Can't record") || app.Rev() != revision || !reflect.DeepEqual(assistantStoredGoal(t, app, g.ID), g) {
		t.Fatalf("legacy saved currency was rewritten: %s", out)
	}
}

func TestAssistantGoalContributionRejectsMalformedNonpositiveAndUnknownTargets(t *testing.T) {
	app, _, tool := assistantGoalWriteFixture(t, "USD")
	for _, raw := range []string{
		`{"goal":"","amount":1}`, `{"goal_id":" ","amount":1}`, `{"goal_id":"missing","amount":1}`,
		`{"goal_id":"write-goal","amount":0}`, `{"goal_id":"write-goal","amount":-1}`, `{"goal_id":"write-goal","amount":null}`,
		`{"goal_id":"write-goal","amount":1.001}`, `{"goal_id":"write-goal","amount":1e2}`, `{"goal_id":"write-goal","amount":"NaN"}`,
		`{"goal_id":"write-goal","amount":"92233720368547758.08"}`, `{"goal_id":"write-goal","amount":1,"currency":"EUR"}`,
		`{"goal_id":"write-goal","amount":1,"date":"not-a-date"}`, `{"goal_id":"write-goal","amount":1,"post_ledger":true}`,
		`{"goal_id":"write-goal","amount":1} {}`, `not-json`,
	} {
		before, revision := assistantStoredGoal(t, app, "write-goal"), app.Rev()
		preview := tool.preview(json.RawMessage(raw))
		out := tool.run(json.RawMessage(raw))
		if !strings.Contains(preview, "Can't record") || !strings.Contains(out, "Can't record") || app.Rev() != revision || !reflect.DeepEqual(assistantStoredGoal(t, app, "write-goal"), before) {
			t.Fatalf("invalid input mutated goal: %s -> %s / %s", raw, preview, out)
		}
	}
	jpy := domain.Goal{ID: "jpy", Name: "Yen", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(100, "JPY"), CurrentAmount: money.Zero("JPY")}
	if err := app.PutGoal(jpy); err != nil {
		t.Fatal(err)
	}
	if preview := tool.preview(json.RawMessage(`{"goal_id":"jpy","amount":1.1}`)); !strings.Contains(preview, "decimal places") {
		t.Fatalf("JPY precision accepted: %s", preview)
	}
}

func TestAssistantGoalContributionNameResolutionIsUniqueAndExactFirst(t *testing.T) {
	app, g, tool := assistantGoalWriteFixture(t, "USD")
	other := g
	other.ID = "other"
	other.Name = "Trip extras"
	if err := app.PutGoal(other); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"goal":"trip","amount":1}`, `{"goal":"extras","amount":1}`} {
		if preview := tool.preview(json.RawMessage(raw)); !strings.Contains(preview, "memo-only") {
			t.Fatalf("unique name not resolved: %s", preview)
		}
	}
	if preview := tool.preview(json.RawMessage(`{"goal":"tri","amount":1}`)); !strings.Contains(preview, "several goals") || !strings.Contains(preview, "write-goal") || !strings.Contains(preview, "other") {
		t.Fatalf("ambiguous name silently selected first: %s", preview)
	}
	other.Name = g.Name
	if err := app.PutGoal(other); err != nil {
		t.Fatal(err)
	}
	if preview := tool.preview(json.RawMessage(`{"goal":"Trip","amount":1}`)); !strings.Contains(preview, "several goals") {
		t.Fatalf("duplicate exact names silently selected first: %s", preview)
	}
	for _, change := range []func(*domain.Goal){func(g *domain.Goal) { g.Archived = true }, func(g *domain.Goal) { g.Archived = false; g.Kind = domain.GoalKindChecklist }} {
		change(&g)
		if err := app.PutGoal(g); err != nil {
			t.Fatal(err)
		}
		if preview := tool.preview(json.RawMessage(`{"goal_id":"write-goal","amount":1}`)); !strings.Contains(preview, "Can't record") {
			t.Fatalf("unsupported goal accepted: %s", preview)
		}
	}
}

func TestAssistantGoalContributionRejectsChangedApprovalGoalOrRates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*appstate.App, domain.Goal) error
	}{
		{"name", func(app *appstate.App, g domain.Goal) error { g.Name = "Renamed"; return app.PutGoal(g) }},
		{"saved", func(app *appstate.App, g domain.Goal) error {
			g.CurrentAmount = money.New(5, "USD")
			return app.PutGoal(g)
		}},
		{"archive", func(app *appstate.App, g domain.Goal) error { g.Archived = true; return app.PutGoal(g) }},
		{"currency", func(app *appstate.App, g domain.Goal) error {
			g.CurrentAmount = money.Zero("EUR")
			g.TargetAmount = money.New(1000, "EUR")
			return app.PutGoal(g)
		}},
		{"delete", func(app *appstate.App, g domain.Goal) error { return app.DeleteGoal(g.ID) }},
		{"rates", func(app *appstate.App, g domain.Goal) error {
			s := app.Settings()
			s.FXRates = map[string]float64{"EUR": 2}
			return app.PutSettings(s)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, g, tool := assistantGoalWriteFixture(t, "USD")
			raw := json.RawMessage(`{"goal_id":"write-goal","amount":1}`)
			if preview := tool.preview(raw); !strings.Contains(preview, "memo-only") {
				t.Fatal(preview)
			}
			if err := tc.change(app, g); err != nil {
				t.Fatal(err)
			}
			revision := app.Rev()
			goalsBefore := app.Goals()
			out := tool.run(raw)
			if !strings.Contains(strings.ToLower(out), "review") || app.Rev() != revision || !reflect.DeepEqual(app.Goals(), goalsBefore) {
				t.Fatalf("stale approval mutated live state: %s", out)
			}
		})
	}
}

func TestAssistantGoalContributionUnknownFXOverflowAndAtomicFailureLeaveState(t *testing.T) {
	for _, tc := range []struct {
		name          string
		configure     func(domain.Goal) domain.Goal
		amount        string
		previewReject bool
	}{
		{"missing FX", func(g domain.Goal) domain.Goal {
			g.Allocations = []domain.GoalAllocation{{AccountID: "foreign", Amount: money.New(1, "EUR")}}
			return g
		}, `1`, true},
		{"saved overflow", func(g domain.Goal) domain.Goal {
			g.CurrentAmount = money.New(math.MaxInt64, "USD")
			g.TargetAmount = money.New(math.MaxInt64, "USD")
			return g
		}, `0.01`, true},
		{"history overflow", func(g domain.Goal) domain.Goal {
			g.ContributionHistory = []domain.GoalMonthTotal{{Month: "2026-07", Amount: money.New(math.MaxInt64, "USD"), Count: 1}}
			for i := 0; i < domain.MaxGoalContributions; i++ {
				g.Contributions = append(g.Contributions, domain.GoalContribution{Amount: money.New(1, "USD"), At: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)})
			}
			return g
		}, `0.01`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, g, tool := assistantGoalWriteFixture(t, "USD")
			g = tc.configure(g)
			if err := app.PutGoal(g); err != nil {
				t.Fatal(err)
			}
			before, revision := assistantStoredGoal(t, app, g.ID), app.Rev()
			raw := json.RawMessage(`{"goal_id":"write-goal","amount":` + tc.amount + `}`)
			preview := tool.preview(raw)
			if strings.Contains(preview, "Can't record") != tc.previewReject {
				t.Fatalf("unexpected preview: %s", preview)
			}
			out := tool.run(raw)
			if strings.Contains(out, "Recorded a") || app.Rev() != revision || !reflect.DeepEqual(assistantStoredGoal(t, app, g.ID), before) || len(app.Transactions()) != 0 {
				t.Fatalf("failed contribution partially changed state: %s", out)
			}
		})
	}
	app, g, tool := assistantGoalWriteFixture(t, "USD")
	raw := json.RawMessage(`{"goal_id":"write-goal","amount":1}`)
	_ = tool.preview(raw)
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
	revision := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "read-only") || app.Rev() != revision || assistantStoredGoal(t, app, g.ID).CurrentAmount.Amount != 0 {
		t.Fatalf("role guard bypassed: %s", out)
	}
}
