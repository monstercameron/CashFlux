// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nlfilter"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

var presentation = Presentation{
	Text: func(key string, args ...any) string {
		parts := make([]string, len(args))
		for i, arg := range args {
			parts[i] = fmt.Sprint(arg)
		}
		return key + strings.Join(parts, " ")
	},
	Money: func(m money.Money) string { return fmt.Sprintf("%s:%d", m.Currency, m.Amount) },
	Date:  func(d time.Time) string { return d.Format("2006-01-02") },
}

func transaction(id string, amount int64) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "checking", Date: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), Amount: money.New(amount, "USD")}
}

func TestDeriveCurrentUncollapsedScope(t *testing.T) {
	coffee := transaction("coffee", -400)
	coffee.Desc = "coffee"
	coffee.CategoryID = "dining"
	rent := transaction("rent", -20000)
	rent.Desc = "rent"
	rent.CategoryID = "housing"
	income := transaction("salary", 100000)
	out := transaction("out", -5000)
	out.TransferAccountID = "savings"
	in := transaction("in", 5000)
	in.AccountID = "savings"
	in.TransferAccountID = "checking"
	foreign := transaction("eur", -100)
	foreign.Amount.Currency = "EUR"
	rows := []domain.Transaction{coffee, rent, income, out, in, foreign}
	rates := currency.Rates{Base: "USD"}
	labels := txnfilter.Labels{BaseCurrency: "USD", ToBase: func(m money.Money) (money.Money, error) { return rates.Convert(m, "USD") }}
	for _, tt := range []struct {
		name                                string
		filter                              txnfilter.Criteria
		spent, income                       int64
		count, unconverted, moved, unpaired int
	}{
		{"coffee", txnfilter.Criteria{Text: "coffee"}, 400, 0, 1, 0, 0, 0},
		{"clear search uses whole pair", txnfilter.Criteria{}, 20400, 100000, 6, 1, 1, 0},
		{"no results", txnfilter.Criteria{Text: "dentist"}, 0, 0, 0, 0, 0, 0},
		{"one leg", txnfilter.Criteria{Account: "checking"}, 20400, 100000, 5, 1, 1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Derive(rows, tt.filter, labels, rates, "USD", rows[0].Date)
			if err != nil {
				t.Fatal(err)
			}
			v := got.View
			if v.Spent != tt.spent || v.Income != tt.income || v.Count != tt.count || v.Unconverted != tt.unconverted || v.MovedRows != tt.moved || v.UnpairedRows != tt.unpaired {
				t.Fatalf("view=%+v", v)
			}
			if len(got.Rows) != tt.count || v.Net != v.Income-v.Spent {
				t.Fatal("scope/totals do not reconcile")
			}
		})
	}
	// TC4-14: both category controls get the same contextual breakdown.
	for _, filter := range []txnfilter.Criteria{{Category: "dining"}, {Categories: "dining"}, {Categories: "dining", ScopeAny: true}} {
		got, err := Derive(rows, filter, labels, rates, "USD", rows[0].Date)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.View.Categories) != 2 {
			t.Fatalf("category context lost: %+v", got.View.Categories)
		}
	}
	for _, amounts := range [][]int64{{math.MinInt64}, {math.MaxInt64, 1}} {
		var bad []domain.Transaction
		for i, v := range amounts {
			bad = append(bad, transaction(fmt.Sprint(i), v))
		}
		if _, err := Derive(bad, txnfilter.Criteria{}, labels, rates, "USD", time.Now()); err == nil {
			t.Fatal("overflow accepted")
		}
	}
	got, err := Derive(rows, txnfilter.Criteria{AmountMin: "1"}, labels, rates, "USD", rows[0].Date)
	if err != nil || len(got.Issues) != 1 {
		t.Fatalf("conversion filter issues=%+v, %v", got.Issues, err)
	}
	empty, err := Derive(nil, txnfilter.Criteria{}, txnfilter.Labels{}, rates, "USD", time.Time{})
	if err != nil || empty.View.Count != 0 {
		t.Fatal(empty, err)
	}
}

