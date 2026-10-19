// SPDX-License-Identifier: MIT

package reviewview

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func txn(id, payee string, amount int64) domain.Transaction {
	return domain.Transaction{ID: id, Payee: payee, AccountID: "checking", Date: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), Amount: money.New(amount, "USD")}
}

func row(key string, tier Tier, items ...domain.Transaction) Row {
	return Row{Group: reviewqueue.Group{Key: key, Merchant: " " + key + " ", Items: items}, Tier: tier}
}

func TestDismissalSnapshots(t *testing.T) {
	suggestions := []txnclassify.PaymentSuggestion{{TransactionID: "a"}, {TransactionID: "b"}}
	for _, tc := range []struct {
		name    string
		in      map[string]bool
		want    map[string]bool
		changed bool
		visible int
	}{
		{"nil", nil, map[string]bool{}, false, 2},
		{"active", map[string]bool{"a": true}, map[string]bool{"a": true}, false, 1},
		{"stale and false", map[string]bool{"gone": true, "a": false, "b": true}, map[string]bool{"b": true}, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := PruneDismissals(tc.in, []domain.Transaction{{ID: "a"}, {ID: "b"}})
			if changed != tc.changed || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("pruned = %v, %v", got, changed)
			}
			if visible := VisibleLoanPayments(suggestions, tc.in); len(visible) != tc.visible {
				t.Fatalf("visible = %v", visible)
			}
			got["sentinel"] = true
			if tc.in["sentinel"] {
				t.Fatal("aliased dismissal map")
			}
		})
	}
	if len(VisibleLoanPayments(nil, nil)) != 0 {
		t.Fatal("nil suggestions")
	}
}

func TestSharedSuggestionAndTiers(t *testing.T) {
	a, b := txn("a", "merchant", -1), txn("b", "merchant", -2)
	high := catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfHigh}
	low := catsuggest.Suggestion{CategoryID: "food", Confidence: catsuggest.ConfLow}
	for _, tc := range []struct {
		name         string
		items        []domain.Transaction
		byID         map[string]catsuggest.Suggestion
		ok, conflict bool
		tier         Tier
	}{
		{"empty", nil, nil, false, false, TierNone},
		{"missing", []domain.Transaction{a}, nil, false, false, TierNone},
		{"empty category", []domain.Transaction{a}, map[string]catsuggest.Suggestion{"a": {}}, false, false, TierNone},
		{"high", []domain.Transaction{a}, map[string]catsuggest.Suggestion{"a": high}, true, false, TierReady},
		{"weakest", []domain.Transaction{a, b}, map[string]catsuggest.Suggestion{"a": high, "b": low}, true, false, TierLook},
		{"stronger second", []domain.Transaction{a, b}, map[string]catsuggest.Suggestion{"a": low, "b": high}, true, false, TierLook},
		{"partial", []domain.Transaction{a, b}, map[string]catsuggest.Suggestion{"a": high}, false, true, TierNone},
		{"conflict", []domain.Transaction{a, b}, map[string]catsuggest.Suggestion{"a": high, "b": {CategoryID: "rent"}}, false, true, TierNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, ok, conflict := SharedSuggestion(reviewqueue.Group{Items: tc.items}, tc.byID)
			if ok != tc.ok || conflict != tc.conflict || TierOf(s, ok) != tc.tier {
				t.Fatalf("got %+v, %v, %v", s, ok, conflict)
			}
		})
	}
	if TierOf(catsuggest.Suggestion{}, true) != TierNone {
		t.Fatal("empty suggestion has tier")
	}
}

