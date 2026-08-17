// SPDX-License-Identifier: MIT

package smartai

import (
	"strings"
	"testing"
	"time"
)

// smCatalog is the household's category list for the parser tables: two paths
// that share a leaf name, so a parser that resolves bare leaves would be caught.
func smCatalog() Catalog {
	return NewCatalog([]Cat{
		{ID: "gro", Path: "Groceries"},
		{ID: "hh", Path: "Household"},
		{ID: "auto-gas", Path: "Auto > Gas"},
		{ID: "util-gas", Path: "Utilities > Gas"},
		{ID: "salary", Path: "Salary", Income: true},
	})
}

func TestParseSplitShares(t *testing.T) {
	cat := smCatalog()
	tests := []struct {
		name   string
		answer string
		want   []SplitShare
	}{
		{
			name:   "two clean lines",
			answer: "Groceries | 70\nHousehold | 30",
			want: []SplitShare{
				{CategoryID: "gro", CategoryName: "Groceries", Percent: 70},
				{CategoryID: "hh", CategoryName: "Household", Percent: 30},
			},
		},
		{
			name:   "percent signs and bullets are tolerated",
			answer: "- Groceries | 60%\n- Household | 40%",
			want: []SplitShare{
				{CategoryID: "gro", CategoryName: "Groceries", Percent: 60},
				{CategoryID: "hh", CategoryName: "Household", Percent: 40},
			},
		},
		{
			name:   "a qualified path resolves to the right one of two same-named leaves",
			answer: "Auto > Gas | 80\nGroceries | 20",
			want: []SplitShare{
				{CategoryID: "auto-gas", CategoryName: "Auto > Gas", Percent: 80},
				{CategoryID: "gro", CategoryName: "Groceries", Percent: 20},
			},
		},
		{
			name:   "an invented category is dropped, and one survivor is not a split",
			answer: "Groceries | 70\nSnacks | 30",
			want:   nil,
		},
		{
			name:   "a bare ambiguous leaf cannot resolve",
			answer: "Gas | 70\nGroceries | 30",
			want:   nil,
		},
		{
			name:   "a single-category answer is not a split",
			answer: "Groceries | 100",
			want:   nil,
		},
		{
			name:   "duplicates collapse, leaving too few lines",
			answer: "Groceries | 50\nGROCERIES | 50",
			want:   nil,
		},
		{
			name:   "out-of-range percentages are dropped",
			answer: "Groceries | 0\nHousehold | 140\nAuto > Gas | 30\nSalary | 70",
			want: []SplitShare{
				{CategoryID: "auto-gas", CategoryName: "Auto > Gas", Percent: 30},
				{CategoryID: "salary", CategoryName: "Salary", Percent: 70},
			},
		},
		{name: "empty answer", answer: "", want: nil},
		{name: "prose instead of the format", answer: "I think this is mostly groceries.", want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseSplitShares(tc.answer, cat)
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("line %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParseSplitShares_CapsAtSix(t *testing.T) {
	cat := NewCatalog([]Cat{
		{ID: "a", Path: "A"}, {ID: "b", Path: "B"}, {ID: "c", Path: "C"},
		{ID: "d", Path: "D"}, {ID: "e", Path: "E"}, {ID: "f", Path: "F"}, {ID: "g", Path: "G"},
	})
	answer := "A | 10\nB | 10\nC | 10\nD | 10\nE | 10\nF | 10\nG | 40"
	if got := ParseSplitShares(answer, cat); len(got) != 6 {
		t.Errorf("got %d lines, want 6", len(got))
	}
}

func TestParseTaskDraft(t *testing.T) {
	tests := []struct {
		name       string
		answer     string
		wantOK     bool
		wantTitle  string
		wantDue    string // "" = zero
		wantRepeat string
	}{
		{
			name: "full line", answer: "Pay the rent | 2026-08-14 | monthly",
			wantOK: true, wantTitle: "Pay the rent", wantDue: "2026-08-14", wantRepeat: "monthly",
		},
		{
			name: "no date, no repeat", answer: "Call the bank | none | none",
			wantOK: true, wantTitle: "Call the bank",
		},
		{
			name: "an invented repeat degrades to a one-shot", answer: "Do it | 2026-08-14 | whenever",
			wantOK: true, wantTitle: "Do it", wantDue: "2026-08-14",
		},
		{
			name: "an unreadable date leaves it unset rather than guessed", answer: "Do it | next friday | weekly",
			wantOK: true, wantTitle: "Do it", wantRepeat: "weekly",
		},
		{
			name: "quotes and bullets are stripped", answer: "- \"Pay the rent\" | 2026-08-14 | none",
			wantOK: true, wantTitle: "Pay the rent", wantDue: "2026-08-14",
		},
		{
			name: "title only", answer: "Review subscriptions",
			wantOK: true, wantTitle: "Review subscriptions",
		},
		{name: "empty", answer: "", wantOK: false},
		{name: "no title", answer: " | 2026-08-14 | monthly", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseTaskDraft(tc.answer)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			gotDue := ""
			if !got.Due.IsZero() {
				gotDue = got.Due.Format("2006-01-02")
			}
			if gotDue != tc.wantDue {
				t.Errorf("Due = %q, want %q", gotDue, tc.wantDue)
			}
			if got.Repeat != tc.wantRepeat {
				t.Errorf("Repeat = %q, want %q", got.Repeat, tc.wantRepeat)
			}
		})
	}
}

func TestParseTxnDraft(t *testing.T) {
	cat := smCatalog()
	fallback := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		answer   string
		wantOK   bool
		wantDate string
		wantPay  string
		wantAmt  string
		wantNeg  bool
		wantCat  string
	}{
		{
			name: "a clean spend", answer: "2026-08-11 | Whole Foods | -40.00 | Groceries",
			wantOK: true, wantDate: "2026-08-11", wantPay: "Whole Foods", wantAmt: "40.00", wantNeg: true, wantCat: "gro",
		},
		{
			name: "income keeps its positive sign", answer: "2026-08-11 | Acme Corp | 4700 | Salary",
			wantOK: true, wantDate: "2026-08-11", wantPay: "Acme Corp", wantAmt: "4700", wantNeg: false, wantCat: "salary",
		},
		{
			name: "a currency symbol and separators are tolerated", answer: "2026-08-11 | Costco | -$1,240.67 | Household",
			wantOK: true, wantDate: "2026-08-11", wantPay: "Costco", wantAmt: "1240.67", wantNeg: true, wantCat: "hh",
		},
		{
			name: "parentheses mean negative", answer: "2026-08-11 | Costco | (35.10) | Household",
			wantOK: true, wantDate: "2026-08-11", wantPay: "Costco", wantAmt: "35.10", wantNeg: true, wantCat: "hh",
		},
		{
			name: "a declined category leaves the field empty", answer: "2026-08-11 | Some Shop | -12 | none",
			wantOK: true, wantDate: "2026-08-11", wantPay: "Some Shop", wantAmt: "12", wantNeg: true, wantCat: "",
		},
		{
			name: "an invented category is dropped, not guessed", answer: "2026-08-11 | Some Shop | -12 | Snacks",
			wantOK: true, wantDate: "2026-08-11", wantPay: "Some Shop", wantAmt: "12", wantNeg: true, wantCat: "",
		},
		{
			name: "an unreadable date falls back to today", answer: "yesterday | Whole Foods | -40 | Groceries",
			wantOK: true, wantDate: "2026-08-12", wantPay: "Whole Foods", wantAmt: "40", wantNeg: true, wantCat: "gro",
		},
		{name: "no amount is not a draft", answer: "2026-08-11 | Whole Foods | none | Groceries", wantOK: false},
		{name: "a zero amount is not a draft", answer: "2026-08-11 | Whole Foods | 0 | Groceries", wantOK: false},
		{name: "no merchant is not a draft", answer: "2026-08-11 |  | -40 | Groceries", wantOK: false},
		{name: "too few fields", answer: "2026-08-11 | Whole Foods", wantOK: false},
		{name: "empty", answer: "", wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseTxnDraft(tc.answer, cat, fallback)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got %+v)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				return
			}
			if d := got.Date.Format("2006-01-02"); d != tc.wantDate {
				t.Errorf("Date = %s, want %s", d, tc.wantDate)
			}
			if got.Payee != tc.wantPay {
				t.Errorf("Payee = %q, want %q", got.Payee, tc.wantPay)
			}
			if got.AmountMajor != tc.wantAmt {
				t.Errorf("AmountMajor = %q, want %q", got.AmountMajor, tc.wantAmt)
			}
			if got.Negative != tc.wantNeg {
				t.Errorf("Negative = %v, want %v", got.Negative, tc.wantNeg)
			}
			if got.CategoryID != tc.wantCat {
				t.Errorf("CategoryID = %q, want %q", got.CategoryID, tc.wantCat)
			}
		})
	}
}