func TestScopeDecisions(t *testing.T) {
	f := txnfilter.Criteria{Text: "coffee", Category: "one", Categories: "two", ScopeAny: true, Uncategorized: true, Flow: "out", Sort: "date", Dir: "asc", Page: 2, PageSize: 10}
	if SummaryScope(f) != AnnouncementScope(f) {
		t.Fatal("summary/announcement scope differ")
	}
	if got := CategoryScope(f); got.Category != "" || got.Categories != "" || got.ScopeAny || got.Uncategorized || got.Flow != "" || got.Text != "coffee" {
		t.Fatal(got)
	}
	for _, tt := range []struct {
		f    txnfilter.Criteria
		want bool
	}{{txnfilter.Criteria{}, false}, {txnfilter.Criteria{Category: "x"}, true}, {txnfilter.Criteria{Categories: "x"}, true}, {txnfilter.Criteria{Uncategorized: true}, true}} {
		if CategoryFiltered(tt.f) != tt.want {
			t.Fatal(tt)
		}
	}
	if ShowUpcoming(txnfilter.Criteria{Text: "x"}) || !ShowUpcoming(txnfilter.Criteria{Text: "  "}) {
		t.Fatal("upcoming search scope")
	}
	for _, tt := range []struct {
		enabled bool
		scope   string
		n       int
		want    bool
	}{{true, "", 2, true}, {false, "", 2, false}, {true, "m", 3, false}, {true, "", 1, false}} {
		if UserColumnVisible(tt.enabled, tt.scope, tt.n) != tt.want {
			t.Fatal(tt)
		}
	}
	if LiveMode(true) != "polite" || LiveMode(false) != "off" {
		t.Fatal("live mode")
	}
}

func TestDatesAndNaturalLanguageLocalDay(t *testing.T) {
	for _, style := range []prefs.DateStyle{prefs.DateISO, prefs.DateUS, prefs.DateEU, prefs.DateLong, "unknown"} {
		for _, iso := range []string{"2024-02-29", "2026-01-31", "2026-12-31"} {
			formatted := FormatDate(style, iso)
			got, ok := ParseDate(style, formatted)
			if !ok || got != iso {
				t.Fatalf("%s round trip=%s/%v", style, got, ok)
			}
			if got, ok := ParseDate(style, iso); !ok || got != iso {
				t.Fatal("ISO fallback")
			}
		}
		if got, ok := ParseDate(style, "bad"); ok || got != "" {
			t.Fatal("bad date accepted")
		}
		if FormatDate(style, "2026-02-30") != "2026-02-30" {
			t.Fatal("legacy date changed")
		}
	}
	if ChipDate("bad", presentation.Date) != "bad" || ChipDate("2026-10-18", presentation.Date) != "2026-10-18" {
		t.Fatal("chip date")
	}
	for _, offset := range []int{-7 * 3600, 14 * 3600} {
		now := time.Date(2026, 10, 1, 0, 30, 0, 0, time.FixedZone("local", offset))
		if offset < 0 {
			now = time.Date(2026, 9, 30, 23, 30, 0, 0, now.Location())
		}
		context := NaturalLanguageContext([]domain.Category{{ID: "c", Name: "Dining"}}, []domain.Transaction{{Payee: "raw"}, {Payee: "raw"}, {Payee: ""}}, []string{"tag"}, func(s string) string {
			if s == "raw" {
				return "Coffee"
			}
			return s
		}, now, time.Monday)
		got, ok := nlfilter.Parse("today", context)
		if !ok || got.From != now.Format("2006-01-02") {
			t.Fatalf("local today=%+v, %v at %v", got, ok, now)
		}
		if context.Now.Location() != now.Location() || len(context.Payees) != 1 || len(context.Categories) != 1 || context.WeekStart != time.Monday {
			t.Fatal(context)
		}
	}
	if got := NaturalLanguageContext(nil, []domain.Transaction{{Payee: "raw"}}, nil, nil, time.Time{}, time.Sunday); len(got.Payees) != 1 {
		t.Fatal(got)
	}
	current := txnfilter.Criteria{Text: "today", From: "2026-01-01", To: "2026-12-31", Account: "a"}
	parsed := txnfilter.Criteria{From: "2026-10-01", AmountMin: "1", AmountMax: "2", Cleared: "yes", Flow: "out", Categories: "c", Tags: "t"}
	got := ApplyNaturalLanguage(current, parsed)
	if got.Text != "" || got.To != "" || got.PeriodPreset != txnfilter.PeriodCustom || got.Account != "a" || got.AmountMin != "1" || got.AmountMax != "2" || got.Cleared != "yes" || got.Flow != "out" || got.Categories != "c" || got.Tags != "t" {
		t.Fatal(got)
	}
	untouched := current
	ApplyParsedDateBounds(&untouched, txnfilter.Criteria{})
	if untouched != current {
		t.Fatal("empty parse changed dates")
	}
	if ApplyNaturalLanguage(current, txnfilter.Criteria{}).Account != "a" {
		t.Fatal("unparsed field lost")
	}
	cleared := ClearBudgetScope(got)
	if cleared.Account != "a" || cleared.From != "" || cleared.Categories != "" || cleared.Tags != "" || cleared.ScopeAny || cleared.PeriodPreset != txnfilter.PeriodAllDates {
		t.Fatal(cleared)
	}
}

