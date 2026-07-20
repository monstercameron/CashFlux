// SPDX-License-Identifier: MIT

package portfolio

import "testing"

func TestReconcile(t *testing.T) {
	tests := []struct {
		name       string
		accts      []AccountValue
		wantSec    int64
		wantUntr   int64
		wantTotal  int64
		wantBehind bool
	}{
		{
			name:  "empty",
			accts: nil,
		},
		{
			// The review scenario: $28,131.02 in tracked securities inside a
			// brokerage whose recorded balance is $31,490.00 → $3,358.98 of cash &
			// untracked balance.
			name: "cash and untracked positive",
			accts: []AccountValue{
				{AccountID: "b", Name: "Brokerage", BalanceMinor: 3149000, SecuritiesMinor: 2813102},
			},
			wantSec:   2813102,
			wantUntr:  335898,
			wantTotal: 3149000,
		},
		{
			// A balance-tracked ("traditional") account holds no securities, so its
			// whole balance is untracked.
			name: "traditional account all untracked",
			accts: []AccountValue{
				{AccountID: "r", Name: "401k", BalanceMinor: 5000000, SecuritiesMinor: 0},
			},
			wantSec:   0,
			wantUntr:  5000000,
			wantTotal: 5000000,
		},
		{
			// Recorded balance lags the holdings' market value → negative untracked,
			// flagged as behind rather than clamped.
			name: "balance behind holdings",
			accts: []AccountValue{
				{AccountID: "b", Name: "Brokerage", BalanceMinor: 1000000, SecuritiesMinor: 1200000},
			},
			wantSec:    1200000,
			wantUntr:   -200000,
			wantTotal:  1000000,
			wantBehind: true,
		},
		{
			// Multiple accounts sum; one behind and one ahead can net positive overall
			// while the per-account behind flag is still visible in the breakdown.
			name: "mixed accounts net positive",
			accts: []AccountValue{
				{AccountID: "b", Name: "Brokerage", BalanceMinor: 3149000, SecuritiesMinor: 2813102},
				{AccountID: "c", Name: "Crypto", BalanceMinor: 500000, SecuritiesMinor: 650000},
			},
			wantSec:   3463102,
			wantUntr:  185898, // (3149000+500000) - (2813102+650000)
			wantTotal: 3649000,
		},
		{
			name: "exact match no untracked",
			accts: []AccountValue{
				{AccountID: "b", Name: "Brokerage", BalanceMinor: 2813102, SecuritiesMinor: 2813102},
			},
			wantSec:   2813102,
			wantUntr:  0,
			wantTotal: 2813102,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Reconcile(tt.accts)
			if got.SecuritiesMinor != tt.wantSec {
				t.Errorf("SecuritiesMinor = %d, want %d", got.SecuritiesMinor, tt.wantSec)
			}
			if got.UntrackedMinor != tt.wantUntr {
				t.Errorf("UntrackedMinor = %d, want %d", got.UntrackedMinor, tt.wantUntr)
			}
			if got.AccountsTotalMinor != tt.wantTotal {
				t.Errorf("AccountsTotalMinor = %d, want %d", got.AccountsTotalMinor, tt.wantTotal)
			}
			if got.BalanceBehind != tt.wantBehind {
				t.Errorf("BalanceBehind = %v, want %v", got.BalanceBehind, tt.wantBehind)
			}
			// The identity must always hold exactly.
			if got.SecuritiesMinor+got.UntrackedMinor != got.AccountsTotalMinor {
				t.Errorf("identity broken: %d + %d != %d",
					got.SecuritiesMinor, got.UntrackedMinor, got.AccountsTotalMinor)
			}
			if len(got.Accounts) != len(tt.accts) {
				t.Fatalf("Accounts len = %d, want %d", len(got.Accounts), len(tt.accts))
			}
		})
	}
}

func TestReconcilePerAccountBreakdown(t *testing.T) {
	accts := []AccountValue{
		{AccountID: "b", Name: "Brokerage", BalanceMinor: 1000000, SecuritiesMinor: 1200000}, // behind
		{AccountID: "c", Name: "Cash", BalanceMinor: 500000, SecuritiesMinor: 0},              // all untracked
	}
	got := Reconcile(accts)
	// Order preserved.
	if got.Accounts[0].AccountID != "b" || got.Accounts[1].AccountID != "c" {
		t.Fatalf("account order not preserved: %+v", got.Accounts)
	}
	// First account is behind (holdings exceed recorded balance).
	if !got.Accounts[0].BalanceBehind || got.Accounts[0].UntrackedMinor != -200000 {
		t.Errorf("account[0] = %+v, want behind with untracked -200000", got.Accounts[0])
	}
	// Second account: whole balance is untracked, not behind.
	if got.Accounts[1].BalanceBehind || got.Accounts[1].UntrackedMinor != 500000 {
		t.Errorf("account[1] = %+v, want untracked 500000 not behind", got.Accounts[1])
	}
}
