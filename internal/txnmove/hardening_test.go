// SPDX-License-Identifier: MIT

package txnmove

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPlanMoveChecksSignFlipOverflowAndDestReconciliation(t *testing.T) {
	accounts := []domain.Account{
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Currency: "USD"},
		{
			ID: "cc", Name: "Card", Class: domain.ClassLiability,
			LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD",
			OpeningBalance:  money.New(1000, "USD"),
			Reconciliations: []domain.Reconciliation{{StatementDate: day.AddDate(0, 0, 5)}},
		},
	}

	// PlanMove must reject MinInt64 sign-flip overflow up front (matching Apply).
	minRow := tx("m", "chk", math.MinInt64)
	if p := PlanMove(minRow, "cc", accounts, nil); p.Err == nil {
		t.Fatal("PlanMove allowed MinInt64 sign-flip that Apply rejects")
	}

	// Moving a cleared row onto a reconciled destination must flag BreaksReconciliation.
	cleared := tx("c", "chk", -2500)
	cleared.Cleared = true
	if p := PlanMove(cleared, "cc", accounts, nil); p.Err != nil || !p.BreaksReconciliation {
		t.Fatalf("destination reconciliation not flagged: %+v", p)
	}
}