func TestPresentationDecisions(t *testing.T) {
	for _, tt := range []struct {
		filter txnfilter.Criteria
		want   string
	}{
		{txnfilter.Criteria{}, "r2a.allDates"},
		{txnfilter.Criteria{To: "2026-10-01"}, "txnTQ6.throughDate"},
		{txnfilter.Criteria{From: "2026-10-01"}, "txnTQ6.fromDate"},
		{txnfilter.Criteria{From: "2024-02-01", To: "2024-02-29"}, "r2a.month"},
		{txnfilter.Criteria{From: "2026-10-02", To: "2026-10-03"}, "txnperiod.customRange"},
		{txnfilter.Criteria{From: "bad"}, "r2a.customRange"},
	} {
		if got := presentation.CountPeriod(tt.filter); !strings.HasPrefix(got, tt.want) {
			t.Fatal(got, tt.want)
		}
	}
	for _, tt := range []struct {
		filter txnfilter.Criteria
		want   string
	}{
		{txnfilter.Criteria{AmountMin: "2", AmountMax: "1"}, "transactions.filterRangeAmountInvalid"},
		{txnfilter.Criteria{From: "2026-10-02", To: "2026-10-01"}, "transactions.filterRangeDateInvalid"},
		{txnfilter.Criteria{AmountMin: "bad", AmountMax: "1"}, ""},
		{txnfilter.Criteria{From: "bad", To: "2026-01-01"}, ""},
		{txnfilter.Criteria{AmountMin: "-2", AmountMax: "1"}, ""},
	} {
		if got := presentation.RangeError(tt.filter, "USD"); got != tt.want {
			t.Fatal(got, tt.want)
		}
	}
	for _, tt := range []struct{ bound, value, base, want string }{{"min", "1.25", "USD", "USD:125"}, {"max", "-1", "USD", "USD:-100"}, {"min", "0.125", "KWD", "0.125"}, {"min", "bad", "USD", "bad"}, {"other", "raw", "USD", "raw"}} {
		if got := presentation.AmountChipLabel(tt.bound, tt.value, tt.base); !strings.Contains(got, tt.want) {
			t.Fatal(got, tt.want)
		}
	}
	for _, tt := range []struct {
		page, size, total int
		want              string
	}{{1, 10, 0, ""}, {1, -1, 10, "txu5.range1 10 10"}, {1, 0, 2, "txu5.range1 2 2"}, {-1, 3, 10, "txu5.range1 3 10"}, {99, 3, 10, "txu5.range10 10 10"}, {2, 3, 10, "txu5.range4 6 10"}} {
		if got := presentation.PageRange(tt.page, tt.size, tt.total); got != tt.want {
			t.Fatal(got, tt.want)
		}
	}
	for _, tt := range []struct {
		n    int
		want string
	}{{0, "0"}, {99, "99"}, {1000, "1,000"}, {1000000, "1,000,000"}} {
		if CountNumber(tt.n) != tt.want {
			t.Fatal(tt)
		}
	}
	for _, tt := range []struct{ name, want string }{{"", "—"}, {"   ", "—"}, {"cam", "C"}, {" élise van doe ", "ÉD"}} {
		if got := MemberInitials(tt.name); got != tt.want {
			t.Fatal(got, tt.want)
		}
	}
}

