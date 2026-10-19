// SPDX-License-Identifier: MIT

package smartcatview

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestDraftEmpty(t *testing.T) {
	for _, tc := range []struct {
		fill  DraftFill
		empty bool
	}{
		{DraftFill{}, true}, {DraftFill{Income: true}, true}, {DraftFill{AmountMajor: "0"}, false},
		{DraftFill{Payee: "Store"}, false}, {DraftFill{CategoryID: "food"}, false}, {DraftFill{Date: time.Unix(0, 0)}, false},
	} {
		if got := tc.fill.Empty(); got != tc.empty {
			t.Fatal(tc.fill, got)
		}
	}
}

func TestParseSentence(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 30, 0, 0, time.FixedZone("west", -8*3600))
	for _, tc := range []struct {
		raw, amount, payee string
		income             bool
		date               string
	}{
		{"", "", "", false, ""}, {"   ", "", "", false, ""},
		{"spent 40 at whole foods", "40", "Whole Foods", false, ""},
		{"spent 40 at whole foods yesterday", "40", "Whole Foods", false, "2026-02-28"},
		{"tomorrow", "", "", false, "2026-03-02"},
		{"received 20 from ACME", "20", "Received From Acme", true, ""},
		{"40 at Refund Center", "40", "Refund Center", false, ""},
		{"nonsense", "", "", false, ""},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got := ParseSentence(tc.raw, now)
			if got.AmountMajor != tc.amount || got.Payee != tc.payee || got.Income != tc.income {
				t.Fatalf("%+v want %+v", got, tc)
			}
			date := ""
			if !got.Date.IsZero() {
				date = got.Date.Format("2006-01-02")
			}
			if date != tc.date {
				t.Fatalf("date %q != %q", date, tc.date)
			}
		})
	}
	for _, amount := range []string{"0", "-1", "9223372036854775808", "999999999999999999999999999"} {
		got := ParseSentence(amount+" at Store", now)
		if got.AmountMajor != amount || got.Payee != "Store" || got.Income {
			t.Fatalf("amount %q: %+v", amount, got)
		}
	}
}

func TestIncomeBoundariesAndLabels(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{"refund", true}, {"refunded", false}, {"refund_40", false}, {"refund40", false},
		{" Refund 40 ", true}, {"received: 40", true}, {"received\u00e9", false}, {"40 refund", false},
	} {
		if got := txnSentenceIsIncome(tc.raw); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	for _, tc := range []struct{ in, want string }{
		{"spent at whole foods for", "whole foods"}, {"the home depot", "home depot"}, {"", ""}, {"at for", ""},
	} {
		if got := cleanDraftLabel(tc.in); got != tc.want {
			t.Fatal(tc, got)
		}
	}
	for _, tc := range []struct{ in, want string }{
		{"whole FOODS", "Whole Foods"}, {"\u00e9PICERIE DU COIN", "\u00c9picerie Du Coin"}, {"", ""}, {"  two   words ", "Two Words"},
	} {
		if got := titleDraftLabel(tc.in); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}

func TestDraftSummary(t *testing.T) {
	translate := func(key string, args ...any) string { return fmt.Sprint(key, ":", args[0]) }
	date := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		f      DraftFill
		format func(time.Time) string
		want   string
	}{
		{DraftFill{}, nil, ""},
		{DraftFill{Payee: "Store", AmountMajor: "40", Date: date}, func(d time.Time) string { return d.Format("2006-01-02") }, "Store \u00b7 sm15.amountOut:40 \u00b7 2026-01-31"},
		{DraftFill{AmountMajor: "0", Income: true}, nil, "sm15.amountIn:0"},
		{DraftFill{Date: date}, nil, ""},
	} {
		if got := DraftSummary(tc.f, tc.format, translate); got != tc.want {
			t.Fatalf("%q != %q", got, tc.want)
		}
	}
}

