// SPDX-License-Identifier: MIT

package subscriptions

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// txnWithAccount builds a non-transfer expense with a specific AccountID.
func txnWithAccount(desc, accountID string, minor int64) domain.Transaction {
	return domain.Transaction{
		AccountID: accountID,
		Desc:      desc,
		Amount:    money.New(-minor, "USD"),
		Date:      time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	}
}

// txnWithPayee builds a non-transfer expense with Payee and Desc set.
func txnWithPayee(desc, payee, accountID string, minor int64) domain.Transaction {
	return domain.Transaction{
		AccountID: accountID,
		Desc:      desc,
		Payee:     payee,
		Amount:    money.New(-minor, "USD"),
		Date:      time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	}
}

// makeSub builds a minimal Subscription with the given display name.
func makeSub(name string) Subscription {
	return Subscription{
		Name:    name,
		Cadence: CadenceMonthly,
		Amount:  1000,
		Count:   3,
	}
}

func TestIsLiabilityPayment(t *testing.T) {
	creditCardAcct := domain.Account{
		ID:    "acct-cc-1",
		Name:  "Visa Signature",
		Type:  domain.TypeCreditCard,
		Class: domain.ClassLiability,
	}
	checkingAcct := domain.Account{
		ID:    "acct-chk-1",
		Name:  "Everyday Checking",
		Type:  domain.TypeChecking,
		Class: domain.ClassAsset,
	}
	loanAcct := domain.Account{
		ID:    "acct-loan-1",
		Name:  "Auto Loan",
		Type:  domain.TypeLoan,
		Class: domain.ClassLiability,
	}
	mortgageAcct := domain.Account{
		ID:    "acct-mtg-1",
		Name:  "Home Mortgage",
		Type:  domain.TypeMortgage,
		Class: domain.ClassLiability,
	}
	locAcct := domain.Account{
		ID:    "acct-loc-1",
		Name:  "HELOC",
		Type:  domain.TypeLineOfCredit,
		Class: domain.ClassLiability,
	}

	allAccounts := []domain.Account{creditCardAcct, checkingAcct, loanAcct, mortgageAcct, locAcct}

	tests := []struct {
		name string
		sub  Subscription
		txns []domain.Transaction
		want bool
	}{
		{
			name: "credit card account — account-type signal",
			sub:  makeSub("Chase Payment"),
			txns: []domain.Transaction{
				txnWithAccount("Chase Payment", creditCardAcct.ID, 25000),
				txnWithAccount("Chase Payment", creditCardAcct.ID, 25000),
				txnWithAccount("Chase Payment", creditCardAcct.ID, 25000),
			},
			want: true,
		},
		{
			name: "loan account — account-type signal",
			sub:  makeSub("AutoLoan"),
			txns: []domain.Transaction{
				txnWithAccount("AutoLoan", loanAcct.ID, 45000),
				txnWithAccount("AutoLoan", loanAcct.ID, 45000),
			},
			want: true,
		},
		{
			name: "mortgage account — account-type signal",
			sub:  makeSub("HomePay"),
			txns: []domain.Transaction{
				txnWithAccount("HomePay", mortgageAcct.ID, 200000),
				txnWithAccount("HomePay", mortgageAcct.ID, 200000),
			},
			want: true,
		},
		{
			name: "line-of-credit account — account-type signal",
			sub:  makeSub("HELOC Transfer"),
			txns: []domain.Transaction{
				txnWithAccount("HELOC Transfer", locAcct.ID, 50000),
				txnWithAccount("HELOC Transfer", locAcct.ID, 50000),
			},
			want: true,
		},
		{
			name: "label signal — 'loan payment' in description",
			sub:  makeSub("Loan Payment"),
			txns: []domain.Transaction{
				txnWithAccount("Loan Payment", checkingAcct.ID, 30000),
				txnWithAccount("Loan Payment", checkingAcct.ID, 30000),
			},
			want: true,
		},
		{
			name: "label signal — 'autopay' in payee",
			sub:  makeSub("Wells Fargo Autopay"),
			txns: []domain.Transaction{
				txnWithPayee("Wells Fargo Autopay", "WF Autopay", checkingAcct.ID, 10000),
				txnWithPayee("Wells Fargo Autopay", "WF Autopay", checkingAcct.ID, 10000),
			},
			want: true,
		},
		{
			name: "label signal — 'mortgage' in description",
			sub:  makeSub("Mortgage"),
			txns: []domain.Transaction{
				txnWithAccount("Mortgage", checkingAcct.ID, 180000),
				txnWithAccount("Mortgage", checkingAcct.ID, 180000),
			},
			want: true,
		},
		{
			name: "label signal — lender name 'chase' in subscription name",
			sub:  makeSub("Chase Card Payment"),
			txns: []domain.Transaction{
				txnWithAccount("Chase Card Payment", checkingAcct.ID, 50000),
			},
			want: true,
		},
		{
			name: "label signal — 'min payment' in payee",
			sub:  makeSub("BofA CC"),
			txns: []domain.Transaction{
				txnWithPayee("BofA CC", "BofA Min Payment", checkingAcct.ID, 2500),
				txnWithPayee("BofA CC", "BofA Min Payment", checkingAcct.ID, 2500),
			},
			want: true,
		},
		{
			name: "real subscription — Netflix from checking, no payment keywords",
			sub:  makeSub("Netflix"),
			txns: []domain.Transaction{
				txnWithAccount("Netflix", checkingAcct.ID, 1599),
				txnWithAccount("Netflix", checkingAcct.ID, 1599),
				txnWithAccount("Netflix", checkingAcct.ID, 1599),
			},
			want: false,
		},
		{
			name: "real subscription — Spotify from checking, no payment keywords",
			sub:  makeSub("Spotify"),
			txns: []domain.Transaction{
				txnWithAccount("Spotify", checkingAcct.ID, 1099),
				txnWithAccount("Spotify", checkingAcct.ID, 1099),
			},
			want: false,
		},
		{
			name: "unmatched transactions — no txn has matching Desc",
			sub:  makeSub("Adobe"),
			txns: []domain.Transaction{
				txnWithAccount("Netflix", checkingAcct.ID, 1599),
			},
			want: false,
		},
		{
			name: "empty transactions",
			sub:  makeSub("Hulu"),
			txns: nil,
			want: false,
		},
		{
			name: "empty accounts — account lookup misses, label also clean",
			sub:  makeSub("Hulu"),
			txns: []domain.Transaction{
				txnWithAccount("Hulu", "acct-unknown", 699),
				txnWithAccount("Hulu", "acct-unknown", 699),
			},
			want: false,
		},
		{
			name: "case-insensitive Desc match — mixed case",
			sub:  makeSub("NETFLIX"),
			txns: []domain.Transaction{
				txnWithAccount("netflix", creditCardAcct.ID, 1599),
				txnWithAccount("netflix", creditCardAcct.ID, 1599),
			},
			want: true,
		},
		{
			name: "case-insensitive label signal",
			sub:  makeSub("HOME LOAN"),
			txns: []domain.Transaction{
				txnWithAccount("HOME LOAN", checkingAcct.ID, 90000),
			},
			want: true,
		},
		{
			name: "label signal — 'card payment' in description",
			sub:  makeSub("Card Payment Citi"),
			txns: []domain.Transaction{
				txnWithAccount("Card Payment Citi", checkingAcct.ID, 30000),
			},
			want: true,
		},
		{
			name: "real subscription from checking despite credit card account existing",
			sub:  makeSub("GitHub"),
			txns: []domain.Transaction{
				txnWithAccount("GitHub", checkingAcct.ID, 700),
				txnWithAccount("GitHub", checkingAcct.ID, 700),
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsLiabilityPayment(tc.sub, tc.txns, allAccounts)
			if got != tc.want {
				t.Errorf("IsLiabilityPayment(%q) = %v, want %v", tc.sub.Name, got, tc.want)
			}
		})
	}
}

