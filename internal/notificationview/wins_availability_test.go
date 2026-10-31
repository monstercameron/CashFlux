// SPDX-License-Identifier: MIT

package notificationview

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/milestones"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/networthview"
)

func TestPrepareWinsRequiresCompleteNetWorthWithoutSuppressingOtherWins(t *testing.T) {
	bank := domain.Account{ID: "bank", Currency: "USD", OpeningBalance: money.New(2_000_000, "USD")}
	foreign := domain.Account{ID: "foreign", Currency: "EUR", Class: domain.ClassLiability, OpeningBalance: money.New(-100, "EUR")}
	broken := domain.Account{ID: "broken", Currency: "USD", OpeningBalance: money.New(100, "EUR")}
	minDebt := domain.Account{ID: "min", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(math.MinInt64, "USD")}
	for _, tc := range []struct {
		name         string
		accounts     []domain.Account
		rates        currency.Rates
		wantNetWorth bool
	}{
		{"complete", []domain.Account{bank}, currency.Rates{Base: "USD"}, true},
		{"missing debt rate", []domain.Account{bank, foreign}, currency.Rates{Base: "USD"}, false},
		{"missing asset rate", []domain.Account{bank, {ID: "asset", Currency: "EUR", OpeningBalance: money.New(100, "EUR")}}, currency.Rates{Base: "USD"}, false},
		{"bad account balance", []domain.Account{bank, broken}, currency.Rates{Base: "USD"}, false},
		{"unrepresentable liability", []domain.Account{bank, minDebt}, currency.Rates{Base: "USD"}, false},
		{"restored rate", []domain.Account{bank, foreign}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := WinsInput{Accounts: tc.accounts, Rates: tc.rates, Now: time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC), NoSpendLimit: 7,
				Goals: []domain.Goal{{ID: "done", Name: "Kept goal", Kind: domain.GoalKindMilestone, DoneAt: time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC)}}}
			for _, precomputed := range []bool{false, true} {
				if precomputed {
					view := networthview.Build(in.Accounts, nil, in.Rates)
					in.NetWorth = &view
				}
				found := milestones.Detect(PrepareWins(in))
				kinds := map[milestones.Kind]bool{}
				for _, win := range found {
					kinds[win.Kind] = true
				}
				if kinds[milestones.KindNetWorth] != tc.wantNetWorth || !kinds[milestones.KindGoalReached] || !kinds[milestones.KindNoSpendStreak] {
					t.Fatalf("precomputed=%v availability incorrectly changed wins: %+v", precomputed, found)
				}
			}
		})
	}
}