func TestParseAIDraft(t *testing.T) {
	now := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		text string
		ok   bool
	}{
		{"not a draft", false}, {"", false},
		{"2026-01-31 | Store | -40 | Food", true},
	} {
		got, ok := ParseAIDraft(tc.text, Catalog(fixtureCats()), now)
		if ok != tc.ok {
			t.Fatalf("%q: %+v %v", tc.text, got, ok)
		}
	}
}

func TestPresentDraft(t *testing.T) {
	local := DraftFill{AmountMajor: "40"}
	ai := DraftFill{Payee: "AI"}
	enabled := smart.Settings{Enabled: map[string]bool{DraftAICode: true}}
	for _, tc := range []struct {
		name                 string
		local, ai            DraftFill
		provider, loading    bool
		settings             smart.Settings
		visible, ask, fromAI bool
	}{
		{"free", local, DraftFill{}, false, false, smart.Settings{}, true, false, false},
		{"AI", local, DraftFill{}, true, false, enabled, true, true, false},
		{"pending", local, DraftFill{}, true, true, enabled, true, true, false},
		{"AI offer", local, ai, true, false, enabled, true, false, true},
		{"muted", local, DraftFill{}, true, false, smart.Settings{Muted: map[string]bool{DraftFreeCode: true, DraftAICode: true}}, false, false, false},
		{"off", local, DraftFill{}, true, false, smart.Settings{ExplicitOff: map[string]bool{DraftFreeCode: true, DraftAICode: true}}, false, false, false},
		{"quiet", local, DraftFill{}, true, false, smart.Settings{Density: smart.DensityMinimal}, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PresentDraft("  sentence ", tc.local, tc.ai, tc.provider, tc.loading, tc.settings)
			if got.Raw != "sentence" || got.Visible != tc.visible || got.ShowAsk != tc.ask || got.FromAI != tc.fromAI {
				t.Fatal(got)
			}
			offer := tc.local
			code, detail := "SMART-T22F", "sm15.readLocal"
			if tc.fromAI {
				offer = tc.ai
				code = "SMART-T22"
				detail = "catSuggest.aiSource"
			}
			ask := "sm15.aiAction"
			if tc.loading {
				ask = "sm15.aiPending"
			}
			if !reflect.DeepEqual(got.Offer, offer) || got.Code != code || got.DetailKey != detail || got.AskKey != ask {
				t.Fatal(got)
			}
		})
	}
	if PresentDraft(" ", local, DraftFill{}, true, false, enabled).ShowAsk {
		t.Fatal("blank sentence offered AI")
	}
}

func FuzzSentenceDeterministic(f *testing.F) {
	f.Add("spent 40 at Store yesterday")
	f.Add("refund 0")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 10000 {
			t.Skip()
		}
		now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
		a, b := ParseSentence(raw, now), ParseSentence(strings.TrimSpace(raw), now)
		if !reflect.DeepEqual(a, b) {
			t.Fatal("whitespace or clock changed result")
		}
	})
}

func TestSentenceDatesKeepLocalCalendar(t *testing.T) {
	for _, tc := range []struct {
		now        time.Time
		word, want string
	}{
		{time.Date(2026, 1, 31, 23, 30, 0, 0, time.FixedZone("east", 14*3600)), "tomorrow", "2026-02-01"},
		{time.Date(2024, 3, 1, 0, 30, 0, 0, time.FixedZone("west", -12*3600)), "yesterday", "2024-02-29"},
	} {
		got := ParseSentence("spent 1 at Store "+tc.word, tc.now)
		if got.Date.Format("2006-01-02") != tc.want || got.Date.Location() != tc.now.Location() || got.Date.Hour() != 0 {
			t.Fatal(got)
		}
	}
}

func TestAIDraftFallbackAndUnknownCategory(t *testing.T) {
	now := time.Date(2026, 1, 31, 23, 30, 0, 0, time.FixedZone("east", 14*3600))
	got, ok := ParseAIDraft("  unknown | Store | +40 | imaginary  ", Catalog(fixtureCats()), now)
	if !ok || got.CategoryID != "" || got.Payee != "Store" || got.AmountMajor != "40" || !got.Income || !got.Date.Equal(now) {
		t.Fatal(got, ok)
	}
}
