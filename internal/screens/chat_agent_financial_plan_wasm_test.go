// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/finplan"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func assistantFinancialPlanFixture(t *testing.T, app *appstate.App, code string) {
	t.Helper()
	now := app.ClockNow().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	ds := store.Dataset{Settings: store.Settings{BaseCurrency: code}, Accounts: []domain.Account{{ID: "cash", Name: "Recorded cash", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: code, OpeningBalance: money.New(300000, code)}}}
	for i := 1; i <= 3; i++ {
		date := month.AddDate(0, -i, 10)
		ds.Transactions = append(ds.Transactions, domain.Transaction{ID: date.Format("2006-01") + "-in", AccountID: "cash", Date: date, Amount: money.New(100000, code)}, domain.Transaction{ID: date.Format("2006-01") + "-out", AccountID: "cash", Date: date, Amount: money.New(-50000, code)})
	}
	if err := app.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
}

func assistantFinancialPlanResult(t *testing.T, tool chatTool) finplan.Assessment {
	t.Helper()
	out := tool.run(json.RawMessage(`{}`))
	var response struct {
		Assessment finplan.Assessment `json:"assessment"`
		Notes      string             `json:"notes"`
	}
	if err := json.Unmarshal([]byte(out), &response); err != nil || response.Assessment.Framework == "" || !strings.Contains(response.Notes, "USD 1,000") || !strings.Contains(response.Notes, "proxy") {
		t.Fatal("missing assessment/evidence contract", out, err)
	}
	return response.Assessment
}

func TestAssistantFinancialPlanReadsBoundChoicesAndFreshNativeAssessment(t *testing.T) {
	first := primaryAccountsGoalsApp(t)
	assistantFinancialPlanFixture(t, first, "USD")
	for key, value := range map[string]string{"plan.framework": "foo", "plan.q.match": "yes", "plan.q.deductible": "no"} {
		if err := first.Store().SetKV(key, value); err != nil {
			t.Fatal(err)
		}
	}
	tool := assistantFinanceTestTool(t, agToolsFinancialPlan(first), "assess_financial_plan")
	if tool.mutates || tool.preview != nil {
		t.Fatal("assessment unexpectedly writes")
	}
	second := primaryAccountsGoalsApp(t)
	if err := second.Store().SetKV("plan.framework", "ramsey"); err != nil {
		t.Fatal(err)
	}
	rev := first.Rev()
	result := assistantFinancialPlanResult(t, tool)
	if result.Framework != "foo" || result.Choices.Deductible != "no" || result.Plan.Steps[0].Status != finplan.Todo || result.Liquid == nil || result.Liquid.Amount != 450000 || result.Inputs.SavingsRatePct != 50 || first.Rev() != rev {
		t.Fatal("global workspace or stale/incorrect native inputs", result)
	}
	if err := first.Store().SetKV("plan.q.deductible", "yes"); err != nil {
		t.Fatal(err)
	}
	result = assistantFinancialPlanResult(t, tool)
	if result.Choices.Deductible != "yes" || result.Plan.Steps[0].Status != finplan.Done {
		t.Fatal("persisted questionnaire update stayed stale", result)
	}
	query := assistantDataTool(t, first, "query_data")
	page := assistantFinancialPage(t, query, "financial_plan_config")
	if page.Total != 1 || !strings.Contains(string(page.Records[0].Data), `"deductibles":"yes"`) || !strings.Contains(string(page.Records[0].Data), `"frameworkStored":true`) {
		t.Fatal("current choices not discoverable", page)
	}
	if err := first.Store().SetKV("plan.q.match", "invalid"); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(json.RawMessage(`{}`)); !strings.Contains(out, `"error"`) || strings.Contains(out, `"assessment"`) {
		t.Fatal("invalid bound choices became an assessment", out)
	}
	if out := query.run(json.RawMessage(`{"collection":"financial_plan_config"}`)); !strings.Contains(out, `"error"`) || strings.Contains(out, `"total":0`) {
		t.Fatal("invalid choices became empty/default data", out)
	}
}

func TestAssistantFinancialPlanKeepsUnknownAPRAndFXNotAssessable(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	assistantFinancialPlanFixture(t, app, "EUR")
	if err := app.Store().SetKV("plan.framework", "ramsey"); err != nil {
		t.Fatal(err)
	}
	tool := assistantFinanceTestTool(t, agToolsFinancialPlan(app), "assess_financial_plan")
	result := assistantFinancialPlanResult(t, tool)
	if result.Plan.Steps[0].Status != finplan.NotAssessable || result.StarterUSD != nil || result.Unavailable["starter"] == "" || result.Plan.Steps[2].Status != finplan.Done {
		t.Fatal("missing USD rate became a starter-fund verdict or hid independent reserves", result)
	}
	if err := app.Store().SetKV("plan.framework", "foo"); err != nil {
		t.Fatal(err)
	}
	card := domain.Account{ID: "card", Name: "Unknown APR", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "EUR", OpeningBalance: money.Zero("EUR")}
	if err := app.Store().PutAccount(card); err != nil {
		t.Fatal(err)
	}
	result = assistantFinancialPlanResult(t, tool)
	if result.Plan.Steps[2].Status != finplan.NotAssessable || result.Unavailable["apr:card"] == "" {
		t.Fatal("unknown APR became paid-off/low-interest", result)
	}
	card.InterestRateAPR = domain.APR(9)
	if err := app.Store().PutAccount(card); err != nil {
		t.Fatal(err)
	}
	result = assistantFinancialPlanResult(t, tool)
	if result.Plan.Steps[2].Status != finplan.Todo {
		t.Fatal("fresh APR evidence not applied", result)
	}
	if err := app.Store().PutSettings(store.Settings{BaseCurrency: "USD"}); err != nil {
		t.Fatal(err)
	}
	result = assistantFinancialPlanResult(t, tool)
	if result.Inputs.HasIncome || result.Inputs.HasLiquidData || result.Plan.Steps[4].Status != finplan.NotAssessable || result.Unavailable["savings"] == "" {
		t.Fatal("failed health FX factor became a completed investing step", result)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(json.RawMessage(`{}`)); !strings.Contains(out, `"error"`) || strings.Contains(out, `"assessment"`) {
		t.Fatal("failed store became a no-data assessment", out)
	}
}

func TestAssistantFinancialPlanConfigDefaultsAndStrictArguments(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	query := assistantDataTool(t, app, "query_data")
	page := assistantFinancialPage(t, query, "financial_plan_config")
	if page.Total != 1 || !strings.Contains(string(page.Records[0].Data), `"effectiveFramework":"foo"`) || !strings.Contains(string(page.Records[0].Data), `"answeredMatch":false`) || !strings.Contains(string(page.Records[0].Data), `"frameworkStored":false`) {
		t.Fatal("absent answers invented a yes/no choice", page)
	}
	tool := assistantFinanceTestTool(t, agToolsFinancialPlan(app), "assess_financial_plan")
	result := assistantFinancialPlanResult(t, tool)
	if result.Plan.Steps[0].Status != finplan.NotAssessable || result.Plan.Steps[2].Status != finplan.NotAssessable || result.Plan.Steps[8].Status != finplan.NotAssessable {
		t.Fatal("empty data inferred completed or failed debt", result)
	}
	for _, raw := range []string{"null", "[]", "{bad", `{"framework":"ramsey"}`, "{} {}"} {
		if out := tool.run(json.RawMessage(raw)); !strings.Contains(out, `"error"`) || strings.Contains(out, `"assessment"`) {
			t.Fatal("invalid/override arguments accepted", raw, out)
		}
	}
}