func TestIndexOptionsAndCategories(t *testing.T) {
	rows := []domain.Transaction{{Tags: []string{" b ", "a", "", "a"}}, {Tags: []string{"b"}}}
	indexes := BuildIndexes([]domain.Account{{ID: "a", Name: "Checking"}}, []domain.Category{{ID: "c", Name: "Dining"}}, []domain.Member{{ID: "m", Name: "Cam"}}, rows)
	if indexes.Accounts["a"] != "Checking" || indexes.Categories["c"] != "Dining" || indexes.Members["m"] != "Cam" || !reflect.DeepEqual(indexes.Tags, []string{"a", "b"}) {
		t.Fatal(indexes)
	}
	empty := BuildIndexes(nil, nil, nil, nil)
	if empty.Accounts == nil || empty.Categories == nil || empty.Members == nil {
		t.Fatal("nil lookup")
	}
	options := []Option{{Value: "", Label: "all"}}
	for i := 0; i < 12; i++ {
		options = append(options, Option{Value: fmt.Sprint(i), Label: fmt.Sprintf("Option %d", i)})
	}
	for _, collapsed := range []bool{false, true} {
		got := VisibleOptions(options, []string{"11"}, collapsed)
		want := 12
		if collapsed {
			want = 9
		}
		if len(got) != want || got[len(got)-1].Value != "11" {
			t.Fatal(got)
		}
	}
	got, n := SearchOptions(options, []string{"0", "11"}, " option 1 ")
	if n != 2 || len(got) != 3 {
		t.Fatal(got, n)
	}
	if got := VisibleOptions(nil, nil, true); len(got) != 0 {
		t.Fatal(got)
	}
	if Contains(nil, "x") || !Contains([]string{"x"}, "x") {
		t.Fatal("membership")
	}
	cats := Categories([]txnview.CategoryTotal{{ID: "a", Minor: 100}, {ID: "b", Minor: 100}, {ID: "c", Minor: 50}}, map[string]string{"a": "Z", "b": "A"}, "Uncategorized")
	if cats[0].ID != "b" || cats[1].ID != "a" || cats[2].Name != "Uncategorized" {
		t.Fatal(cats)
	}
	var top []CategoryTotal
	for i := 0; i < 8; i++ {
		top = append(top, CategoryTotal{ID: fmt.Sprint(i), Minor: int64(10 - i)})
	}
	for _, selected := range []string{"", "1", "7"} {
		got := presentation.CategoryTopFive(top, selected)
		var sum int64
		for _, c := range got {
			sum += c.Minor
		}
		if sum != 52 {
			t.Fatal("top-five sum", got, sum)
		}
	}
	if len(presentation.CategoryTopFive(top[:3], "")) != 3 {
		t.Fatal("small categories")
	}
	zero := append([]CategoryTotal(nil), top...)
	for i := 5; i < len(zero); i++ {
		zero[i].Minor = 0
	}
	if len(presentation.CategoryTopFive(zero, "")) != 5 {
		t.Fatal("zero other")
	}
	bad := append([]CategoryTotal(nil), top...)
	bad[5].Minor = math.MaxInt64
	if len(presentation.CategoryTopFive(bad, "")) != len(bad) {
		t.Fatal("overflow must retain source")
	}
	for _, tt := range []struct {
		a, total int64
		want     float64
	}{{0, 100, 0}, {10, 0, 0}, {-1, 10, 0}, {25, 100, 25}} {
		if CategoryShare(tt.a, tt.total) != tt.want {
			t.Fatal(tt)
		}
	}
}

func TestSelectionOwnersAndTags(t *testing.T) {
	rows := []domain.Transaction{transaction("a", -100), transaction("b", 0)}
	for _, selected := range []map[string]bool{nil, {"a": true}, {"a": true, "b": true}} {
		got := ToggleSelection(rows, selected)
		if selected["a"] && selected["b"] {
			if len(got) != 0 {
				t.Fatal(got)
			}
		} else if len(got) != 2 {
			t.Fatal(got)
		}
	}
	if len(ToggleSelection(nil, nil)) != 0 {
		t.Fatal("empty selection")
	}
	if !reflect.DeepEqual(SelectionIDs(map[string]bool{"b": true, "a": false}), []string{"a", "b"}) {
		t.Fatal("deterministic IDs")
	}
	if !reflect.DeepEqual(IDs(rows), []string{"a", "b"}) || len(SelectedIDs(rows)) != 2 {
		t.Fatal("row index")
	}
	if len(SelectedRows(rows, nil, false)) != 0 || len(SelectedRows(rows, map[string]bool{"a": true}, false)) != 1 {
		t.Fatal("selected rows")
	}
	for _, tt := range []struct {
		selected []string
		value    string
		want     []string
	}{
		{nil, "m", []string{"m"}}, {[]string{"m"}, "m", []string{}},
		{[]string{UnassignedOwner}, "m", []string{"m"}}, {[]string{"m"}, UnassignedOwner, []string{UnassignedOwner}},
		{[]string{UnassignedOwner}, UnassignedOwner, nil}, {[]string{"a", "b"}, "b", []string{"a"}},
	} {
		if got := ToggleOwner(tt.selected, tt.value); !reflect.DeepEqual(got, tt.want) {
			t.Fatal(got, tt.want)
		}
	}
	source := []string{"a", "b"}
	copied := AppliedOwners(source)
	copied[0] = "changed"
	if source[0] != "a" || AppliedOwners([]string{UnassignedOwner}) != nil {
		t.Fatal("owners alias")
	}
	if !SameOwners(nil, nil) || SameOwners(nil, source) || SameOwners(source, []string{"b", "a"}) {
		t.Fatal("owner comparison")
	}
	tags := []string{" Work "}
	for _, tag := range []string{"", " ", "work"} {
		if len(AddTag(tags, tag)) != 1 {
			t.Fatal("duplicate tag")
		}
	}
	added := AddTag(tags, " new ")
	added[0] = "changed"
	if tags[0] != " Work " || added[1] != "new" || !HasTag(tags, "WORK") || HasTag(nil, "x") {
		t.Fatal("tag copy")
	}
	if LensName(nil, true, nil, func(int) string { return "many" }) != "" || LensName([]string{"a"}, false, nil, nil) != "" || LensName([]string{"a"}, true, map[string]string{"a": "Cam"}, nil) != "Cam" || LensName([]string{"a", "b"}, true, nil, func(int) string { return "many" }) != "many" {
		t.Fatal("lens")
	}
}

