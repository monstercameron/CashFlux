// SPDX-License-Identifier: MIT

package catname

import "github.com/monstercameron/CashFlux/internal/domain"

// Label is how a category is NAMED to a user, anywhere in the app (C619).
//
// The app had drifted into three conventions at once: always-qualified with
// " > " (Path), always-qualified with " › " (the category picker's own
// path building), and bare leaf name (the ledger's Category column). The same
// category therefore read as "Transportation > Gas" in Review and "Gas" in the
// ledger — which looks like two different categories to anyone scanning.
//
// One rule instead: show the LEAF name, and qualify it only when the leaf alone
// would be ambiguous — i.e. when some other category shares that name. So a
// household with one "Gas" sees "Gas" everywhere, and a household with "Auto >
// Gas" and "Home > Gas" sees both qualified everywhere. Qualification is a
// property of the DATA, not of which screen you happen to be on, which is what
// makes it consistent.
//
// Ambiguity is judged with the same case/whitespace folding as the rest of this
// package (EqualNames), so "Gas" and "gas " count as the same name — they are
// indistinguishable in a list, which is the whole point.
func Label(cats []domain.Category, c domain.Category) string {
	if Ambiguous(cats, c) {
		return Path(cats, c)
	}
	return c.Name
}

// Ambiguous reports whether another category shares c's leaf name, making the
// bare name insufficient to identify it. A category is never ambiguous with
// itself; matching is by id, so two distinct categories that happen to share an
// id (impossible through the store) still resolve safely.
func Ambiguous(cats []domain.Category, c domain.Category) bool {
	for _, x := range cats {
		if x.ID == c.ID {
			continue
		}
		if EqualNames(x.Name, c.Name) {
			return true
		}
	}
	return false
}

// LabelByID is Label for a category id, returning "" when the id resolves to
// nothing — the shape most call sites need, since they hold an id from a
// transaction or a budget rather than the category itself.
func LabelByID(cats []domain.Category, id string) string {
	if id == "" {
		return ""
	}
	for _, c := range cats {
		if c.ID == id {
			return Label(cats, c)
		}
	}
	return ""
}
