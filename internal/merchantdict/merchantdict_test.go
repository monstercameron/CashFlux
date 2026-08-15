// SPDX-License-Identifier: MIT

package merchantdict

import "testing"

func TestLookupRealDescriptors(t *testing.T) {
	// Raw descriptors in the shape banks actually emit, including the
	// payment-processor prefixes payeealias strips before we see them.
	tests := []struct {
		name     string
		raw      string
		merchant string
		cat      Category
	}{
		{"plain name", "STARBUCKS", "Starbucks", CatCoffee},
		{"store number", "TRADER JOE'S #453 QPS", "Trader Joe's", CatGroceries},
		{"amazon marketplace alias", "AMZN MKTP US*2H4RT9", "Amazon", CatShopping},
		{"amazon dot com alias", "AMZN.COM/BILL WA", "Amazon", CatShopping},
		{"square prefix", "SQ *BLUE BOTTLE COFFE", "Blue Bottle", CatCoffee},
		{"toast prefix", "TST* SWEETGREEN 210", "Sweetgreen", CatDining},
		{"gas with ref", "SHELL OIL 57445208", "Shell", CatGas},
		{"phone with punctuation", "T-MOBILE PCS SVC", "T-Mobile", CatPhone},
		{"ampersand in name", "BED BATH & BEYOND 1187", "Bed Bath & Beyond", CatHousehold},
		{"apostrophe dropped", "LOWES #1234", "Lowes", CatHome},
		{"streaming", "NETFLIX.COM 866-579-7172", "Netflix", CatSubscriptions},
		{"case insensitive", "planet fitness club", "Planet Fitness", CatFitness},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Lookup(tc.raw)
			if !ok {
				t.Fatalf("Lookup(%q) found nothing, want %q", tc.raw, tc.merchant)
			}
			if got.Merchant != tc.merchant {
				t.Errorf("Lookup(%q) merchant = %q, want %q", tc.raw, got.Merchant, tc.merchant)
			}
			if got.Category != tc.cat {
				t.Errorf("Lookup(%q) category = %q, want %q", tc.raw, got.Category, tc.cat)
			}
		})
	}
}

// TestLookupPaymentProcessorsResolveToTheMerchant is the trap named in C514: the
// processor is a prefix to strip, never the merchant to report.
func TestLookupPaymentProcessorsResolveToTheMerchant(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"PAYPAL *STEAM GAMES", "Steam"},
		{"SQ *BLUE BOTTLE COFFE", "Blue Bottle"},
		{"TST* WINGSTOP 442", "Wingstop"},
	}
	for _, tc := range tests {
		got, ok := Lookup(tc.raw)
		if !ok {
			t.Errorf("Lookup(%q) found nothing, want %q", tc.raw, tc.want)
			continue
		}
		if got.Merchant != tc.want {
			t.Errorf("Lookup(%q) = %q, want %q", tc.raw, got.Merchant, tc.want)
		}
	}
	// The processors themselves must not be dictionary entries — an entry for
	// "PayPal" would shadow every merchant billed through it.
	for _, proc := range []string{"paypal", "square", "toast", "clover", "stripe"} {
		if _, ok := byKey[proc]; ok {
			t.Errorf("payment processor %q must not be a dictionary entry", proc)
		}
	}
}

// TestLookupRejectsUnknownAndPartialWords guards the naive-substring failure.
func TestLookupRejectsUnknownAndPartialWords(t *testing.T) {
	for _, raw := range []string{
		"",
		"   ",
		"ACH DEBIT ELAN FIN SVC",     // genuinely unknown
		"POS DEBIT 4471",             // no merchant at all
		"ZZQX HOLDINGS LLC",          // unknown business
		"GAPING VOID STUDIO",         // must NOT match "Gap"
		"TARGETED THERAPY CLINIC",    // must NOT match "Target"
		"SUBWAYS OF NEW YORK MUSEUM", // must NOT match "Subway"
	} {
		if e, ok := Lookup(raw); ok {
			t.Errorf("Lookup(%q) matched %q/%q, want no match", raw, e.Merchant, e.Category)
		}
	}
}

// TestLookupPrefersTheLongestMatch: when two entries could match, the more
// specific merchant must win.
func TestLookupPrefersTheLongestMatch(t *testing.T) {
	got, ok := Lookup("APPLE STORE R123 PALO ALTO")
	if !ok {
		t.Fatal("Lookup(apple store) found nothing")
	}
	if got.Merchant != "Apple Store" {
		t.Errorf("got %q, want Apple Store (longest match)", got.Merchant)
	}
}

func TestAliasesCoverEverySlugUsedByTheTable(t *testing.T) {
	seen := map[Category]bool{}
	for _, e := range table {
		seen[e.Category] = true
	}
	for cat := range seen {
		if len(cat.Aliases()) == 0 {
			t.Errorf("category %q is used by the table but has no aliases, so it can never "+
				"resolve onto a user's category", cat)
		}
	}
}

// TestTableIsWellFormed catches editing mistakes that would silently weaken the
// dictionary: blank fields, or a duplicate that shadows an earlier entry.
func TestTableIsWellFormed(t *testing.T) {
	seen := map[string]string{}
	for _, e := range table {
		if e.Merchant == "" {
			t.Error("entry with an empty merchant name")
			continue
		}
		if e.Category == "" {
			t.Errorf("%q has no category", e.Merchant)
		}
		k := key(e.Merchant)
		if k == "" {
			t.Errorf("%q normalizes to an empty key", e.Merchant)
			continue
		}
		if prev, dup := seen[k]; dup {
			t.Errorf("%q duplicates %q (both key to %q) — the later one is dead", e.Merchant, prev, k)
		}
		seen[k] = e.Merchant
	}
	if Len() < 200 {
		t.Errorf("dictionary has %d entries; it should not shrink below 200", Len())
	}
}

func TestKeyNormalization(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Trader Joe's", "trader joes"},
		{"TRADER JOES", "trader joes"},
		{"Bed Bath & Beyond", "bed bath & beyond"},
		{"H-E-B", "h e b"},
		{"  spaced   out  ", "spaced out"},
		{"", ""},
		{"!!!", ""},
	}
	for _, tc := range tests {
		if got := key(tc.in); got != tc.want {
			t.Errorf("key(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestContainsAtTokenBoundary(t *testing.T) {
	tests := []struct {
		hay, need string
		want      bool
	}{
		{"blue bottle coffee", "blue bottle", true},
		{"the blue bottle", "blue bottle", true},
		{"blue bottle", "blue bottle", true},
		{"bluebottle", "blue bottle", false},
		{"gaping void", "gap", false},
		{"gap kids", "gap", true},
		{"targeted therapy", "target", false},
		{"anything", "", false},
		{"short", "much longer needle", false},
	}
	for _, tc := range tests {
		if got := containsAtTokenBoundary(tc.hay, tc.need); got != tc.want {
			t.Errorf("containsAtTokenBoundary(%q, %q) = %v, want %v", tc.hay, tc.need, got, tc.want)
		}
	}
}