// Every prompt must carry the caller's context verbatim, and the structured ones
// must stamp today's date — the difference between "friday" landing this week and
// landing in whatever week the model believes it is.
func TestSMRequestsCarryContext(t *testing.T) {
	today := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)

	if r := SplitSuggest("Costco $240", "Groceries | expense"); !strings.Contains(r.User, "Costco $240") ||
		!strings.Contains(r.User, "Groceries | expense") {
		t.Errorf("SplitSuggest dropped context: %q", r.User)
	}
	if r := WhyOver("Groceries: limit $500, spent $680"); !strings.Contains(r.User, "spent $680") {
		t.Errorf("WhyOver dropped context: %q", r.User)
	}
	if r := BalanceAnomaly("Checking fell $3,200"); !strings.Contains(r.User, "$3,200") {
		t.Errorf("BalanceAnomaly dropped context: %q", r.User)
	}
	if r := GoalPace("Emergency fund: $4,000 of $9,000"); !strings.Contains(r.User, "$9,000") {
		t.Errorf("GoalPace dropped context: %q", r.User)
	}

	r := ExplainAlert("Budget overspent", "Groceries is $180 over.", "Runway: 22 days")
	for _, want := range []string{"Budget overspent", "Groceries is $180 over.", "Runway: 22 days"} {
		if !strings.Contains(r.User, want) {
			t.Errorf("ExplainAlert dropped %q: %q", want, r.User)
		}
	}
	// An alert with no body or figures must still produce a usable request.
	if r := ExplainAlert("Card due soon", "", ""); !strings.Contains(r.User, "Card due soon") {
		t.Errorf("ExplainAlert dropped a bare title: %q", r.User)
	}

	if r := TaskParseStructured("pay rent the tuesday after payday", today); !strings.Contains(r.User, "2026-08-12") ||
		!strings.Contains(r.User, "Wednesday") || !strings.Contains(r.User, "after payday") {
		t.Errorf("TaskParseStructured is missing today or the sentence: %q", r.User)
	}
	if r := TxnDraftRequest("spent 40 at whole foods yesterday", "Groceries | expense", today); !strings.Contains(r.User, "2026-08-12") ||
		!strings.Contains(r.User, "whole foods") || !strings.Contains(r.User, "Groceries | expense") {
		t.Errorf("TxnDraftRequest is missing today, the sentence, or the catalog: %q", r.User)
	}
}

func TestParseAmountMajor(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantNeg bool
		ok      bool
	}{
		{"-40.00", "40.00", true, true},
		{"40.00", "40.00", false, true},
		{" -$1,240.67 ", "1240.67", true, true},
		{"(35.10)", "35.10", true, true},
		{"4700", "4700", false, true},
		{"12.", "12", false, true},
		{"0", "", false, false},
		{"0.00", "", false, false},
		{"none", "", false, false},
		{"", "", false, false},
	}
	for _, tc := range tests {
		got, neg, ok := parseAmountMajor(tc.in)
		if ok != tc.ok || (ok && (got != tc.want || neg != tc.wantNeg)) {
			t.Errorf("parseAmountMajor(%q) = %q, %v, %v; want %q, %v, %v", tc.in, got, neg, ok, tc.want, tc.wantNeg, tc.ok)
		}
	}
}
