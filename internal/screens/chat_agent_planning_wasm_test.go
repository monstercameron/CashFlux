// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func assistantPlanningTool(t *testing.T, tools []chatTool, name string) chatTool {
	t.Helper()
	for _, tool := range tools {
		if tool.spec.Function.Name == name {
			if tool.mutates || tool.preview != nil {
				t.Fatalf("read tool %s unexpectedly mutates", name)
			}
			return tool
		}
	}
	t.Fatalf("missing planning tool %s", name)
	return chatTool{}
}

func assistantPlanningRead[T any](t *testing.T, tool chatTool, raw string) T {
	t.Helper()
	var envelope struct {
		ReadOnly bool            `json:"read_only"`
		Error    string          `json:"error"`
		Revision string          `json:"revision"`
		Data     json.RawMessage `json:"data"`
	}
	out := tool.run(json.RawMessage(raw))
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("invalid tool JSON: %s (%v)", out, err)
	}
	if envelope.Error != "" || !envelope.ReadOnly || envelope.Revision == "" {
		t.Fatalf("tool failed: %s", out)
	}
	var result T
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		t.Fatalf("invalid result: %s (%v)", out, err)
	}
	return result
}

func assistantPlanningAccount() domain.Account {
	return domain.Account{ID: "plan-bank", Name: "Planning bank", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")}
}

func TestAssistantPlanningToolsRegisteredOnceAndReadOnly(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	counts := map[string]int{}
	for _, tool := range buildChatTools(app, "USD", currency.Rates{Base: "USD"}, aicontext.TierFull) {
		counts[tool.spec.Function.Name]++
	}
	for _, name := range []string{"get_account_details", "project_account", "list_goals", "goal_funding_plan", "list_goal_reservations", "list_recurring"} {
		if counts[name] != 1 {
			t.Errorf("%s registration count=%d, want 1", name, counts[name])
		}
	}
	for _, tool := range agToolsPlanning(app, "USD", currency.Rates{Base: "USD"}) {
		if tool.mutates || tool.preview != nil {
			t.Fatalf("planning read %s requires mutation approval", tool.spec.Function.Name)
		}
	}
}

func TestAssistantPlanningAccountToolsReadFreshStateAndExactMoney(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	tools := agToolsPlanning(app, "EUR", currency.Rates{Base: "EUR"}) // deliberately wrong captured values
	tool := assistantPlanningTool(t, tools, "get_account_details")
	if out := tool.run(json.RawMessage(`{"account_id":"plan-bank"}`)); !strings.Contains(out, "no account") {
		t.Fatalf("missing account became zero: %s", out)
	}
	a := assistantPlanningAccount()
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	f := assistantPlanningRead[assistantdata.AccountFact](t, tool, `{"account_id":"plan-bank"}`)
	if f.Balance.Amount.Minor != "10000" || f.BaseBalance.Amount.Currency != "USD" {
		t.Fatalf("new account or live base missing: %+v", f)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "plan-income", AccountID: a.ID, Desc: "Income", Date: time.Now(), Amount: money.New(2000, "USD")}); err != nil {
		t.Fatal(err)
	}
	f = assistantPlanningRead[assistantdata.AccountFact](t, tool, `{"account_id":"plan-bank"}`)
	if f.Balance.Amount.Minor != "12000" || f.Reservation.Free.Amount.Minor != "12000" {
		t.Fatalf("account handler retained pre-write snapshot: %+v", f)
	}
	a.OpeningBalance = money.New(math.MaxInt64, "USD")
	a.ID = "extreme"
	a.Name = "Extreme"
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	f = assistantPlanningRead[assistantdata.AccountFact](t, tool, `{"account_id":"extreme"}`)
	if f.Balance.Amount.Minor != "9223372036854775807" {
		t.Fatalf("extreme integer lost: %+v", f.Balance)
	}
	revision := app.Rev()
	_ = assistantPlanningRead[assistantdata.AccountFact](t, tool, `{"account_id":"plan-bank"}`)
	if app.Rev() != revision {
		t.Fatal("read changed dataset revision")
	}
}