func TestIndexScopeAndLookups(t *testing.T) {
	a, b, c := txn("a", "AMZN MKTP US*1", -100), txn("b", "AMZN MKTP US*1", -100), txn("c", "SHELL OIL", -50)
	d := txn("d", "dismissed", -200)
	res := reviewqueue.Resolutions{}
	res.Dismiss("d")
	for _, tc := range []struct {
		name      string
		scope     reviewscope.Scope
		ledger    txnfilter.Criteria
		fallback  bool
		want      int
		wantScope reviewscope.Scope
	}{
		{"all", reviewscope.ScopeEntireQueue, txnfilter.Criteria{Text: "AMZN"}, false, 3, reviewscope.ScopeEntireQueue},
		{"view", reviewscope.ScopeCurrentView, txnfilter.Criteria{Text: "AMZN", Page: 10, PageSize: 1}, false, 2, reviewscope.ScopeCurrentView},
		{"empty chosen", reviewscope.ScopeCurrentView, txnfilter.Criteria{Text: "nothing"}, false, 0, reviewscope.ScopeCurrentView},
		{"empty default", reviewscope.ScopeCurrentView, txnfilter.Criteria{Text: "nothing"}, true, 3, reviewscope.ScopeEntireQueue},
		{"normalize", "invalid", txnfilter.Criteria{}, false, 3, reviewscope.Normalize("invalid")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := BuildIndex(Input{Transactions: []domain.Transaction{a, b, c, d}, Resolutions: res, Scope: tc.scope, Ledger: tc.ledger, FallbackEmptyView: tc.fallback, Now: a.Date, Suggestions: map[string]catsuggest.Suggestion{"a": {CategoryID: "food", Confidence: catsuggest.ConfHigh}, "b": {CategoryID: "food", Confidence: catsuggest.ConfHigh}}})
			if idx.Total != tc.want || idx.Scope != tc.wantScope || idx.QueueTotal != 3 {
				t.Fatalf("index = %+v", idx)
			}
			if len(idx.Dupes["a"]) != 2 || idx.TxnByID["d"].ID != "d" {
				t.Fatalf("context lookups = %+v", idx)
			}
			count := 0
			for _, r := range idx.Rows {
				count += len(r.Group.Items)
				if idx.Byid[r.Group.Key].Group.Key != r.Group.Key {
					t.Fatal("missing row lookup")
				}
			}
			if count != idx.Total {
				t.Fatal("groups do not reconcile")
			}
		})
	}
	if idx := BuildIndex(Input{}); idx.Total != 0 || idx.Byid == nil {
		t.Fatalf("nil index = %+v", idx)
	}
	if got := RestrictQueue([]domain.Transaction{c, a, b}, []string{"a", "c", "a", "unknown"}); len(got) != 2 || got[0].ID != "c" || got[1].ID != "a" {
		t.Fatalf("restriction order = %v", got)
	}
	if len(LedgerVisibleIDs(nil, txnfilter.Criteria{})) != 0 || len(RestrictQueue(nil, nil)) != 0 {
		t.Fatal("empty filters")
	}
}

func TestCandidatesAgreeWithIndexPopulation(t *testing.T) {
	items := []domain.Transaction{txn("a", "coffee", -10), txn("b", "other", -20)}
	for _, tc := range []struct {
		name     string
		scope    reviewscope.Scope
		search   string
		fallback bool
		want     int
	}{
		{"queue", reviewscope.ScopeEntireQueue, "coffee", false, 2},
		{"view", reviewscope.ScopeCurrentView, "coffee", false, 1},
		{"chosen empty", reviewscope.ScopeCurrentView, "none", false, 0},
		{"default empty", reviewscope.ScopeCurrentView, "none", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{Transactions: items, Scope: tc.scope, Ledger: txnfilter.Criteria{Text: tc.search}, FallbackEmptyView: tc.fallback}
			candidates, index := Candidates(in), BuildIndex(in)
			if len(candidates) != tc.want || len(candidates) != index.Total {
				t.Fatalf("candidates=%v index=%+v", candidates, index)
			}
		})
	}
	if len(Candidates(Input{})) != 0 {
		t.Fatal("nil candidates")
	}
}

func TestCategoryPrecedence(t *testing.T) {
	r := row("a", TierReady, txn("a1", "a", -1))
	for _, tc := range []struct {
		name            string
		manual, ai      map[string]string
		local, conflict bool
		want            string
	}{
		{"none", nil, nil, false, false, ""},
		{"AI", nil, map[string]string{"a": "ai"}, false, false, "ai"},
		{"conflict blocks AI", nil, map[string]string{"a": "ai"}, false, true, ""},
		{"local", nil, map[string]string{"a": "ai"}, true, false, "local"},
		{"manual", map[string]string{"a": "manual"}, nil, true, false, "manual"},
		{"explicit empty", map[string]string{"a": ""}, map[string]string{"a": "ai"}, true, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r.HasSugg, r.SuggestionConflict = tc.local, tc.conflict
			r.Suggestion.CategoryID = "local"
			if got := Category(r, tc.manual, tc.ai); got != tc.want {
				t.Fatalf("category = %q", got)
			}
		})
	}
}

