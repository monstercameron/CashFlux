// SPDX-License-Identifier: MIT

package merchantdict

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzLookup: a bank descriptor is arbitrary bytes. Lookup must never panic and
// must never return a merchant the table does not contain — a wrong-but-confident
// category is worse than none, because the user has no reason to check it.
func FuzzLookup(f *testing.F) {
	for _, seed := range []string{
		"", " ", "\x00", "\n\t",
		"STARBUCKS", "TRADER JOE'S #453 QPS", "SQ *BLUE BOTTLE COFFE",
		"PAYPAL *STEAM GAMES", "AMZN MKTP US*2H4RT9",
		"GAPING VOID", "TARGETED THERAPY", "SUBWAYS OF NEW YORK",
		"POS DEBIT 4471", "ACH DEBIT ELAN FIN SVC",
		"\ufeffSTARBUCKS", // BOM
		"\uff33\uff34\uff21\uff32\uff22\uff35\uff23\uff2b\uff33", // fullwidth
		"Café Amazôn",
		strings.Repeat("SHELL ", 200),
		strings.Repeat("*", 500),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		e, ok := Lookup(raw)
		if !ok {
			if e != (Entry{}) {
				t.Errorf("miss returned a non-zero Entry: %+v", e)
			}
			return
		}
		if e.Merchant == "" || e.Category == "" {
			t.Fatalf("hit returned an incomplete Entry: %+v", e)
		}
		// The hit must be a REAL table entry, not something assembled from input.
		if _, known := byKey[key(e.Merchant)]; !known {
			t.Errorf("Lookup(%q) returned %q, which is not in the table", raw, e.Merchant)
		}
		// Every returned bucket must be resolvable onto a user's categories, or
		// the hit is useless downstream.
		if len(e.Category.Aliases()) == 0 {
			t.Errorf("category %q has no aliases", e.Category)
		}
		// A hit must round-trip: the merchant's own name finds the same entry.
		again, ok2 := Lookup(e.Merchant)
		if !ok2 || again.Merchant != e.Merchant {
			t.Errorf("Lookup is not idempotent: %q -> %q -> %+v", raw, e.Merchant, again)
		}
	})
}

// FuzzKey: the normalizer feeds a map, so it must be total, idempotent, and
// never emit anything that would make two different merchants collide by
// accident (leading/trailing space, doubled spaces).
func FuzzKey(f *testing.F) {
	for _, seed := range []string{"", " ", "Trader Joe's", "H-E-B", "Bed Bath & Beyond", "  a  b  ", "\x00\x01", "Ünïcôdé"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		k := key(in)
		if k != key(k) {
			t.Errorf("key is not idempotent: %q -> %q -> %q", in, k, key(k))
		}
		if k != strings.TrimSpace(k) {
			t.Errorf("key %q has edge whitespace", k)
		}
		if strings.Contains(k, "  ") {
			t.Errorf("key %q has a doubled space", k)
		}
		if !utf8.ValidString(k) {
			t.Errorf("key %q is not valid UTF-8", k)
		}
		if strings.ToLower(k) != k {
			t.Errorf("key %q is not lower-cased", k)
		}
	})
}

// TestNoEntryIsASubstringTrapForAnother guards the failure mode the matcher was
// built to avoid: a SHORT entry whose key appears as a whole token inside a
// longer entry's name silently steals its matches unless longest-match wins.
func TestNoEntryIsASubstringTrapForAnother(t *testing.T) {
	for _, e := range table {
		got, ok := Lookup(e.Merchant)
		if !ok {
			t.Errorf("%q does not match itself", e.Merchant)
			continue
		}
		// Looking up a merchant by its own name must return THAT merchant, never
		// a shorter entry embedded in it ("Apple Store" must not yield "Apple").
		if got.Merchant != e.Merchant {
			t.Errorf("%q resolved to %q — a shorter entry is shadowing it", e.Merchant, got.Merchant)
		}
	}
}

// TestEveryEntryResolvesThroughRealDescriptorShapes: entries are useless if they
// only match when typed perfectly. Each must survive the mangling banks apply.
func TestEveryEntryResolvesThroughRealDescriptorShapes(t *testing.T) {
	shapes := []func(string) string{
		strings.ToUpper,
		strings.ToLower,
		func(s string) string { return s + " #1234" },
		func(s string) string { return s + " 425-6816830" },
		func(s string) string { return "  " + s + "  " },
	}
	for _, e := range table {
		for i, shape := range shapes {
			raw := shape(e.Merchant)
			got, ok := Lookup(raw)
			if !ok {
				t.Errorf("shape %d: Lookup(%q) found nothing (entry %q)", i, raw, e.Merchant)
				continue
			}
			if got.Category != e.Category {
				t.Errorf("shape %d: Lookup(%q) = %q/%q, want category %q",
					i, raw, got.Merchant, got.Category, e.Category)
			}
		}
	}
}

// TestAliasesAreOrderedMostSpecificFirst: the resolver takes the FIRST alias a
// household has, so a generic fallback listed before a specific name would send
// every coffee shop to "Dining" even when "Coffee" exists.
func TestAliasesAreOrderedMostSpecificFirst(t *testing.T) {
	cases := map[Category][2]string{
		CatCoffee:   {"Coffee", "Dining"},
		CatGas:      {"Gas", "Transportation"},
		CatPharmacy: {"Pharmacy", "Health & Fitness"},
		CatFitness:  {"Fitness", "Health"},
		CatInternet: {"Internet", "Utilities"},
	}
	for cat, want := range cases {
		aliases := cat.Aliases()
		specific, generic := -1, -1
		for i, a := range aliases {
			if a == want[0] {
				specific = i
			}
			if a == want[1] && generic == -1 {
				generic = i
			}
		}
		if specific == -1 || generic == -1 {
			t.Errorf("%q aliases %v are missing %q or %q", cat, aliases, want[0], want[1])
			continue
		}
		if specific > generic {
			t.Errorf("%q lists the generic %q before the specific %q — a household with both "+
				"would get the wrong one", cat, want[1], want[0])
		}
	}
}

// TestNoAliasIsEmptyOrDuplicated keeps the resolution list honest.
func TestNoAliasIsEmptyOrDuplicated(t *testing.T) {
	seen := map[Category]bool{}
	for _, e := range table {
		if seen[e.Category] {
			continue
		}
		seen[e.Category] = true
		dupes := map[string]bool{}
		for _, a := range e.Category.Aliases() {
			if strings.TrimSpace(a) == "" {
				t.Errorf("%q has an empty alias", e.Category)
			}
			if dupes[a] {
				t.Errorf("%q lists alias %q twice", e.Category, a)
			}
			dupes[a] = true
		}
	}
}

// TestLookupIsDeterministic: the table is indexed once at init and iterated in a
// sorted order, so repeated lookups must not depend on map iteration order.
func TestLookupIsDeterministic(t *testing.T) {
	for _, raw := range []string{"SHELL OIL 5744", "APPLE STORE R123", "GAS STATION", "SQ *SOMETHING"} {
		first, ok1 := Lookup(raw)
		for i := 0; i < 200; i++ {
			got, ok := Lookup(raw)
			if ok != ok1 || got != first {
				t.Fatalf("Lookup(%q) is not deterministic: %+v/%v then %+v/%v", raw, first, ok1, got, ok)
			}
		}
	}
}