func TestAssistantPlanningGoalsRefreshRatesTasksAndRetainedClaims(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	tools := agToolsPlanning(app, "USD", currency.Rates{Base: "USD"})
	a := assistantPlanningAccount()
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	g := domain.Goal{ID: "plan-goal", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, TargetAmount: money.New(10000, "EUR"), CurrentAmount: money.Zero("EUR"), TargetDate: now.AddDate(0, 4, 0), Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(2000, "USD")}}}
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	list := assistantPlanningTool(t, tools, "list_goals")
	unknown := assistantPlanningRead[assistantPlanningPage[assistantdata.GoalFact]](t, list, `{"goal_id":"plan-goal"}`)
	if unknown.Items[0].Known || unknown.Items[0].Covered.Amount != nil || unknown.Items[0].Error == "" {
		t.Fatalf("missing FX hidden: %+v", unknown)
	}
	s := app.Settings()
	s.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	known := assistantPlanningRead[assistantPlanningPage[assistantdata.GoalFact]](t, list, `{"goal_id":"plan-goal"}`)
	if !known.Items[0].Known || known.Items[0].Covered.Amount.Minor != "1000" || known.Items[0].Covered.Amount.Currency != "EUR" {
		t.Fatalf("live FX ignored: %+v", known)
	}
	g.Archived = true
	if err := app.PutGoal(g); err != nil {
		t.Fatal(err)
	}
	active := assistantPlanningRead[assistantPlanningPage[assistantdata.GoalFact]](t, list, `{}`)
	if active.Total != 0 {
		t.Fatalf("archived goal remained active: %+v", active)
	}
	reservation := assistantPlanningRead[assistantPlanningPage[assistantdata.ReservationFact]](t, assistantPlanningTool(t, tools, "list_goal_reservations"), `{"account_id":"plan-bank"}`)
	if reservation.Items[0].Reserved.Amount.Minor != "2000" || len(reservation.Items[0].Goals) != 1 {
		t.Fatalf("archive erased retained reservations: %+v", reservation)
	}
	check := domain.Goal{ID: "check", Name: "Checklist", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Kind: domain.GoalKindChecklist}
	if err := app.PutGoal(check); err != nil {
		t.Fatal(err)
	}
	task := domain.Task{ID: "check-task", Title: "Task", Status: domain.StatusOpen, Priority: domain.PriorityMedium, RelatedType: domain.RelatedGoal, RelatedID: check.ID}
	if err := app.PutTask(task); err != nil {
		t.Fatal(err)
	}
	before := assistantPlanningRead[assistantPlanningPage[assistantdata.GoalFact]](t, list, `{"goal_id":"check"}`)
	if before.Items[0].Complete || before.Items[0].TasksTotal != 1 {
		t.Fatalf("task facts unavailable: %+v", before)
	}
	task.Status = domain.StatusDone
	if err := app.PutTask(task); err != nil {
		t.Fatal(err)
	}
	after := assistantPlanningRead[assistantPlanningPage[assistantdata.GoalFact]](t, list, `{"goal_id":"check"}`)
	if !after.Items[0].Complete || after.Items[0].TasksDone != 1 {
		t.Fatalf("task update stale: %+v", after)
	}
}