func TestMerchantName(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{{"", ""}, {" \t\n", ""}, {" Café Market \n", "Café Market"}, {"A  B", "A  B"}} {
		r := Row{Group: reviewqueue.Group{Merchant: tc.raw}}
		if got := MerchantName(r); got != tc.want || r.Group.Merchant != tc.raw {
			t.Fatalf("merchant=%q want=%q", got, tc.want)
		}
	}
}

func TestSelectionAndRuleDrafts(t *testing.T) {
	rows := []Row{row("a", TierReady, txn("a1", "a", -1), txn("a2", "a", -2)), row("b", TierLook, txn("b1", "b", 0)), row("empty", TierNone), row(" ", TierNone, txn("blank", "", 1))}
	manual := map[string]string{"a": "food", "empty": "food", " ": "food"}
	sel := map[string]bool{"old": true, "b": false}
	got := SelectTier(rows, sel, TierReady, manual, nil)
	if !got["a"] || !got["old"] || sel["a"] {
		t.Fatal("select tier lost or mutated selection")
	}
	assign := Assignments(rows, got, manual, nil)
	if !reflect.DeepEqual(assign, map[string]string{"a1": "food", "a2": "food"}) {
		t.Fatalf("assignments = %v", assign)
	}
	got["b"] = true
	got["empty"] = true
	got[" "] = true
	rules := RuleDrafts(rows, got, manual, nil)
	if len(rules) != 1 || rules[0].Match != "a" || rules[0].ID != "" || rules[0].Order != 0 || rules[0].SetCategoryID != "food" {
		t.Fatalf("rules = %v", rules)
	}
	if m, c := SelectionCount(rows, got); m != 4 || c != 4 {
		t.Fatalf("count = %d, %d", m, c)
	}
	if m, c := TierCount(rows, TierReady); m != 1 || c != 2 {
		t.Fatalf("tier count = %d, %d", m, c)
	}
	if ids := ChargeIDs(rows[0]); !reflect.DeepEqual(ids, []string{"a1", "a2"}) {
		t.Fatalf("ids = %v", ids)
	}
	for _, tc := range []struct {
		tier Tier
		want string
	}{{TierNone, "is-none"}, {TierLook, "is-look"}, {TierReady, "is-ready"}, {Tier(99), "is-none"}} {
		if TierClass(tc.tier) != tc.want {
			t.Fatal("tier class")
		}
	}
	for _, focus := range []int{-1, 0, 9} {
		subset, key := TierRows(rows, TierLook, focus)
		if len(subset) != 1 || subset[0].Group.Key != "b" {
			t.Fatal("tier subset")
		}
		if (focus == 0) != (key == "a") {
			t.Fatalf("focus key = %q", key)
		}
	}
	if len(Assignments(nil, nil, nil, nil)) != 0 || len(RuleDrafts(nil, nil, nil, nil)) != 0 {
		t.Fatal("nil selection")
	}
}

func TestImmutableMapsAndAcceptance(t *testing.T) {
	for _, seed := range []map[string]bool{nil, {}, {"a": true, "b": false}} {
		first := Toggle(seed, "a")
		second := Toggle(first, "a")
		if second["a"] != seed["a"] || first["a"] == seed["a"] {
			t.Fatal("toggle invariant")
		}
		dismissed := Dismiss(seed, "new")
		if !dismissed["new"] || seed["new"] {
			t.Fatal("dismiss mutation")
		}
	}
	manual := map[string]string{"a": "hand", "empty": ""}
	chosen := Choose(manual, "new", "food")
	if chosen["new"] != "food" || manual["new"] != "" {
		t.Fatal("choose mutation")
	}
	if Choose(nil, "x", "")["x"] != "" {
		t.Fatal("nil choose")
	}
	m, s := AcceptScan(manual, map[string]bool{"old": true}, map[string]string{"a": "AI", "empty": "AI", "b": "AI"})
	if m["a"] != "hand" || m["empty"] != "" || m["b"] != "AI" || !s["b"] || !s["old"] || s["a"] {
		t.Fatalf("accepted = %v %v", m, s)
	}
	if m, s := AcceptScan(nil, nil, nil); m == nil || s == nil {
		t.Fatal("nil acceptance")
	}
}

