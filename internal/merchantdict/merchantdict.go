// SPDX-License-Identifier: MIT

// Package merchantdict maps well-known merchants to a default spending category
// (C514). It exists for the cold start: a household with no rules and no history
// has nothing to categorize its first import with, and the only other way out is
// paying for an LLM pass. This table is SMART, not SMART+ — it is local,
// deterministic, costs nothing, and never leaves the device.
//
// Matching runs on the output of payeealias.Normalize, which has already stripped
// payment-processor prefixes ("SQ *", "PAYPAL *", "TST*", "DD *DOORDASH") and
// canonicalized the obvious aliases. That is what keeps the matcher honest:
// "PAYPAL *STEAM GAMES" must resolve to Steam rather than PayPal, and the
// processor is a prefix to remove, not a merchant to match.
//
// The table maps to canonical Category SLUGS, never to the user's category IDs —
// at first run the user may not have created any categories yet. Resolving a slug
// onto a real category is the caller's job (see internal/catsuggest).
package merchantdict

import (
	"sort"
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/payeealias"
)

// Category is a canonical spending bucket. It is deliberately a small closed set
// — the dictionary describes what a merchant SELLS, and the user's own category
// tree is matched onto it by name (see Aliases).
type Category string

// The canonical buckets. Keep this list short: every addition needs aliases that
// map onto the categories real households actually create.
const (
	CatGroceries     Category = "groceries"
	CatDining        Category = "dining"
	CatCoffee        Category = "coffee"
	CatGas           Category = "gas"
	CatTransport     Category = "transport"
	CatTravel        Category = "travel"
	CatShopping      Category = "shopping"
	CatHousehold     Category = "household"
	CatSubscriptions Category = "subscriptions"
	CatEntertainment Category = "entertainment"
	CatHealth        Category = "health"
	CatPharmacy      Category = "pharmacy"
	CatUtilities     Category = "utilities"
	CatInternet      Category = "internet"
	CatPhone         Category = "phone"
	CatInsurance     Category = "insurance"
	CatFitness       Category = "fitness"
	CatPets          Category = "pets"
	CatKids          Category = "kids"
	CatHome          Category = "home"
	CatElectronics   Category = "electronics"
	CatClothing      Category = "clothing"
	CatEducation     Category = "education"
	CatCharity       Category = "charity"
	CatFees          Category = "fees"
)

// Aliases returns candidate category NAMES for a slug, most specific first. A
// caller matches these (case-insensitively) against the household's real
// categories and uses the first that exists — so a user with "Dining out" and a
// user with "Restaurants" both get a hit, and a user with neither gets none
// rather than having a category invented for them.
func (c Category) Aliases() []string {
	switch c {
	case CatGroceries:
		return []string{"Groceries", "Food", "Supermarket"}
	case CatDining:
		return []string{"Dining", "Dining out", "Restaurants", "Eating out", "Food"}
	case CatCoffee:
		return []string{"Coffee", "Coffee shops", "Dining", "Dining out", "Restaurants"}
	case CatGas:
		return []string{"Gas", "Fuel", "Petrol", "Transportation", "Auto"}
	case CatTransport:
		return []string{"Transportation", "Transport", "Travel", "Auto"}
	case CatTravel:
		return []string{"Travel", "Holidays", "Vacation"}
	case CatShopping:
		return []string{"Shopping", "General merchandise"}
	case CatHousehold:
		return []string{"Household", "Home", "Home supplies", "Shopping"}
	case CatSubscriptions:
		return []string{"Subscriptions", "Streaming", "Memberships", "Entertainment"}
	case CatEntertainment:
		return []string{"Entertainment", "Fun", "Leisure"}
	case CatHealth:
		return []string{"Health & Fitness", "Health", "Medical", "Healthcare"}
	case CatPharmacy:
		return []string{"Pharmacy", "Health & Fitness", "Health", "Medical"}
	case CatUtilities:
		return []string{"Utilities", "Bills"}
	case CatInternet:
		return []string{"Internet", "Utilities", "Bills"}
	case CatPhone:
		return []string{"Phone", "Mobile", "Utilities", "Bills"}
	case CatInsurance:
		return []string{"Insurance"}
	case CatFitness:
		return []string{"Fitness", "Gym", "Health & Fitness", "Health"}
	case CatPets:
		return []string{"Pets", "Pet care"}
	case CatKids:
		return []string{"Baby & Childcare", "Kids", "Childcare", "Children"}
	case CatHome:
		return []string{"Home improvement", "Home", "Housing", "Household"}
	case CatElectronics:
		return []string{"Electronics", "Technology", "Shopping"}
	case CatClothing:
		return []string{"Clothing", "Clothes", "Apparel", "Shopping"}
	case CatEducation:
		return []string{"Education & Loans", "Education", "Learning"}
	case CatCharity:
		return []string{"Gifts & Charity", "Charity", "Donations", "Giving"}
	case CatFees:
		return []string{"Fees & Charges", "Fees", "Bank fees"}
	}
	return nil
}