func TestAssistantPlanningProjectionRefreshesDriversAndReportsOverflow(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	tools := agToolsPlanning(app, "USD", currency.Rates{Base: "USD"})
	tool := assistantPlanningTool(t, tools, "project_account")
	a := assistantPlanningAccount()
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	r := domain.Recurring{ID: "plan-rent", Label: "Rent", AccountID: a.ID, Amount: money.New(-500, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	raw := `{"account_id":"plan-bank","as_of":"2026-10-20T12:00:00Z","horizon_days":30}`
	f := assistantPlanningRead[assistantdata.ProjectionFact](t, tool, raw)
	if !f.Known || len(f.Drivers) != 1 || f.End.Amount.Minor != "9500" {
		t.Fatalf("wrong canonical projection: %+v", f)
	}
	r.Paused = true
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	f = assistantPlanningRead[assistantdata.ProjectionFact](t, tool, raw)
	if !f.Known || len(f.Drivers) != 0 || f.End.Amount.Minor != "10000" {
		t.Fatalf("paused driver retained: %+v", f)
	}
	r.Paused = false
	r.Amount = money.New(1, "USD")
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	a.OpeningBalance = money.New(math.MaxInt64, "USD")
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	f = assistantPlanningRead[assistantdata.ProjectionFact](t, tool, raw)
	if f.Known || f.End.Amount != nil || !strings.Contains(f.Error, "overflow") {
		t.Fatalf("overflow reported as zero: %+v", f)
	}
	for _, raw := range []string{`{"account_id":"plan-bank","horizon_days":367}`, `{"account_id":"plan-bank","as_of":"not a date"}`} {
		if out := tool.run(json.RawMessage(raw)); !strings.Contains(out, `"error"`) {
			t.Fatalf("invalid parameters accepted: %s", out)
		}
	}
}

func TestAssistantPlanningRecurringReadsPaidFlagsAndPaginates(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	tools := agToolsPlanning(app, "USD", currency.Rates{Base: "USD"})
	tool := assistantPlanningTool(t, tools, "list_recurring")
	a := assistantPlanningAccount()
	if err := app.PutAccount(a); err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	r := domain.Recurring{ID: "daily", Label: "Daily", AccountID: a.ID, Amount: money.New(-1, "USD"), Cadence: domain.CadenceDaily, NextDue: due}
	if err := app.PutRecurring(r); err != nil {
		t.Fatal(err)
	}
	paused := r
	paused.ID = "paused"
	paused.Label = "Paused"
	paused.Paused = true
	if err := app.PutRecurring(paused); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Schedules assistantdata.RecurringResult `json:"schedules"`
		Next      *int                          `json:"definitions_next_offset"`
	}
	raw := `{"from":"2026-10-20","before":"2026-10-23","limit":1,"include_paused":true}`
	result = assistantPlanningRead[struct {
		Schedules assistantdata.RecurringResult `json:"schedules"`
		Next      *int                          `json:"definitions_next_offset"`
	}](t, tool, raw)
	if result.Schedules.Total != 3 || len(result.Schedules.Occurrences) != 1 || result.Schedules.NextOffset == nil || result.Schedules.DefinitionCount != 2 || result.Next == nil || result.Schedules.Occurrences[0].Paid {
		t.Fatalf("page metadata mismatch: %+v", result)
	}
	if err := app.MarkOccurrencePaid("recurring:daily", due); err != nil {
		t.Fatal(err)
	}
	result = assistantPlanningRead[struct {
		Schedules assistantdata.RecurringResult `json:"schedules"`
		Next      *int                          `json:"definitions_next_offset"`
	}](t, tool, raw)
	if !result.Schedules.Occurrences[0].Paid {
		t.Fatal("manual paid status stale")
	}
	if err := app.UnmarkOccurrencePaid("recurring:daily", due); err != nil {
		t.Fatal(err)
	}
	result = assistantPlanningRead[struct {
		Schedules assistantdata.RecurringResult `json:"schedules"`
		Next      *int                          `json:"definitions_next_offset"`
	}](t, tool, `{"from":"2026-10-20","before":"2026-10-23","limit":1,"offset":2,"definitions_offset":1,"include_paused":true}`)
	if result.Schedules.Occurrences[0].Paid || result.Schedules.NextOffset != nil || len(result.Schedules.Definitions) != 1 || !result.Schedules.Definitions[0].Paused || result.Next != nil {
		t.Fatalf("paid flags/last page stale: %+v", result)
	}
}

func TestAssistantPlanningReadFailureIsExplicit(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	tools := agToolsPlanning(app, "USD", currency.Rates{Base: "USD"})
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	out := assistantPlanningTool(t, tools, "list_goals").run(json.RawMessage(`{}`))
	if !strings.Contains(out, `"error"`) || strings.Contains(out, `"data"`) {
		t.Fatalf("closed store became an empty result: %s", out)
	}
}
