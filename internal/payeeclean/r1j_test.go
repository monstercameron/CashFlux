// SPDX-License-Identifier: MIT

package payeeclean

import "testing"

func TestSuggestRemovesCommonBankDescriptorNoise(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "Apple Pay all caps", raw: "APLPAY GREENFIELD MKT", want: "Greenfield Mkt"},
		{name: "Apple Pay mixed case", raw: "APLPAY Greenfield Mkt", want: "Greenfield Mkt"},
		{name: "Apple Pay spaced", raw: "APPLE PAY BLUE BOTTLE COFFEE", want: "Blue Bottle Coffee"},
		{name: "Square star", raw: "SQ *BLUE BOTTLE COFFEE", want: "Blue Bottle Coffee"},
		{name: "Square compact star", raw: "SQ*BLUE BOTTLE COFFEE", want: "Blue Bottle Coffee"},
		{name: "Toast", raw: "TST*COFFEE BAR", want: "Coffee Bar"},
		{name: "Toast separator", raw: "TST* THE COFFEE BAR #123", want: "The Coffee Bar"},
		{name: "PayPal star", raw: "PAYPAL *SPOTIFY", want: "Spotify"},
		{name: "PayPal short code", raw: "PP*NETFLIX", want: "Netflix"},
		{name: "Check card", raw: "CHECKCARD STARBUCKS", want: "Starbucks"},
		{name: "Check card store number", raw: "CHECKCARD STARBUCKS #204", want: "Starbucks"},
		{name: "ACH web", raw: "ACH WEB GREENFIELD MARKET", want: "Greenfield Market"},
		{name: "stacked ACH and PayPal", raw: "ACH WEB PAYPAL *MUSIC STREAM", want: "Music Stream"},
		{name: "ACH", raw: "ACH VENMO PAYMENT", want: "Venmo Payment"},
		{name: "point of sale", raw: "POS WALMART SUPERCENTER", want: "Walmart Supercenter"},
		{name: "point of sale store number", raw: "POS WALMART SUPERCENTER #204", want: "Walmart Supercenter"},
		{name: "debit", raw: "DEBIT TARGET", want: "Target"},
		{name: "debit card", raw: "DEBIT CARD WHOLE FOODS MARKET", want: "Whole Foods Market"},
		{name: "store number word", raw: "STARBUCKS STORE 204", want: "Starbucks"},
		{name: "store number with hash", raw: "STARBUCKS STORE #204", want: "Starbucks"},
		{name: "number marker", raw: "TARGET #204", want: "Target"},
		{name: "trailing reference", raw: "COSTCO WHOLESALE 004821", want: "Costco Wholesale"},
		{name: "masked card tail", raw: "SQ *BLUE BOTTLE x1234", want: "Blue Bottle"},
		{name: "city and state", raw: "COSTCO WHSE PORTLAND OR", want: "Costco Whse"},
		{name: "city and state after store number", raw: "SQ *BLUE BOTTLE #204 PORTLAND OR", want: "Blue Bottle"},
		{name: "city state and reference", raw: "POS TARGET PORTLAND OR 12345", want: "Target"},
		{name: "do not strip a bare merchant", raw: "PayPal", want: "PayPal"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Suggest(tc.raw); got != tc.want {
				t.Errorf("Suggest(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}
