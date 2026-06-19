// Package quotes provides a small curated set of finance and motivation quotes
// for the lock screen's "smart quotes" option (B17.5). It is pure Go (no
// syscall/js, no randomness) and rotates deterministically by day, so a given
// date always shows the same quote and tests are reproducible.
package quotes

import "time"

// Quote is a short line plus its attribution.
type Quote struct {
	Text   string
	Author string
}

// curated is the rotating set. Kept short, non-preachy, and free of anything
// that would feel out of place on a household budgeting app's lock screen.
var curated = []Quote{
	{"A budget is telling your money where to go instead of wondering where it went.", "John Maxwell"},
	{"Do not save what is left after spending, but spend what is left after saving.", "Warren Buffett"},
	{"Beware of little expenses; a small leak will sink a great ship.", "Benjamin Franklin"},
	{"It's not your salary that makes you rich, it's your spending habits.", "Charles Jaffe"},
	{"The quickest way to double your money is to fold it and put it back in your pocket.", "Will Rogers"},
	{"Money looks better in the bank than on your feet.", "Sophia Amoruso"},
	{"Never spend your money before you have it.", "Thomas Jefferson"},
	{"Wealth consists not in having great possessions, but in having few wants.", "Epictetus"},
	{"Small amounts saved daily add up to huge investments in the end.", "Margo Vader"},
	{"An investment in knowledge pays the best interest.", "Benjamin Franklin"},
	{"The art is not in making money, but in keeping it.", "Proverb"},
	{"Every time you borrow money, you're robbing your future self.", "Nathan Morris"},
}

// All returns a copy of the curated quotes (defensive, so callers can't mutate
// the package set).
func All() []Quote {
	out := make([]Quote, len(curated))
	copy(out, curated)
	return out
}

// Count is how many curated quotes there are.
func Count() int { return len(curated) }

// OfDay returns the quote for t's calendar day, rotating once per UTC day so the
// same date always shows the same quote (deterministic, no randomness). The set
// is never empty, so this always returns a valid quote.
func OfDay(t time.Time) Quote {
	day := t.UTC().Unix() / 86400 // whole days since the Unix epoch
	idx := int(((day % int64(len(curated))) + int64(len(curated))) % int64(len(curated)))
	return curated[idx]
}
