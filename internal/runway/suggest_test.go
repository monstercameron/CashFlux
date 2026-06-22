package runway

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// liquidAcct builds a non-archived checking account with the given opening balance.
func liquidAcct(id, name string, openingMinor int64, typ domain.AccountType) domain.Account {
	return domain.Account{
		ID: id, Name: name,
		Class:          domain.ClassAsset,
		Type:           typ,
		Currency:       "USD",
		OpeningBalance: money.New(openingMinor, "USD"),
	}
}

func usdRates() currency.Rates { return currency.Rates{Base: "USD"} }

func TestSuggestCoverPicksLargestLiquid(t *testing.T) {
	accounts := []domain.Account{
		liquidAcct("chk1", "Main Checking", 50000, domain.TypeChecking),      // $500
		liquidAcct("sav1", "High-Yield Savings", 200000, domain.TypeSavings), // $2000 ← largest
		liquidAcct("inv1", "Brokerage", 999999, domain.TypeInvestment),       // not liquid
	}
	s := SuggestCover(30000, accounts, nil, usdRates())
	if !s.Found {
		t.Fatal("expected a suggestion, got none")
	}
	if s.SourceName != "High-Yield Savings" {
		t.Errorf("SourceName = %q, want %q", s.SourceName, "High-Yield Savings")
	}
	if s.SourceID != "sav1" {
		t.Errorf("SourceID = %q, want sav1", s.SourceID)
	}
	// shortfall (300) < source balance (2000) → move = shortfall
	if s.AmountMinor != 30000 {
		t.Errorf("AmountMinor = %d, want 30000", s.AmountMinor)
	}
}

func TestSuggestCoverAmountCappedAtSourceBalance(t *testing.T) {
	// shortfall $500, but only $200 available
	accounts := []domain.Account{
		liquidAcct("chk", "Checking", 20000, domain.TypeChecking),
	}
	s := SuggestCover(50000, accounts, nil, usdRates())
	if !s.Found {
		t.Fatal("expected a suggestion")
	}
	if s.AmountMinor != 20000 {
		t.Errorf("AmountMinor = %d, want 20000 (capped at source balance)", s.AmountMinor)
	}
}

func TestSuggestCoverNoLiquidAccounts(t *testing.T) {
	accounts := []domain.Account{
		{ID: "inv", Name: "Brokerage", Class: domain.ClassAsset, Type: domain.TypeInvestment, Currency: "USD",
			OpeningBalance: money.New(999999, "USD")},
	}
	s := SuggestCover(10000, accounts, nil, usdRates())
	if s.Found {
		t.Error("expected no suggestion when all accounts are non-liquid")
	}
}

func TestSuggestCoverArchivedAccountExcluded(t *testing.T) {
	archived := liquidAcct("old", "Old Checking", 999999, domain.TypeChecking)
	archived.Archived = true
	accounts := []domain.Account{archived}
	s := SuggestCover(10000, accounts, nil, usdRates())
	if s.Found {
		t.Error("expected no suggestion when only account is archived")
	}
}

func TestSuggestCoverAllZeroBalance(t *testing.T) {
	accounts := []domain.Account{
		liquidAcct("chk", "Checking", 0, domain.TypeChecking),
	}
	s := SuggestCover(10000, accounts, nil, usdRates())
	if s.Found {
		t.Error("expected no suggestion when all liquid accounts are empty")
	}
}

func TestSuggestCoverZeroShortfall(t *testing.T) {
	accounts := []domain.Account{
		liquidAcct("chk", "Checking", 100000, domain.TypeChecking),
	}
	s := SuggestCover(0, accounts, nil, usdRates())
	if s.Found {
		t.Error("zero shortfall should return no suggestion")
	}
}

func TestSuggestCoverLiabilityExcluded(t *testing.T) {
	// A credit card with a positive balance (available credit) must not be
	// suggested as a cover source.
	cc := domain.Account{
		ID: "cc", Name: "Credit Card", Class: domain.ClassLiability,
		Type: domain.TypeCreditCard, Currency: "USD",
		OpeningBalance: money.New(100000, "USD"),
	}
	accounts := []domain.Account{cc}
	s := SuggestCover(10000, accounts, nil, usdRates())
	if s.Found {
		t.Error("liability accounts must never be suggested as cover sources")
	}
}

func TestSuggestCoverConsidersTransactions(t *testing.T) {
	// Opening $1000, spent $900 → only $100 left; shortfall is $200 → move capped at $100.
	accounts := []domain.Account{
		liquidAcct("chk", "Checking", 100000, domain.TypeChecking),
	}
	txns := []domain.Transaction{
		{AccountID: "chk", Amount: money.New(-90000, "USD")},
	}
	s := SuggestCover(20000, accounts, txns, usdRates())
	if !s.Found {
		t.Fatal("expected a suggestion when some balance remains")
	}
	if s.AmountMinor != 10000 {
		t.Errorf("AmountMinor = %d, want 10000 (net balance after txns, capped)", s.AmountMinor)
	}
}

func TestSuggestCoverAllTypesOfLiquidAccounts(t *testing.T) {
	// All four liquid types should be candidates.
	accounts := []domain.Account{
		liquidAcct("chk", "Checking", 10000, domain.TypeChecking),
		liquidAcct("dbt", "Debit", 20000, domain.TypeDebit),
		liquidAcct("sav", "Savings", 30000, domain.TypeSavings),
		liquidAcct("csh", "Cash", 40000, domain.TypeCash), // ← largest
	}
	s := SuggestCover(5000, accounts, nil, usdRates())
	if !s.Found {
		t.Fatal("expected a suggestion")
	}
	if s.SourceName != "Cash" {
		t.Errorf("SourceName = %q, want Cash (largest liquid)", s.SourceName)
	}
}
