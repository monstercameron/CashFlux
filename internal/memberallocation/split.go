// SPDX-License-Identifier: MIT

// Package memberallocation apportions transaction minor units across owners.
package memberallocation

// Share is one owner's exact integer minor-unit share.
type Share struct {
	MemberID string
	Amount   int64
}

// SplitAmount divides amount evenly among owners. The first owners in the
// supplied stable order receive any leftover minor units; the signed shares
// always sum to amount and differ by at most one. Empty owners returns nil.
func SplitAmount(amount int64, owners []string) []Share {
	if len(owners) == 0 {
		return nil
	}
	base := amount / int64(len(owners))
	remainder := amount % int64(len(owners))
	out := make([]Share, len(owners))
	for i, owner := range owners {
		share := base
		if int64(i) < absMinor(remainder) {
			if amount < 0 {
				share--
			} else {
				share++
			}
		}
		out[i] = Share{MemberID: owner, Amount: share}
	}
	return out
}

func absMinor(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