func TestScanDraftAndResults(t *testing.T) {
	rows := []Row{row("empty", TierNone), row("expense", TierNone, txn("a", "a", -100)), row("income", TierNone, txn("b", "b", 100)), row("zero", TierNone, txn("c", "c", 0))}
	rows[1].HasSugg = true
	rows[2].SuggestionConflict = true
	if gaps, n := Gaps(rows); len(gaps) != 2 || n != 1 {
		t.Fatalf("gaps=%v n=%d", gaps, n)
	}
	for _, tc := range []struct{ cap, want int }{{-1, 0}, {0, 0}, {1, 1}, {2, 2}, {100, 3}} {
		draft := DraftScan(rows, tc.cap, func(m money.Money) string { return fmt.Sprint(m.Amount) })
		if len(draft.Rows) != tc.want || len(draft.IncomeByRef) != tc.want {
			t.Fatalf("cap %d: %+v", tc.cap, draft)
		}
		if tc.want > 0 && !strings.HasPrefix(draft.Lines, "1 | expense | -100\n") {
			t.Fatalf("lines=%q", draft.Lines)
		}
	}
	draft := DraftScan(rows, 10, func(m money.Money) string { return fmt.Sprint(m.Amount) })
	if draft.IncomeByRef[1] || !draft.IncomeByRef[2] || draft.IncomeByRef[3] {
		t.Fatal("signs")
	}
	catalog := smartai.NewCatalog([]smartai.Cat{{ID: "food", Path: "Food"}, {ID: "salary", Path: "Salary", Income: true}})
	for _, tc := range []struct {
		answer string
		want   [2]int
	}{
		{"1 => Food | high\n2 => Salary | high\n3 => Food | high", [2]int{3, 0}},
		{"1 => Salary | high\n2 => Food | high\n4 => Food | high\n0 => Food | high", [2]int{0, 3}},
		{"garbage\n1 => Unknown | high", [2]int{0, 3}},
	} {
		previous := map[string]string{"old": "kept"}
		out, stats := ScanResult(draft, tc.answer, catalog, previous)
		if stats != tc.want || out["old"] != "kept" || len(previous) != 1 {
			t.Fatalf("result=%v stats=%v", out, stats)
		}
	}
	for _, tc := range []struct {
		gaps  int
		state string
		want  bool
	}{{0, "idle", false}, {1, "scanning", false}, {1, "done", true}, {-1, "idle", false}} {
		if CanScan(tc.gaps, tc.state) != tc.want {
			t.Fatal("scan enable")
		}
	}
}

