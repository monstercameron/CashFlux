// SPDX-License-Identifier: MIT

package domain

import "testing"

func TestAccountCountsInNetWorth(t *testing.T) {
	for _, archived := range []bool{false, true} {
		for _, excluded := range []bool{false, true} {
			a := Account{Archived: archived, ExcludeFromNetWorth: excluded}
			if got := a.CountsInNetWorth(); got != (!archived && !excluded) {
				t.Fatalf("archived=%v excluded=%v: %v", archived, excluded, got)
			}
		}
	}
}
