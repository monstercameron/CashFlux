// SPDX-License-Identifier: MIT

package taskresolve

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestResolves(t *testing.T) {
	tests := []struct {
		name    string
		rule    domain.TaskResolve
		event   Event
		want    bool
		wantErr bool
	}{
		{
			name: "empty rule never resolves",
			rule: domain.TaskResolve{},
			want: false,
		},
		{
			name:  "condition truthy resolves",
			rule:  domain.TaskResolve{Condition: "balance_updated > 0"},
			event: Event{Vars: map[string]float64{"balance_updated": 1}},
			want:  true,
		},
		{
			name:  "condition false does not resolve",
			rule:  domain.TaskResolve{Condition: "balance_updated > 0"},
			event: Event{Vars: map[string]float64{"balance_updated": 0}},
			want:  false,
		},
		{
			name:    "malformed condition errors and does not resolve",
			rule:    domain.TaskResolve{Condition: "this is not && valid"},
			event:   Event{Vars: map[string]float64{}},
			want:    false,
			wantErr: true,
		},
		{
			name:  "refund matcher: payee + magnitude + credit sign",
			rule:  domain.TaskResolve{MatchPayee: "Acme", MatchAmountMinor: 4200, MatchRefund: true},
			event: Event{Txn: &TxnEvent{Payee: "ACME Store", AmountMinor: 4200}},
			want:  true,
		},
		{
			name:  "refund matcher rejects a debit (negative) when MatchRefund",
			rule:  domain.TaskResolve{MatchPayee: "Acme", MatchAmountMinor: 4200, MatchRefund: true},
			event: Event{Txn: &TxnEvent{Payee: "Acme", AmountMinor: -4200}},
			want:  false,
		},
		{
			name:  "matcher rejects wrong payee",
			rule:  domain.TaskResolve{MatchPayee: "Acme", MatchAmountMinor: 4200},
			event: Event{Txn: &TxnEvent{Payee: "Other", AmountMinor: 4200}},
			want:  false,
		},
		{
			name:  "amount within tolerance matches",
			rule:  domain.TaskResolve{MatchPayee: "Acme", MatchAmountMinor: 4200, MatchToleranceMinor: 100, MatchRefund: true},
			event: Event{Txn: &TxnEvent{Payee: "Acme", AmountMinor: 4150}},
			want:  true,
		},
		{
			name:  "amount outside tolerance fails",
			rule:  domain.TaskResolve{MatchPayee: "Acme", MatchAmountMinor: 4200, MatchToleranceMinor: 100, MatchRefund: true},
			event: Event{Txn: &TxnEvent{Payee: "Acme", AmountMinor: 4000}},
			want:  false,
		},
		{
			name:  "currency mismatch fails",
			rule:  domain.TaskResolve{MatchPayee: "Acme", MatchCurrency: "USD"},
			event: Event{Txn: &TxnEvent{Payee: "Acme", AmountMinor: 100, Currency: "EUR"}},
			want:  false,
		},
		{
			name:  "matcher fires without txn present -> no resolve, falls to condition",
			rule:  domain.TaskResolve{MatchPayee: "Acme", Condition: "x > 0"},
			event: Event{Vars: map[string]float64{"x": 5}},
			want:  true,
		},
		{
			name:  "payee-only matcher (no amount) matches on payee",
			rule:  domain.TaskResolve{MatchPayee: "Netflix"},
			event: Event{Txn: &TxnEvent{Payee: "NETFLIX.COM", AmountMinor: -1599}},
			want:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolves(tc.rule, tc.event)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Resolves = %v, want %v", got, tc.want)
			}
		})
	}
}
