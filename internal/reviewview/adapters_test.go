// SPDX-License-Identifier: MIT

package reviewview

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

func TestSessionTransitions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input Session
		want  Session
	}{
		{"zero", Session{}, Session{Mode: "bulk", Scope: reviewscope.Normalize(""), Commit: reviewscope.CommitCharge}},
		{"invalid", Session{Mode: "other", Scope: "other", Commit: "other", Cursor: -9}, Session{Mode: "bulk", Scope: reviewscope.Normalize(""), Commit: reviewscope.CommitCharge}},
		{"valid", Session{Mode: "single", Scope: reviewscope.ScopeCurrentView, Commit: reviewscope.CommitMerchant, Cursor: 4, CursorKey: "a", CommitKey: "a", ScopeChosen: true}, Session{Mode: "single", Scope: reviewscope.ScopeCurrentView, Commit: reviewscope.CommitMerchant, Cursor: 4, CursorKey: "a", CommitKey: "a", ScopeChosen: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.input.Normalized(); got != tc.want || got.Normalized() != got {
				t.Fatalf("normalized=%+v want=%+v", got, tc.want)
			}
		})
	}
	seed := Session{Mode: "single", Cursor: 4, CursorKey: "a", CommitKey: "a", Commit: reviewscope.CommitMerchant}
	scope := seed.WithScope(string(reviewscope.ScopeCurrentView))
	if scope.Cursor != 0 || scope.CursorKey != "" || !scope.ScopeChosen || scope.Scope != reviewscope.ScopeCurrentView || scope.CommitKey != "a" {
		t.Fatalf("scope=%+v", scope)
	}
	if mode := seed.WithMode("bulk"); mode.Cursor != 4 || mode.CursorKey != "a" || mode.Mode != "bulk" {
		t.Fatalf("mode=%+v", mode)
	}
	if mode := seed.WithMode("invalid"); mode.Mode != "bulk" {
		t.Fatal("invalid mode")
	}
	if cursor := seed.WithCursor(-1, "b"); cursor.Cursor != 0 || cursor.CursorKey != "b" || cursor.CommitKey != "a" {
		t.Fatalf("cursor=%+v", cursor)
	}
	if commit := seed.WithCommit("merchant", "b"); commit.Commit != reviewscope.CommitMerchant || commit.CommitKey != "b" {
		t.Fatalf("commit=%+v", commit)
	}
	if commit := seed.WithCommit("invalid", "b"); commit.Commit != reviewscope.CommitCharge {
		t.Fatal("invalid commit scope")
	}
}

func TestMemoUsesOwnerAndRevision(t *testing.T) {
	var memo Memo[string, []int]
	calls := 0
	load := func() []int { calls++; return []int{calls} }
	for _, tc := range []struct {
		owner string
		rev   uint64
		calls int
	}{{"a", 0, 1}, {"a", 0, 1}, {"a", 1, 2}, {"b", 1, 3}, {"b", 1, 3}, {"a", 0, 4}} {
		got := memo.Get(tc.owner, tc.rev, load)
		if calls != tc.calls || got[0] != calls {
			t.Fatalf("memo=%v calls=%d want=%d", got, calls, tc.calls)
		}
	}
	var empty Memo[int, []int]
	loads := 0
	loadEmpty := func() []int { loads++; return nil }
	empty.Get(0, 0, loadEmpty)
	empty.Get(0, 0, loadEmpty)
	if loads != 1 {
		t.Fatal("nil result reloaded")
	}
}

func TestMemoSeesAppendAtSameSliceBaseOnNewRevision(t *testing.T) {
	buffer := make([]int, 1, 8)
	buffer[0] = 10
	var memo Memo[string, []int]
	first := memo.Get("owner", 1, func() []int { return buffer })
	buffer = append(buffer, 20)
	second := memo.Get("owner", 2, func() []int { return buffer })
	if &first[0] != &second[0] || len(first) != 1 || len(second) != 2 {
		t.Fatal("revision failed to invalidate a reused slice base")
	}
}

