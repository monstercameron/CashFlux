// SPDX-License-Identifier: MIT

package smarttext_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/smarttext"
)

// ── CleanMerchant ────────────────────────────────────────────────────────────

func TestCleanMerchant(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Empty / whitespace-only.
		{"", ""},
		{"  ", ""},

		// POS prefix stripping.
		{"SQ *BLUE BOTTLE COFFEE #1234", "Blue Bottle Coffee"},
		{"SQ* BLUE BOTTLE COFFEE #1234", "Blue Bottle Coffee"},
		{"TST*CHIPOTLE", "Chipotle"},
		{"PP*PAYPAL PAYMENT", "Paypal Payment"},
		{"POS WALMART SUPERCENTER", "Walmart Supercenter"},
		{"DEBIT AMAZON.COM", "Amazon.com"},
		{"ACH VENMO PAYMENT", "Venmo Payment"},

		// Trailing reference/store-number strip.
		{"STARBUCKS #4521", "Starbucks"},
		{"TARGET 00012345", "Target"}, // 8-digit ref number
		{"HOME DEPOT #987", "Home Depot"},

		// Keeps short ALL-CAPS acronyms.
		{"ATM WITHDRAWAL", "ATM Withdrawal"},
		{"BP GAS STATION", "BP Gas Station"},
		{"SQ *BP #456", "BP"},

		// Already clean — returns trimmed input unchanged.
		{"Starbucks", "Starbucks"},
		{"Blue Bottle Coffee", "Blue Bottle Coffee"},

		// All-uppercase non-acronym merchant — should be title-cased.
		{"WHOLE FOODS MARKET", "Whole Foods Market"},

		// No prefix, but has trailing code.
		{"COSTCO WHOLESALE #00012", "Costco Wholesale"},

		// Mixed spacing.
		{"  SQ *  THE COFFEE SHOP  ", "The Coffee Shop"},
	}

	for _, tc := range cases {
		got := smarttext.CleanMerchant(tc.in)
		if got != tc.want {
			t.Errorf("CleanMerchant(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// ── ParseWish ────────────────────────────────────────────────────────────────

func TestParseWish(t *testing.T) {
	cases := []struct {
		in        string
		wantName  string
		wantMinor int64
		wantOK    bool
	}{
		// Empty / no amount → not ok.
		{"", "", 0, false},
		{"just a thought", "", 0, false},
		{"save for a laptop", "", 0, false}, // amount-less

		// Basic "save X for Y" patterns.
		{"save $2,000 for a new laptop", "New Laptop", 200000, true},
		{"save $500 for vacation", "Vacation", 50000, true},
		{"save 1000 for a rainy day fund", "Rainy Day Fund", 100000, true},

		// Amount before name.
		{"$500 vacation fund", "Vacation Fund", 50000, true},
		{"2000 emergency fund", "Emergency Fund", 200000, true},

		// Name before amount.
		{"laptop 2000", "Laptop", 200000, true},
		{"new car 15000", "New Car", 1500000, true},

		// "toward" keyword.
		{"save $3,500 toward a new car", "New Car", 350000, true},
		{"$750 toward vacation", "Vacation", 75000, true},

		// Comma-formatted amounts. "PC" is a 2-char acronym → stays all-caps.
		{"save $1,500 for a gaming PC", "Gaming PC", 150000, true},

		// Decimal amounts.
		{"save $99.99 for a book", "Book", 9999, true},

		// "I want to" prefix.
		{"i want to save $200 for shoes", "Shoes", 20000, true},

		// Capitalisation of result.
		{"save $100 for a NEW LAPTOP", "New Laptop", 10000, true},

		// Zero amount → not ok.
		{"save $0 for a car", "", 0, false},
	}

	for _, tc := range cases {
		name, minor, ok := smarttext.ParseWish(tc.in)
		if ok != tc.wantOK {
			t.Errorf("ParseWish(%q) ok=%v; want %v", tc.in, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if name != tc.wantName {
			t.Errorf("ParseWish(%q) name=%q; want %q", tc.in, name, tc.wantName)
		}
		if minor != tc.wantMinor {
			t.Errorf("ParseWish(%q) minor=%d; want %d", tc.in, minor, tc.wantMinor)
		}
	}
}