func TestCardPositionScopeAndConfirmation(t *testing.T) {
	rows := []Row{row("a", TierReady, txn("a1", "a", -1), txn("a2", "a", -2)), row("b", TierNone, txn("b1", "b", -1))}
	for _, tc := range []struct {
		cursor         int
		key, commitKey string
		commit         reviewscope.CommitScope
		at, targets    int
	}{
		{1, "a", "a", reviewscope.CommitMerchant, 0, 2},
		{99, "gone", "a", reviewscope.CommitMerchant, 0, 2},
		{-1, "", "b", reviewscope.CommitMerchant, 0, 1},
		{0, "", "a", "invalid", 0, 1},
		{1, "b", "b", reviewscope.CommitMerchant, 1, 1},
	} {
		c := CurrentCard(rows, tc.cursor, tc.key, tc.commitKey, tc.commit)
		if c.At != tc.at || len(c.Targets) != tc.targets {
			t.Fatalf("card=%+v", c)
		}
	}
	if c := CurrentCard(nil, 0, "", "", ""); c.Txn.ID != "" || len(c.Targets) != 0 {
		t.Fatalf("empty card=%+v", c)
	}
	if c := CurrentCard([]Row{row("empty", TierNone)}, 0, "", "", ""); c.Txn.ID != "" {
		t.Fatal("empty group")
	}
	for _, tc := range []struct{ pos, length, want int }{{-1, 2, 1}, {0, 2, 0}, {2, 2, 0}, {1, 2, 1}, {-1, 0, 0}, {3, -1, 0}} {
		if Focus(tc.pos, tc.length) != tc.want {
			t.Fatalf("focus %+v", tc)
		}
	}
	for _, tc := range []struct {
		pos  int
		key  string
		want int
	}{{-1, "a", 0}, {99, "b", 1}, {0, "a", 0}} {
		at, key := MoveCard(rows, tc.pos)
		if at != tc.want || key != tc.key {
			t.Fatalf("move=%d,%q", at, key)
		}
	}
	if at, key := MoveCard(nil, 9); at != 0 || key != "" {
		t.Fatal("empty move")
	}
	broken := txn("split", "split", -100)
	broken.Splits = []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-20, "USD")}}
	for _, tc := range []struct {
		txn     domain.Transaction
		cat     string
		n       int
		pending string
		want    ConfirmDecision
	}{
		{domain.Transaction{}, "", 0, "", ConfirmDecision{}},
		{broken, "food", 1, "", ConfirmDecision{NoticeKey: "review.splitNeeded"}},
		{rows[0].Group.Items[0], "", 2, "", ConfirmDecision{NoticeKey: "review.chooseFirst"}},
		{rows[0].Group.Items[0], "food", 2, "", ConfirmDecision{Ask: true}},
		{rows[0].Group.Items[0], "food", 2, "a", ConfirmDecision{Commit: true}},
		{rows[0].Group.Items[0], "", 1, "", ConfirmDecision{Commit: true}},
	} {
		if got := Confirm(tc.txn, tc.cat, tc.n, tc.pending, "a"); got != tc.want {
			t.Fatalf("confirm=%+v want=%+v", got, tc.want)
		}
	}
	for _, tc := range []struct {
		key, pending string
		n            int
		want         bool
	}{{"a", "a", 2, true}, {"a", "b", 2, false}, {"", "", 2, false}, {"a", "a", 1, false}} {
		if PendingConfirmation(tc.key, tc.pending, tc.n) != tc.want {
			t.Fatal("pending confirmation")
		}
	}
	for _, tc := range []struct {
		criteria txnfilter.Criteria
		chosen   bool
		scope    reviewscope.Scope
		want     reviewscope.Scope
		filtered bool
	}{
		{txnfilter.Criteria{}, false, "", reviewscope.ScopeEntireQueue, false},
		{txnfilter.Criteria{Text: "coffee"}, false, "", reviewscope.ScopeCurrentView, true},
		{txnfilter.Criteria{Text: "coffee"}, true, reviewscope.ScopeEntireQueue, reviewscope.ScopeEntireQueue, true},
	} {
		scope, filtered := ActiveScope(tc.criteria, tc.chosen, tc.scope)
		if scope != tc.want || filtered != tc.filtered {
			t.Fatalf("scope=%q filtered=%v", scope, filtered)
		}
	}
}

func TestDirectionsAndCheckedSummary(t *testing.T) {
	for _, tc := range []struct {
		amount int64
		want   domain.CategoryKind
	}{{math.MinInt64, domain.KindExpense}, {-1, domain.KindExpense}, {0, domain.KindExpense}, {1, domain.KindIncome}, {math.MaxInt64, domain.KindIncome}} {
		tx := txn("a", "a", tc.amount)
		if Kind(tx) != tc.want || PickerKind(Index{Byid: map[string]Row{"a": row("a", TierNone, tx)}}, "a") != tc.want {
			t.Fatal("direction")
		}
	}
	if PickerKind(Index{}, "unknown") != domain.KindExpense || PickerKind(Index{Byid: map[string]Row{"empty": {}}}, "empty") != domain.KindExpense {
		t.Fatal("picker default")
	}
	a, b := txn("a", "a", -100), txn("b", "a", 50)
	a.Date = time.Date(2026, 10, 31, 23, 0, 0, 0, time.FixedZone("west", -7*3600))
	b.Date = time.Date(2026, 11, 1, 8, 0, 0, 0, time.FixedZone("east", 2*3600))
	for _, tc := range []struct {
		name  string
		items []domain.Transaction
		want  int64
		err   error
	}{
		{"empty", nil, 0, nil}, {"negative", []domain.Transaction{a}, -100, nil}, {"net", []domain.Transaction{a, b}, -50, nil},
		{"reverse", []domain.Transaction{b, a}, -50, nil}, {"zero", []domain.Transaction{txn("z", "a", 0)}, 0, nil},
		{"overflow", []domain.Transaction{txn("1", "a", math.MaxInt64), txn("2", "a", 1)}, 0, money.ErrOverflow},
		{"underflow", []domain.Transaction{txn("1", "a", math.MinInt64), txn("2", "a", -1)}, 0, money.ErrOverflow},
		{"mixed missing FX", []domain.Transaction{a, {Amount: money.New(10, "EUR")}}, 0, money.ErrCurrencyMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sum, first, last, err := GroupSummary(row("a", TierNone, tc.items...))
			if sum.Amount != tc.want || !errors.Is(err, tc.err) {
				t.Fatalf("summary=%v err=%v", sum, err)
			}
			if err == nil && len(tc.items) > 0 && first.After(last) {
				t.Fatal("reversed date span")
			}
		})
	}
	// Compare instants but preserve each endpoint's calendar date and location.
	b.Date = b.Date.Add(time.Hour)
	_, first, last, err := GroupSummary(row("a", TierNone, b, a))
	if err != nil || dateutil.FormatDate(first) != "2026-10-31" || dateutil.FormatDate(last) != "2026-11-01" {
		t.Fatalf("span=%v %v %v", first, last, err)
	}
}

