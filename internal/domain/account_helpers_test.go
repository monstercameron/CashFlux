// SPDX-License-Identifier: MIT

package domain_test

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// acct builds a minimal Account with the given institution string.
func acctInst(id, institution string) domain.Account {
	return domain.Account{ID: id, Institution: institution}
}

func TestUniqueInstitutions(t *testing.T) {
	tests := []struct {
		name     string
		accounts []domain.Account
		want     []string
	}{
		{
			name:     "empty input",
			accounts: nil,
			want:     []string{},
		},
		{
			name: "all blanks skipped",
			accounts: []domain.Account{
				acctInst("a1", ""),
				acctInst("a2", ""),
			},
			want: []string{},
		},
		{
			name: "single institution",
			accounts: []domain.Account{
				acctInst("a1", "Chase"),
			},
			want: []string{"Chase"},
		},
		{
			name: "dedup — exact duplicates",
			accounts: []domain.Account{
				acctInst("a1", "Chase"),
				acctInst("a2", "Chase"),
				acctInst("a3", "Chase"),
			},
			want: []string{"Chase"},
		},
		{
			name: "dedup — case-insensitive, preserve first-seen casing",
			accounts: []domain.Account{
				acctInst("a1", "Chase"),
				acctInst("a2", "CHASE"),
				acctInst("a3", "chase"),
			},
			want: []string{"Chase"}, // first-seen casing preserved
		},
		{
			name: "blanks skipped, rest deduped",
			accounts: []domain.Account{
				acctInst("a1", ""),
				acctInst("a2", "Wells Fargo"),
				acctInst("a3", ""),
				acctInst("a4", "Wells Fargo"),
			},
			want: []string{"Wells Fargo"},
		},
		{
			name: "sort order — case-insensitive",
			accounts: []domain.Account{
				acctInst("a1", "Wells Fargo"),
				acctInst("a2", "Chase"),
				acctInst("a3", "Bank of America"),
			},
			want: []string{"Bank of America", "Chase", "Wells Fargo"},
		},
		{
			name: "sort order — mixed case",
			accounts: []domain.Account{
				acctInst("a1", "Zions"),
				acctInst("a2", "ally"),
				acctInst("a3", "Fidelity"),
			},
			// case-insensitive: ally < Fidelity < Zions
			want: []string{"ally", "Fidelity", "Zions"},
		},
		{
			name: "first-seen casing preserved on dedup + correct sort",
			accounts: []domain.Account{
				acctInst("a1", "WELLS FARGO"), // first seen
				acctInst("a2", "wells fargo"), // duplicate — suppressed
				acctInst("a3", "chase"),       // first seen
				acctInst("a4", "Chase"),       // duplicate — suppressed
			},
			// sort: chase < wells fargo (case-insensitive)
			want: []string{"chase", "WELLS FARGO"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.UniqueInstitutions(tt.accounts)
			// Normalise nil vs empty slice for comparison.
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UniqueInstitutions() = %v, want %v", got, tt.want)
			}
		})
	}
}
