// SPDX-License-Identifier: MIT

package assistantview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/insightsperiod"
	"github.com/monstercameron/CashFlux/internal/scope"
)

func TestBriefingScopeAndPeriodShareOneSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	visible := questionTxn("visible", "", "USD", -100, now)
	other := questionTxn("other", "", "USD", -900, now)
	other.AccountID = "other"
	prior := questionTxn("prior", "", "USD", -70, now.AddDate(0, -1, 0))
	in := BriefingInput{
		Accounts: []domain.Account{{ID: "checking"}, {ID: "other"}}, Transactions: []domain.Transaction{visible, other, prior},
		Scope: scope.ReportScope{AccountIDs: []string{"checking"}}, Period: insightsperiod.ThisMonth, Rates: currency.Rates{Base: "USD"}, Now: now,
	}
	got := DeriveBriefing(in)
	if got.Spent != 100 || got.Prior != 70 || len(got.Transactions) != 2 || len(got.MonthMerchants) != 1 || got.MonthMerchants[0].Amount != 100 || got.TrendError != nil {
		t.Fatalf("incorrect scoped briefing: %+v", got)
	}
	in.Period = insightsperiod.LastMonth
	last := DeriveBriefing(in)
	if last.Spent != 70 || last.Prior != 0 || last.Window.Period != insightsperiod.LastMonth {
		t.Fatalf("period selection was ignored: %+v", last)
	}
	if other.AccountID != "other" || len(in.Transactions) != 3 {
		t.Fatal("input snapshot mutated")
	}
}