func TestLargeIndexPartitionAndStableOrder(t *testing.T) {
	items := make([]domain.Transaction, 12000)
	for i := range items {
		items[i] = txn(fmt.Sprint(i), fmt.Sprintf("Merchant %c", 'a'+i%26), int64(-i))
	}
	before := append([]domain.Transaction(nil), items...)
	a := BuildIndex(Input{Transactions: items, Scope: reviewscope.ScopeEntireQueue})
	b := BuildIndex(Input{Transactions: items, Scope: reviewscope.ScopeEntireQueue})
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(before, items) {
		t.Fatal("unstable index or mutated input")
	}
	seen := map[string]bool{}
	for _, r := range a.Rows {
		for _, id := range ChargeIDs(r) {
			if seen[id] {
				t.Fatal("duplicate membership")
			}
			seen[id] = true
		}
	}
	if len(seen) != len(items) || a.Total != len(items) {
		t.Fatal("index lost charges")
	}
}

func FuzzSelectionRoundTrip(f *testing.F) {
	f.Add("coffee", true)
	f.Add("", false)
	f.Fuzz(func(t *testing.T, key string, value bool) {
		seed := map[string]bool{key: value, "other": false}
		got := Toggle(Toggle(seed, key), key)
		if !reflect.DeepEqual(seed, got) {
			t.Fatal("toggle round-trip")
		}
	})
}

func FuzzGroupTotalsReconcile(f *testing.F) {
	f.Add(int64(-100), int64(50))
	f.Add(int64(math.MinInt64), int64(-1))
	f.Fuzz(func(t *testing.T, a, b int64) {
		want, err := money.Add(a, b)
		got, _, _, actual := GroupSummary(row("a", TierNone, txn("a", "a", a), txn("b", "a", b)))
		if errors.Is(actual, money.ErrOverflow) != errors.Is(err, money.ErrOverflow) || (err == nil && got.Amount != want) {
			t.Fatalf("sum=%v,%v want=%v,%v", got, actual, want, err)
		}
	})
}