func TestBulkDraftEveryOperationAndSnapshotIsolation(t *testing.T) {
	row := transaction("a", -100)
	row.Tags = []string{"keep"}
	row.SetOwners([]string{"old"})
	transfer := transaction("transfer", -500)
	transfer.TransferAccountID = "savings"
	rows := []domain.Transaction{row, transfer}
	selected := map[string]bool{"a": true, "transfer": true}
	for _, tt := range []struct {
		name  string
		edit  Edit
		count int
	}{
		{"clear", Edit{Kind: Clear, Value: true, Now: row.Date}, 2}, {"clear no-op", Edit{Kind: Clear, Value: false}, 0},
		{"review", Edit{Kind: Review}, 2}, {"exclude", Edit{Kind: Exclude, Value: true}, 2}, {"exclude no-op", Edit{Kind: Exclude}, 0},
		{"category skips transfer", Edit{Kind: Category, Text: "dining"}, 1},
		{"owners", Edit{Kind: Owners, Owners: []string{"new"}}, 2}, {"owners same", Edit{Kind: Owners, Owners: []string{"old"}}, 1},
		{"tag", Edit{Kind: Tag, Text: "new"}, 2}, {"tag empty", Edit{Kind: Tag, Text: " "}, 0}, {"tag same", Edit{Kind: Tag, Text: "keep"}, 1},
		{"unknown", Edit{Kind: EditKind(99)}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := Draft(rows, selected, tt.edit)
			if len(got) != tt.count {
				t.Fatalf("draft=%+v want %d", got, tt.count)
			}
			for _, draft := range got {
				if len(draft.Tags) > 0 {
					draft.Tags[0] = "changed"
				}
			}
			if rows[0].Tags[0] != "keep" {
				t.Fatal("draft mutated original tags")
			}
		})
	}
	if len(Draft(rows, nil, Edit{Kind: Review})) != 0 {
		t.Fatal("nil selection")
	}
	if CategoryKind(nil, nil) != domain.KindExpense || CategoryKind(rows, selected) != domain.KindExpense {
		t.Fatal("expense default")
	}
	positive := transaction("positive", 1)
	if CategoryKind([]domain.Transaction{transfer, positive}, map[string]bool{"transfer": true, "positive": true}) != domain.KindIncome {
		t.Fatal("income category")
	}
	if len(SelectedRows(rows, selected, true)) != 1 {
		t.Fatal("group transfers")
	}
}

