// SPDX-License-Identifier: MIT

package domain

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestLiabilitySignConvention(t *testing.T) {
	for _, tt := range []struct {
		name           string
		class          AccountClass
		opening        int64
		explicit, want LiabilitySign
		inflow         int64
	}{
		{"zero defaults positive owed", ClassLiability, 0, "", LiabilityPositiveOwed, -1},
		{"positive opening", ClassLiability, 100, "", LiabilityPositiveOwed, -1},
		{"negative opening", ClassLiability, -100, "", LiabilityNegativeOwed, 1},
		{"positive owed with credit balance", ClassLiability, -100, LiabilityPositiveOwed, LiabilityPositiveOwed, -1},
		{"negative owed with credit balance", ClassLiability, 100, LiabilityNegativeOwed, LiabilityNegativeOwed, 1},
		{"asset overdraft", ClassAsset, -100, "", "", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := Account{Class: tt.class, OpeningBalance: money.New(tt.opening, "USD"), LiabilitySign: tt.explicit}
			got := account.WithLiabilitySign()
			if got.LiabilitySign != tt.want || got.InflowSign() != tt.inflow {
				t.Fatalf("convention = %q, inflow = %d; want %q, %d", got.LiabilitySign, got.InflowSign(), tt.want, tt.inflow)
			}
			if got.WithLiabilitySign().LiabilitySign != got.LiabilitySign || account.LiabilitySign != tt.explicit {
				t.Fatal("normalization changed its input or was not idempotent")
			}
		})
	}
}