func TestTextDecisions(t *testing.T) {
	text := Text(func(key string, args ...any) string {
		if len(args) == 0 {
			return key
		}
		return fmt.Sprintf("%s:%v", key, args)
	})
	for _, tc := range []struct {
		scope              reviewscope.Scope
		queue, suggestions int
		title, sub         string
	}{
		{reviewscope.ScopeEntireQueue, 0, 0, "review.emptyTitle", "review.emptySub"},
		{reviewscope.ScopeEntireQueue, 0, 1, "r4loan1.suggestionOnlyTitle", "r4loan1.suggestionOnlySub"},
		{reviewscope.ScopeCurrentView, 1, 1, "review.emptyViewTitle", "review.emptyViewSub:[review.leftOne]"},
	} {
		title, sub := text.EmptyCopy(tc.scope, tc.queue, tc.suggestions)
		if title != tc.title || sub != tc.sub {
			t.Fatalf("empty copy=%q,%q", title, sub)
		}
	}
	checks := []struct{ name, got, want string }{
		{"return view", text.ReturnLabel(reviewscope.ScopeCurrentView, Index{Total: 3}), "review.returnLabelView:[3]"},
		{"return queue", text.ReturnLabel(reviewscope.ScopeEntireQueue, Index{}), "review.returnLabelQueue:[0]"},
		{"search", text.WorkingSetPhrase(txnfilter.Criteria{Text: " coffee "}), "review.scopeBySearch:[coffee]"},
		{"filters", text.WorkingSetPhrase(txnfilter.Criteria{Account: "a"}), "review.scopeByFilters:[1 filter]"},
		{"view", text.WorkingSetPhrase(txnfilter.Criteria{}), "review.scopeByView"},
		{"scope all", text.ScopeNote(reviewscope.ScopeCurrentView, 2, 2, "view"), "review.scopeViewNoteAll:[2 charges view]"},
		{"scope clamp", text.ScopeNote(reviewscope.ScopeCurrentView, 3, 2, "view"), "review.scopeViewNoteAll:[3 charges view]"},
		{"scope outside", text.ScopeNote(reviewscope.ScopeCurrentView, 2, 3, "view"), "review.scopeViewNote:[2 charges view review.waitingOne]"},
		{"scope queue", text.ScopeNote(reviewscope.ScopeEntireQueue, 2, 3, "view"), "review.scopeQueueNote:[3 charges 2 charges view]"},
		{"waiting many", text.WaitingPhrase(2), "review.waitingMany:[2]"},
		{"left one", text.LeftPhrase(1), "review.leftOne"},
		{"left many", text.LeftPhrase(0), "review.leftMany:[0]"},
		{"tier none", text.TierLabel(TierNone), "review.tierNone"},
		{"tier look", text.TierLabel(TierLook), "review.tierLook"},
		{"tier ready", text.TierLabel(TierReady), "review.tierReady"},
		{"reason none", text.ReasonLabel(reviewqueue.ReasonUncategorized), "review.reasonUncategorized"},
		{"reason flagged", text.ReasonLabel(reviewqueue.ReasonFlagged), "review.reasonFlagged"},
		{"reason split", text.ReasonLabel(reviewqueue.ReasonSplitUnbalanced), "review.reasonSplit"},
		{"bulk hint", text.KeyboardHint(true), "review.keyboardHintBulkPlain"},
		{"single hint", text.KeyboardHint(false), "review.keyboardHintSinglePlain"},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("got=%q want=%q", tc.got, tc.want)
			}
		})
	}
	for _, n := range []int{0, 1, 2} {
		confirm, snooze, dismiss := text.ActionLabels(n)
		if n > 1 {
			if confirm != "review.categorizeAllN:[2]" || snooze != "review.snoozeN:[2]" || dismiss != "review.dismissN:[2]" {
				t.Fatal("bulk action labels")
			}
		} else if confirm != "review.categorizeNext" || snooze != "review.snooze" || dismiss != "review.dismiss" {
			t.Fatal("single action labels")
		}
	}
	for _, n := range []int{0, 2} {
		label, sub, confirm := text.SelectionLabels(n, 3)
		if n == 0 {
			if label != "review.nothingPicked" || sub != "review.pickHint" || confirm != "review.confirm" {
				t.Fatal("empty selection labels")
			}
		} else if label != "review.selectionCountPlain:[2 merchants 3 charges]" || sub != "review.undoAvailable" || confirm != "review.confirmN:[3]" {
			t.Fatal("selection labels")
		}
	}
	format := func(m money.Money) string { return fmt.Sprint(m.Amount) }
	a, b := txn("a", "a", -100), txn("b", "a", 200)
	if got := text.GroupFacts(Row{}, prefs.Prefs{}, format); got != "" {
		t.Fatal("empty facts")
	}
	if got := text.GroupFacts(row("a", TierNone, a, b), prefs.Prefs{}, format); !strings.HasPrefix(got, "review.applyAllFacts:[100 ") {
		t.Fatalf("facts=%q", got)
	}
	b.Date = b.Date.AddDate(0, 1, 0)
	if got := text.GroupFacts(row("a", TierNone, b, a), prefs.Prefs{}, format); !strings.Contains(got, "review.dateRange:") {
		t.Fatalf("range facts=%q", got)
	}
	b.Amount = money.New(1, "EUR")
	if got := text.GroupFacts(row("a", TierNone, a, b), prefs.Prefs{}, format); got != "" {
		t.Fatal("invalid total displayed")
	}
}
