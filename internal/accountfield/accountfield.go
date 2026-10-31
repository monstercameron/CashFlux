// SPDX-License-Identifier: MIT

// Package accountfield holds the pure conversions between an Account and the
// text its edit forms show, so the meaning of an empty box is decided in one
// tested place rather than in view code.
package accountfield

import (
	"math"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// RateText renders an account's APR for a form field, or "" when no rate has been
// recorded, so an empty box reads back as no rate. A recorded 0% shows as "0",
// because somebody typed it. A rate that is not a finite number also shows as "".
func RateText(a domain.Account) string {
	r, ok := a.RateAPR()
	if !ok || math.IsNaN(r) || math.IsInf(r, 0) {
		return ""
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// CloneShares returns a copy of src, or nil when src is nil or empty. Form state
// seeded from it never aliases the account's own map.
func CloneShares(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
