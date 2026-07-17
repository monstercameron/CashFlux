// SPDX-License-Identifier: MIT

package freshness

import (
	"fmt"
	"time"
)

// RelAge renders the age of t relative to now as a compact unit figure for the
// top-bar "Updated …" stamp: "now" under a minute, then "4m", "3h", "12d",
// "5mo". Returns "" for a zero t (nothing recorded yet) or a t in the future
// (clock skew — an age of "-2m" would read as a bug). Pure, so the stamp's
// wording is table-testable without a browser.
func RelAge(t, now time.Time) string {
	if t.IsZero() || t.After(now) {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo", int(d.Hours()/(24*30)))
	}
}