func TestAccountByID(t *testing.T) {
	accounts := []domain.Account{
		{ID: "a1", Name: "Checking"},
		{ID: "a2", Name: "Credit Card"},
	}

	t.Run("found", func(t *testing.T) {
		a, ok := accountByID(accounts, "a2")
		if !ok || a.Name != "Credit Card" {
			t.Errorf("got %+v ok=%v, want Credit Card ok=true", a, ok)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := accountByID(accounts, "missing")
		if ok {
			t.Error("expected ok=false for missing id")
		}
	})

	t.Run("empty slice", func(t *testing.T) {
		_, ok := accountByID(nil, "a1")
		if ok {
			t.Error("expected ok=false for empty accounts")
		}
	})
}

// TestIsEssentialSpend locks the QA M5 fix: utilities, grocery/pharmacy/tobacco
// runs, rent, insurance, and taxes recur like subscriptions but are everyday
// spending — never cancellable services with "How to cancel" affordances.
func TestIsEssentialSpend(t *testing.T) {
	catNameOf := func(id string) string {
		return map[string]string{
			"cat-util":   "Utilities",
			"cat-health": "Health & Pharmacy",
			"cat-fun":    "Entertainment",
		}[id]
	}
	withCat := func(desc, catID string) domain.Transaction {
		tx := txnWithAccount(desc, "acct-chk", 5000)
		tx.CategoryID = catID
		return tx
	}

	cases := []struct {
		name string
		sub  Subscription
		txns []domain.Transaction
		want bool
	}{
		{"electricity by name", makeSub("Electricity"), nil, true},
		{"gas by name", makeSub("Gas"), nil, true},
		{"cigarettes by name", makeSub("Cigarettes"), nil, true},
		{"pharmacy by name", makeSub("CVS Pharmacy"), nil, true},
		{"rent by name", makeSub("Rent"), nil, true},
		{"utility by category", makeSub("City Services"),
			[]domain.Transaction{withCat("City Services", "cat-util")}, true},
		{"pharmacy by category", makeSub("Corner Store"),
			[]domain.Transaction{withCat("Corner Store", "cat-health")}, true},
		{"streaming service is NOT essential", makeSub("Netflix"),
			[]domain.Transaction{withCat("Netflix", "cat-fun")}, false},
		{"gym is NOT essential", makeSub("Planet Fitness"), nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsEssentialSpend(tc.sub, tc.txns, catNameOf); got != tc.want {
				t.Errorf("IsEssentialSpend(%q) = %v, want %v", tc.sub.Name, got, tc.want)
			}
		})
	}
}