func TestDismissalCodec(t *testing.T) {
	for _, raw := range []string{"", "null", "[]", "broken", `{"a":"wrong"}`} {
		if got := DecodeDismissals(raw); got == nil || len(got) != 0 {
			t.Fatalf("decode %q = %v", raw, got)
		}
	}
	for _, tc := range []map[string]bool{{}, {"a": true, "b": false}, {"quote\"\\\n": true}} {
		if got := DecodeDismissals(EncodeDismissals(tc)); !reflect.DeepEqual(tc, got) {
			t.Fatalf("round-trip=%v want=%v", got, tc)
		}
	}
	if EncodeDismissals(nil) != "null" {
		t.Fatal("nil codec changed")
	}
}

func FuzzDismissalCodecRoundTrip(f *testing.F) {
	f.Add("a", true)
	f.Add("quote\"\\\n", false)
	f.Fuzz(func(t *testing.T, key string, value bool) {
		// JSON replaces invalid UTF-8; normalized strings have a lossless round-trip.
		key = strings.ToValidUTF8(key, "�")
		seed := map[string]bool{key: value}
		if got := DecodeDismissals(EncodeDismissals(seed)); !reflect.DeepEqual(seed, got) {
			t.Fatalf("round-trip=%v want=%v", got, seed)
		}
	})
}

func TestLoanPaymentValidation(t *testing.T) {
	suggestion := txnclassify.PaymentSuggestion{TransactionID: "a", AccountID: "loan"}
	tx := txn("a", "car payment", -100)
	accounts := []domain.Account{{ID: "checking", Class: domain.ClassAsset, Currency: "USD"}, {ID: "loan", Class: domain.ClassLiability, Currency: "USD"}}
	for _, tc := range []struct {
		name     string
		current  []txnclassify.PaymentSuggestion
		found    bool
		accounts []domain.Account
		wantErr  string
	}{
		{"stale", nil, true, accounts, "no longer available"},
		{"wrong account", []txnclassify.PaymentSuggestion{{TransactionID: "a", AccountID: "other"}}, true, accounts, "no longer available"},
		{"missing transaction", []txnclassify.PaymentSuggestion{suggestion}, false, accounts, `transaction "a" was not found`},
		{"missing account", []txnclassify.PaymentSuggestion{suggestion}, true, nil, "classify loan payment"},
		{"valid", []txnclassify.PaymentSuggestion{suggestion}, true, accounts, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := LoanPayment(suggestion, tc.current, tx, tc.found, tc.accounts)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err=%v want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got.ID != tx.ID || got.TransferAccountID != "loan" || got.BillAccountID != "loan" || tx.IsTransfer() {
				t.Fatalf("draft=%+v err=%v", got, err)
			}
		})
	}
}

func TestClickBoundary(t *testing.T) {
	for _, tc := range []struct {
		button                                int
		numeric, meta, ctrl, shift, alt, want bool
	}{
		{0, true, false, false, false, false, true}, {9, false, false, false, false, false, true},
		{1, true, false, false, false, false, false}, {2, true, false, false, false, false, false},
		{0, true, true, false, false, false, false}, {0, true, false, true, false, false, false},
		{0, true, false, false, true, false, false}, {0, true, false, false, false, true, false},
	} {
		if got := PlainLeftClick(tc.button, tc.numeric, tc.meta, tc.ctrl, tc.shift, tc.alt); got != tc.want {
			t.Fatalf("click %+v = %v", tc, got)
		}
	}
}

func TestSnoozeCalendarDates(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		now  time.Time
		want string
	}{
		{time.Date(2026, 1, 31, 23, 45, 0, 0, time.UTC), "2026-02-07T23:45:00Z"},
		{time.Date(2026, 10, 31, 10, 0, 0, 0, newYork), "2026-11-07T10:00:00-05:00"},
		{time.Date(2028, 2, 25, 8, 0, 0, 0, time.UTC), "2028-03-03T08:00:00Z"},
	} {
		got := SnoozeUntil(tc.now)
		if got.Format(time.RFC3339) != tc.want || got.Location() != tc.now.Location() {
			t.Fatalf("snooze=%v want=%s", got, tc.want)
		}
	}
}
