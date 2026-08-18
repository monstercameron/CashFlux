// SPDX-License-Identifier: MIT

package merchantdict

import "testing"

// C691: the household's real utility descriptors. The spelled-out names already
// in the table ("Florida Power & Light", "Comcast") are not what a bank prints,
// so these assert against the DESCRIPTOR forms that actually arrive on a
// statement — the only form that matters for categorizing an import.
func TestUtilityDescriptorsResolve(t *testing.T) {
	tests := []struct {
		descriptor string
		want       Category
	}{
		{"FPL DIRECT DEBIT", CatUtilities},
		{"FPL ELECTRIC PAYMENT", CatUtilities},
		{"CITY OF LAUDERHILL UTILITY", CatUtilities},
		{"WIMBLEDON HOA DUES", CatUtilities},
		{"HOA ASSESSMENT", CatUtilities},
		{"COMCAST CABLE COMM", CatInternet},
		{"GOOGLE FI", CatPhone},
	}
	for _, tt := range tests {
		t.Run(tt.descriptor, func(t *testing.T) {
			got, ok := Lookup(tt.descriptor)
			if !ok {
				t.Fatalf("Lookup(%q) found nothing — the descriptor will import uncategorized", tt.descriptor)
			}
			if got.Category != tt.want {
				t.Errorf("Lookup(%q) = %q (%s), want %q", tt.descriptor, got.Category, got.Merchant, tt.want)
			}
		})
	}
}

// Google Fi (mobile) and Google Fiber (internet) are different products whose
// descriptors share a prefix. Longest-key-first matching is what keeps them
// apart, and it is worth pinning: adding "Google Fi" naively ahead of the longer
// key would silently refile every Fiber bill as a phone bill.
func TestGoogleFiDoesNotShadowGoogleFiber(t *testing.T) {
	fiber, ok := Lookup("GOOGLE FIBER MONTHLY")
	if !ok || fiber.Category != CatInternet {
		t.Errorf("GOOGLE FIBER = %+v ok=%v, want internet — the shorter Google Fi key shadowed it", fiber, ok)
	}
	fi, ok := Lookup("GOOGLE FI WIRELESS")
	if !ok || fi.Category != CatPhone {
		t.Errorf("GOOGLE FI = %+v ok=%v, want phone", fi, ok)
	}
}

// "FPL" and "HOA" are short keys, which is exactly how a dictionary starts
// filing unrelated merchants. Token-boundary matching is the guard; these pin it,
// because a false positive here silently miscategorizes real spending.
func TestShortUtilityKeysDoNotMatchInsideWords(t *testing.T) {
	for _, descriptor := range []string{
		"FPLUS FITNESS",
		"FPLANNER SOFTWARE",
		"WHOA COFFEE",
		"SHOAL CREEK GRILL",
	} {
		if got, ok := Lookup(descriptor); ok && got.Category == CatUtilities {
			t.Errorf("Lookup(%q) = %q (%s) — a short utility key matched inside a word", descriptor, got.Category, got.Merchant)
		}
	}
}
