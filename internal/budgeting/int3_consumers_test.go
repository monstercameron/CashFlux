// SPDX-License-Identifier: MIT

package budgeting_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/coverformula"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/tasksuggest"
	"github.com/monstercameron/CashFlux/internal/widgetsource"
)

func TestINT3ConsumersKeepExplicitRefundScope(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	now := start.AddDate(0, 0, 27)
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{{ID: "buy", CategoryID: "food", Date: start, Amount: money.New(-10000, "USD")}, {ID: "refund", CategoryID: "food", Date: end, Amount: money.New(6000, "USD")}}
	links := []domain.TxnLink{{ID: "pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(6000, "USD")}}
	b := domain.Budget{ID: "food", Name: "Food", CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(5000, "USD")}
	cats := []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense, CategoryClass: domain.ClassFlex}}
	for _, linked := range []bool{true, false} {
		t.Run(map[bool]string{true: "linked", false: "separate_scope"}[linked], func(t *testing.T) {
			var scopeLinks []domain.TxnLink
			want := int64(10000)
			if linked {
				scopeLinks = links
				want = 4000
			}
			ledger := budgeting.NewNettedLedger(txns, scopeLinks)
			vars := engineenv.Vars(engineenv.Data{Transactions: txns, Ledger: ledger, Budgets: []domain.Budget{b}, Categories: cats, Rates: rates, Now: now})
			if got := vars["budget_food_spent"]; got != float64(want)/100 {
				t.Fatalf("engine spent=%v want %v", got, float64(want)/100)
			}
			amount, err := (coverformula.Context{Txns: ledger, Rates: rates, Now: now}).AmountMinor("spent", b)
			if err != nil || amount != want {
				t.Fatalf("cover spent=%d err=%v", amount, err)
			}
			flex, err := budgeting.EvaluateFlex(cats, ledger, nil, 5000, rates, start, end)
			if err != nil || flex.Spent.Amount != want {
				t.Fatalf("flex=%+v err=%v", flex, err)
			}
			frame := widgetsource.BudgetStatus([]domain.Budget{b}, cats, ledger, rates, start, end, false, 0)
			if frame.Rows != 1 {
				t.Fatalf("widget rows=%d", frame.Rows)
			}
			expectedPercent := float64(want) * 100 / 5000
			field, _ := frame.Column("percent")
			if field.Values[0] != expectedPercent {
				t.Fatalf("widget percent=%v want %v", field.Values[0], expectedPercent)
			}
			suggestions := tasksuggest.Scan(nil, ledger, []domain.Budget{b}, freshness.Windows{}, rates, now, time.Sunday)
			over := false
			for _, s := range suggestions {
				if s.Kind == tasksuggest.KindOverspentBudget {
					over = true
				}
			}
			if over == linked {
				t.Fatalf("overspent task=%v linked=%v", over, linked)
			}
			insights := smartengine.RunPage(smartengine.Input{Now: now, Base: "USD", Rates: rates, Transactions: txns, Ledger: ledger, Categories: cats, Budgets: []domain.Budget{b}}, smart.EnableFreeOnly(smart.Settings{}), smart.PageBudgets)
			pacing := false
			for _, s := range insights {
				if s.Feature == "SMART-B9" {
					pacing = true
				}
			}
			if pacing == linked {
				t.Fatalf("pacing warning=%v linked=%v", pacing, linked)
			}
		})
	}
	if txns[0].Amount.Amount != -10000 {
		t.Fatal("netting mutated raw history")
	}
}