// Entry is one dictionary hit: the merchant's display name and what it sells.
type Entry struct {
	// Merchant is the canonical display name ("Trader Joe's", not "TRADER JOE'S #453").
	Merchant string
	// Category is the default bucket for this merchant.
	Category Category
}

// key is the lookup form of a merchant name: lower-cased, punctuation dropped,
// whitespace collapsed. "Trader Joe's" and "TRADER JOES" share a key.
func key(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		case r == '\'' || r == '’':
			// Apostrophes are DROPPED, not separated on: "Joe's" and "JOES" must
			// key alike, and splitting would strand a bare "s" token.
		case r == '&':
			// "&" carries meaning in names ("Bed Bath & Beyond") — keep it as a token.
			if !space {
				b.WriteByte(' ')
			}
			b.WriteByte('&')
			b.WriteByte(' ')
			space = true
		default:
			if !space && b.Len() > 0 {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// index is the table keyed for exact lookup, plus the same keys ordered longest
// first for the token-boundary pass. Built once at init: the table is a compile
// time constant, so there is no reason to rebuild it per call.
var (
	byKey    map[string]Entry
	byLength []string
)

func init() {
	byKey = make(map[string]Entry, len(table))
	byLength = make([]string, 0, len(table))
	for _, e := range table {
		k := key(e.Merchant)
		if k == "" {
			continue
		}
		if _, dup := byKey[k]; dup {
			continue // first definition wins; keeps the table order meaningful
		}
		byKey[k] = e
		byLength = append(byLength, k)
	}
	// Longest first so "whole foods" beats a hypothetical "foods", and so the
	// most specific merchant wins when two entries could both match.
	sort.Slice(byLength, func(i, j int) bool {
		if len(byLength[i]) != len(byLength[j]) {
			return len(byLength[i]) > len(byLength[j])
		}
		return byLength[i] < byLength[j] // stable, deterministic
	})
}

// containsAtTokenBoundary reports whether need appears in hay bounded by token
// edges. This is the guard against the naive-substring failure: "SP * THE FEED"
// normalizes to a name containing "feed", and a bare strings.Contains would match
// any merchant whose name embeds another's.
func containsAtTokenBoundary(hay, need string) bool {
	if need == "" || len(need) > len(hay) {
		return false
	}
	for i := 0; i+len(need) <= len(hay); i++ {
		if hay[i:i+len(need)] != need {
			continue
		}
		if i > 0 && hay[i-1] != ' ' {
			continue
		}
		if end := i + len(need); end < len(hay) && hay[end] != ' ' {
			continue
		}
		return true
	}
	return false
}

// Lookup resolves a RAW bank descriptor to a dictionary entry. It normalizes
// through payeealias first (stripping processor prefixes), then tries an exact
// key match before falling back to the longest token-boundary match, so
// "TRADER JOE'S #453 QPS" still resolves after its trailing reference survives
// cleaning. Returns ok=false when nothing matches — an unknown merchant must
// produce no suggestion rather than a bad one.
func Lookup(rawPayee string) (Entry, bool) {
	clean := key(payeealias.Normalize(rawPayee))
	if clean == "" {
		return Entry{}, false
	}
	if e, ok := byKey[clean]; ok {
		return e, true
	}
	for _, k := range byLength {
		if containsAtTokenBoundary(clean, k) {
			return byKey[k], true
		}
	}
	return Entry{}, false
}

// Len reports how many merchants the dictionary knows. Exposed so a UI can say
// what it is matching against, and so a test can assert the table did not shrink.
func Len() int { return len(byKey) }
