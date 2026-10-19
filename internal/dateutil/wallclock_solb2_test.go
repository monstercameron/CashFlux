// SPDX-License-Identifier: MIT

package dateutil

import (
	"testing"
	"time"
)

func TestSOLB2WallClockUTC(t *testing.T) {
	for _, offset := range []int{-4 * 3600, 9 * 3600, 0} {
		local := time.Date(2026, 10, 31, 21, 20, 19, 123, time.FixedZone("local", offset))
		got := WallClockUTC(local)
		want := time.Date(2026, 10, 31, 21, 20, 19, 123, time.UTC)
		if got != want || WallClockUTC(got) != got {
			t.Fatalf("offset %d: %v", offset, got)
		}
	}
}
