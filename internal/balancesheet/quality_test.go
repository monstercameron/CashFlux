// SPDX-License-Identifier: MIT

package balancesheet

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

var qNow = time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

func qAcct(id string, class domain.AccountClass, t domain.AccountType, openingMinor int64, asOfDaysAgo int) domain.Account {
	a := acct(id, class, t, openingMinor)
	if asOfDaysAgo >= 0 {
		a.BalanceAsOf = qNow.AddDate(0, 0, -asOfDaysAgo)
	}
	return a
}

func TestAssessQuality(t *testing.T) {
	condo := qAcct("condo", domain.ClassAsset, domain.TypeProperty, 30400000, 400)
	condo.Name = "Condo"
	checking := qAcct("chk", domain.ClassAsset, domain.TypeChecking, 4900000, 2)
	checking.Name = "Checking"
	euro := qAcct("eur", domain.ClassAsset, domain.TypeSavings, 100000, 5)
	euro.Name = "Euro savings"
	euro.Currency = "EUR"
	mortgage := qAcct("mort", domain.ClassLiability, domain.TypeMortgage, -25000000, 10)
	mortgage.Name = "Mortgage"
	gone := qAcct("gone", domain.ClassAsset, domain.TypeChecking, 999999, 1)
	gone.Archived = true

	accounts := []domain.Account{condo, checking, euro, mortgage, gone}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}

	q := AssessQuality(QualityInput{
		Accounts: accounts, Rates: rates, Now: qNow,
		IsStale:          func(a domain.Account) bool { return a.ID == "condo" },
		IsManual:         func(a domain.Account) bool { return a.ID == "condo" || a.ID == "euro" },
		ExcludedByChoice: 2, ExcludedNoRate: 1,
	})

	if q.AccountsIncluded != 4 {
		t.Errorf("AccountsIncluded = %d, want 4 (the archived account is not in the figure)", q.AccountsIncluded)
	}
	if len(q.Stale) != 1 || q.Stale[0].ID != "condo" {
		t.Fatalf("Stale = %+v, want just the condo", q.Stale)
	}
	if q.Stale[0].DaysSince != 400 {
		t.Errorf("DaysSince = %d, want 400", q.Stale[0].DaysSince)
	}
	// The condo is both the oldest hand-entered valuation AND most of the asset
	// side — which together is exactly the risk worth disclosing.
	if !q.HasOldestManual || q.OldestManual.ID != "condo" {
		t.Fatalf("OldestManual = %+v, want the condo", q.OldestManual)
	}
	if !q.HasDominant || q.Dominant.ID != "condo" {
		t.Fatalf("Dominant = %+v, want the condo", q.Dominant)
	}
	if q.Dominant.ShareOfSideBips < 8000 {
		t.Errorf("Dominant share = %d bips, want the condo to read as most of the asset side", q.Dominant.ShareOfSideBips)
	}
	if q.OldestManual.Source != SourceManual {
		t.Errorf("OldestManual.Source = %s, want manual", q.OldestManual.Source)
	}
	if len(q.Converted) != 1 || q.Converted[0] != "EUR" {
		t.Errorf("Converted = %v, want [EUR]", q.Converted)
	}
	if q.BaseCurrency != "USD" {
		t.Errorf("BaseCurrency = %s, want USD", q.BaseCurrency)
	}
	if q.ExcludedByChoice != 2 || q.ExcludedNoRate != 1 {
		t.Errorf("exclusions = %d/%d, want 2/1", q.ExcludedByChoice, q.ExcludedNoRate)
	}
	if !q.NeedsAttention() {
		t.Error("NeedsAttention = false, want true when something is stale or excluded")
	}
}

func TestAssessQualityStaysQuietWhenEverythingIsFine(t *testing.T) {
	a := qAcct("chk", domain.ClassAsset, domain.TypeChecking, 100000, 1)
	q := AssessQuality(QualityInput{
		Accounts: []domain.Account{a}, Rates: currency.Rates{Base: "USD"}, Now: qNow,
	})
	if q.NeedsAttention() {
		t.Error("NeedsAttention = true on a clean sheet — a clean bill of health must not become permanent furniture")
	}
	if len(q.Converted) != 0 {
		t.Errorf("Converted = %v, want none when everything is already in base", q.Converted)
	}
	if q.HasDominant {
		t.Error("a single account should not be reported as a dominance RISK when it is the only one")
	}
}

func TestAssessQualityRanksNeverConfirmedAsMostOverdue(t *testing.T) {
	never := qAcct("never", domain.ClassAsset, domain.TypeProperty, 5000000, -1)
	never.Name = "Never confirmed"
	old := qAcct("old", domain.ClassAsset, domain.TypeProperty, 5000000, 200)
	old.Name = "Old"
	q := AssessQuality(QualityInput{
		Accounts: []domain.Account{old, never}, Rates: currency.Rates{Base: "USD"}, Now: qNow,
		IsStale:  func(domain.Account) bool { return true },
		IsManual: func(domain.Account) bool { return true },
	})
	if len(q.Stale) != 2 || q.Stale[0].ID != "never" {
		t.Fatalf("Stale = %+v, want the never-confirmed account first", q.Stale)
	}
	if q.Stale[0].DaysSince != -1 {
		t.Errorf("DaysSince = %d, want -1 for never confirmed", q.Stale[0].DaysSince)
	}
	if q.OldestManual.ID != "never" {
		t.Errorf("OldestManual = %s, want the never-confirmed one — unknown age is the weakest case", q.OldestManual.ID)
	}
}

// A dominant holding must be reported even when it is perfectly fresh: the
// point is that the figure DEPENDS on it, which is true regardless.
func TestAssessQualityReportsDominanceIndependentOfStaleness(t *testing.T) {
	big := qAcct("big", domain.ClassAsset, domain.TypeProperty, 30000000, 1)
	big.Name = "House"
	small := qAcct("small", domain.ClassAsset, domain.TypeChecking, 100000, 1)
	q := AssessQuality(QualityInput{
		Accounts: []domain.Account{big, small}, Rates: currency.Rates{Base: "USD"}, Now: qNow,
	})
	if !q.HasDominant || q.Dominant.ID != "big" {
		t.Fatalf("Dominant = %+v, want the house", q.Dominant)
	}
	if q.NeedsAttention() {
		t.Error("dominance alone is context, not a problem to nag about")
	}
}
