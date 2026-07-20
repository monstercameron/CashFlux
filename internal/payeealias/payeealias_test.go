// SPDX-License-Identifier: MIT

package payeealias

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"amazon marketplace hash", "AMZN Mktp US*2K4RT0", "Amazon"},
		{"amzn bare", "AMZN", "Amazon"},
		{"amazon prime", "AMAZON PRIME*4X12", "Amazon Prime"},
		{"square", "SQ *BLUE BOTTLE", "Blue Bottle"},
		{"toast", "TST* JOES PIZZA", "Joes Pizza"},
		{"clover", "CKE*THE CORNER CAFE", "The Corner Cafe"},
		{"paypal star", "PAYPAL *STEAMGAMES", "Steamgames"},
		{"sp merchant", "SP MERCHANT CO", "Merchant Co"},
		{"apple pay", "APLPAY TARGET 00123", "Target"},
		{"venmo payment", "VENMO PAYMENT 190X", "Venmo"},
		{"trailing store number", "STARBUCKS 08842", "Starbucks"},
		{"trailing hash only", "SHELL OIL #4471", "Shell Oil"},
		{"no noise passthrough", "Corner Grocery", "Corner Grocery"},
		{"acronym preserved", "AMC LLC 0091", "AMC LLC"},
		{"empty", "   ", ""},
		{"whitespace collapse", "BIG   MART   0012", "Big Mart"},
		// Descriptor noise the recurring-discovery review strip was showing raw.
		{"support phone stripped", "MSFT * XBOX GAME PASS 425-6816830", "Xbox Game Pass"},
		{"msft star no space", "MSFT*XBOX GAME PASS", "Xbox Game Pass"},
		{"doordash platform", "DD DOORDASH WINGSTOP 855-431-0459", "DoorDash"},
		{"doordash star", "DD *DOORDASH CHIPOTLE", "DoorDash"},
		{"doordash bare", "DOORDASH*MCDONALDS", "DoorDash"},
		{"uber eats", "UBER EATS PENDING 8005928996", "Uber Eats"},
		{"grubhub", "GRUBHUB*JOES", "Grubhub"},
		{"multiple trailing refs", "CORNER DELI 4471 00982", "Corner Deli"},
		{"all reference keeps a token", "998812", "998812"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.raw); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestResolver(t *testing.T) {
	aliases := []domain.PayeeAlias{
		{ID: "1", RawPayee: "AMZN Mktp US*2K4RT0", Display: "Amazon (books)"},
		{ID: "2", RawPayee: "WEIRD RAW NAME", Display: "My Barber"},
	}
	r := NewResolver(aliases)

	// Learned alias wins over the rule pack (exact, case-insensitive).
	if got := r.Resolve("amzn mktp us*2k4rt0"); got != "Amazon (books)" {
		t.Errorf("learned alias case-insensitive = %q", got)
	}
	// Rule pack applies when no learned alias.
	if got := r.Resolve("SQ *BLUE BOTTLE"); got != "Blue Bottle" {
		t.Errorf("rule pack fallback = %q", got)
	}
	// Learned alias for a name the rule pack would not clean.
	if got := r.Resolve("WEIRD RAW NAME"); got != "My Barber" {
		t.Errorf("learned non-rulepack = %q", got)
	}
	// Empty stays empty.
	if got := r.Resolve("  "); got != "" {
		t.Errorf("empty resolve = %q", got)
	}
	if !r.HasLearned("weird raw name") {
		t.Error("HasLearned should be true for learned raw")
	}
	if r.HasLearned("SQ *BLUE BOTTLE") {
		t.Error("HasLearned should be false for rule-pack-only")
	}
}

func TestNewResolverSkipsBlank(t *testing.T) {
	r := NewResolver([]domain.PayeeAlias{
		{ID: "1", RawPayee: "  ", Display: "X"},
		{ID: "2", RawPayee: "Y", Display: "  "},
	})
	if r.HasLearned("") || r.HasLearned("Y") {
		t.Error("blank alias entries should be skipped")
	}
}

func TestNilResolver(t *testing.T) {
	var r *Resolver
	if got := r.Resolve("SQ *BLUE BOTTLE"); got != "Blue Bottle" {
		t.Errorf("nil resolver should still normalize: %q", got)
	}
	if r.HasLearned("x") {
		t.Error("nil resolver HasLearned should be false")
	}
}