func TestCheckedFutureTotalsAndFacts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		amounts []money.Money
		total   int64
		omitted int
	}{
		{"nil", nil, 0, 0}, {"positive negative zero", []money.Money{money.New(100, "USD"), money.New(-30, "USD"), money.New(0, "USD")}, 70, 0},
		{"missing FX", []money.Money{money.New(1, "EUR")}, 0, 1},
		{"overflow", []money.Money{money.New(math.MaxInt64, "USD"), money.New(1, "USD")}, math.MaxInt64, 1},
		{"underflow", []money.Money{money.New(math.MinInt64, "USD"), money.New(-1, "USD")}, math.MinInt64, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			total, n := FutureTotal(tt.amounts, currency.Rates{Base: "USD"}, "USD")
			if total.Amount != tt.total || total.Currency != "USD" || n != tt.omitted {
				t.Fatal(total, n)
			}
		})
	}
	amounts := BillAmounts([]bills.Bill{{Amount: money.New(-100, "USD")}, {Amount: money.New(-500, "USD"), TransferAccountID: "savings"}})
	if len(amounts) != 1 || amounts[0].Amount != -100 {
		t.Fatal(amounts)
	}
	if WithCurrency(money.New(1, ""), "USD").Currency != "USD" || WithCurrency(money.New(1, "EUR"), "USD").Currency != "EUR" {
		t.Fatal("currency fallback")
	}
	if !ShowsNet(money.New(1, "USD"), money.New(1, "USD")) || ShowsNet(money.New(0, "USD"), money.New(1, "USD")) {
		t.Fatal("net visibility")
	}
	if ExcludedCount([]domain.Transaction{{ExcludeFromReports: true}, {}}) != 1 || ExcludedCount(nil) != 0 {
		t.Fatal("excluded rows")
	}
	issues := []txnfilter.ConversionIssue{{Transaction: domain.Transaction{Amount: money.New(1, " eur ")}}, {Transaction: domain.Transaction{Amount: money.New(1, "EUR")}}, {Transaction: domain.Transaction{}}}
	if got := ConversionIssueCounts(issues); !reflect.DeepEqual(got, []IssueCount{{Currency: "EUR", Count: 2}, {Currency: "unknown", Count: 1}}) {
		t.Fatal(got)
	}
	if len(ConversionIssueCounts(nil)) != 0 {
		t.Fatal("nil issues")
	}
}

func TestMemoAndFacts(t *testing.T) {
	var memo Memo[int]
	calls := 0
	compute := func() int { calls++; return calls }
	filter := txnfilter.Criteria{Text: "coffee"}
	if memo.Get(1, filter, compute) != 1 || memo.Get(1, filter, compute) != 1 || memo.Get(2, filter, compute) != 2 || memo.Get(2, txnfilter.Criteria{}, compute) != 3 {
		t.Fatal("memo invalidation")
	}
	for _, tt := range []struct {
		facts, want                            []string
		moved, unpaired, unconverted, excluded int
		lens                                   bool
	}{
		{[]string{"count"}, []string{}, 0, 0, 0, 0, false},
		{[]string{"count", "moved", "unpaired", "unconverted", "FX", "excluded", "lens"}, []string{"FX", "lens"}, 1, 1, 1, 1, true},
		{[]string{"count", "moved", "FX", "excluded"}, []string{"FX"}, 1, 0, 0, 1, false},
		{nil, []string{}, 0, 0, 0, 0, false},
	} {
		if got := UniqueFacts(tt.facts, tt.moved, tt.unpaired, tt.unconverted, tt.excluded, tt.lens); !reflect.DeepEqual(got, tt.want) {
			t.Fatal(got, tt.want)
		}
	}
}

func TestTotalsReconcileProperty(t *testing.T) {
	if err := quick.Check(func(a, b, c int32) bool {
		rows := []domain.Transaction{transaction("a", int64(a)), transaction("b", int64(b)), transaction("c", int64(c))}
		got, err := Derive(rows, txnfilter.Criteria{}, txnfilter.Labels{}, currency.Rates{Base: "USD"}, "USD", time.Time{})
		return err == nil && got.View.Net == int64(a)+int64(b)+int64(c)
	}, &quick.Config{MaxCount: 200}); err != nil {
		t.Fatal(err)
	}
	huge := make([]domain.Transaction, 20000)
	for i := range huge {
		huge[i] = transaction(fmt.Sprint(i), -1)
	}
	got, err := Derive(huge, txnfilter.Criteria{}, txnfilter.Labels{}, currency.Rates{Base: "USD"}, "USD", time.Time{})
	if err != nil || got.View.Spent != 20000 || got.View.Count != 20000 {
		t.Fatal(got.View, err)
	}
}

func FuzzDateRoundTrip(f *testing.F) {
	f.Add("2024-02-29")
	f.Add("bad")
	f.Add("2026-01-31")
	f.Fuzz(func(t *testing.T, value string) {
		if _, ok := ParseDate(prefs.DateISO, value); !ok {
			return
		}
		for _, style := range []prefs.DateStyle{prefs.DateISO, prefs.DateUS, prefs.DateEU, prefs.DateLong} {
			got, ok := ParseDate(style, FormatDate(style, value))
			if !ok || got != value {
				t.Fatalf("round trip %q -> %q (%v)", value, got, ok)
			}
		}
	})
}