// TestIsRealSubscriptionName locks the QA R3 CF-03 shared rule: price changes,
// renewals, and the annual report must classify a NAME exactly like the active
// subscriptions list — essentials, liability payments, and planned recurring
// flows are never presented as cancellable subscriptions.
func TestIsRealSubscriptionName(t *testing.T) {
	catNameOf := func(id string) string {
		return map[string]string{"cat-util": "Utilities"}[id]
	}
	planned := map[string]bool{"hoa dues": true}
	utilTxn := txnWithAccount("City Power", "acct-chk", 9000)
	utilTxn.CategoryID = "cat-util"

	cases := []struct {
		name string
		txns []domain.Transaction
		want bool
	}{
		{"Netflix", nil, true},
		{"Cigarettes", nil, false},
		{"Gas (Priya's car)", nil, false},
		{"Pharmacy", nil, false},
		{"HOA Dues", nil, false}, // planned recurring flow
		{"Chase Card Payment", nil, false},
		{"City Power", []domain.Transaction{utilTxn}, false}, // essential via category
		{"Planet Fitness", nil, true},
	}
	for _, tc := range cases {
		if got := IsRealSubscriptionName(tc.name, tc.txns, nil, catNameOf, planned); got != tc.want {
			t.Errorf("IsRealSubscriptionName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
