package budgetview

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTrackCodec(t *testing.T) {
	for _, tc := range []struct {
		raw string
		n   int
	}{{"", 0}, {"bad|c~1~oops~~1|a~1~9223372036854775808~~1", 0}, {"c~1~10~~1|d~0~-3~budget~0", 2}, {"c~1~1~~0|c~0~2~~1", 1}} {
		if got := ParseTrackRows(tc.raw); len(got) != tc.n {
			t.Fatal(got)
		}
	}
	for _, amount := range []int64{0, -1, 1, math.MinInt64, math.MaxInt64} {
		for _, on := range []bool{false, true} {
			r := TrackRow{Include: on, Amount: amount, BudgetID: "b", Raise: !on}
			if got := ParseTrackRows(EncodeTrackRow("c", r))["c"]; got != r {
				t.Fatal(got, r)
			}
		}
	}
	got := PutTrackRow("|a~1~1~~1|b~1~2~~1|a~0~3~~0|", "a", TrackRow{Amount: 4})
	if got != "b~1~2~~1|a~0~4~~0" {
		t.Fatal(got)
	}
	cands := []untracked.Candidate{{CategoryID: "a", SuggestMinor: 10}, {CategoryID: "b", SuggestMinor: 20}}
	rows := ResolveTrackRows(cands, map[string]TrackRow{"a": {Amount: 5}, "stale": {Include: true}})
	if len(rows) != 2 || rows[0].Amount != 5 || rows[0].Include || !rows[1].Include || !rows[1].Raise || rows[1].Amount != 20 {
		t.Fatal(rows)
	}
}

func TestPlanTrackUntracked(t *testing.T) {
	candidates := []untracked.Candidate{{CategoryID: "annual", Name: "Annual", SuggestMinor: 120000, Period: domain.PeriodYearly}, {CategoryID: "other", SuggestMinor: 15000, Period: domain.PeriodYearly}}
	destination := budget("dest", 5000)
	destination.Limit = money.New(5000, "EUR")
	destination.CategoryIDs = make([]string, 1, 5)
	destination.CategoryIDs[0] = "existing"
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": .8}}
	for _, member := range []string{"", "m"} {
		creates, updates, err := PlanTrackUntracked(candidates, map[string]TrackRow{"other": {Include: true, Amount: 15000, BudgetID: "dest", Raise: true}}, []domain.Budget{destination}, "USD", rates, member, func() string { return "new" })
		if err != nil || len(creates) != 1 || len(updates) != 1 || updates[0].Limit.Amount != 6563 {
			t.Fatal(creates, updates, err)
		}
		if creates[0].ID != "new" || creates[0].Limit.Amount != 120000 || (member == "" && creates[0].OwnerID != domain.GroupOwnerID) || (member != "" && creates[0].OwnerID != member) {
			t.Fatal(creates)
		}
		if destination.CategoryIDs[:2][1] != "" {
			t.Fatal("mutated input backing array")
		}
	}
	cands := []untracked.Candidate{{CategoryID: "c", SuggestMinor: 10, Period: domain.PeriodMonthly}, {CategoryID: "d", SuggestMinor: 20, Period: domain.PeriodMonthly}}
	for _, tc := range []struct {
		name    string
		dest    domain.Budget
		row     TrackRow
		wantErr string
		want    int64
	}{
		{"same category", budget("dest", 5), TrackRow{Include: true, Amount: 10, BudgetID: "dest", Raise: true}, "", 15},
		{"unchanged limit", budget("dest", 5), TrackRow{Include: true, Amount: 10, BudgetID: "dest"}, "", 5},
		{"unset currency", func() domain.Budget { b := budget("dest", 5); b.Limit.Currency = ""; return b }(), TrackRow{Include: true, Amount: 10, BudgetID: "dest", Raise: true}, "", 15},
		{"missing", budget("dest", 5), TrackRow{Include: true, Amount: 10, BudgetID: "deleted"}, "no longer exists", 0},
		{"missing fx", func() domain.Budget { b := budget("dest", 5); b.Limit.Currency = "GBP"; return b }(), TrackRow{Include: true, Amount: 10, BudgetID: "dest", Raise: true}, "convert raise", 0},
		{"overflow", budget("dest", math.MaxInt64), TrackRow{Include: true, Amount: 10, BudgetID: "dest", Raise: true}, "add raise", 0},
		{"scale overflow", func() domain.Budget { b := budget("dest", 1); b.Period = domain.PeriodYearly; return b }(), TrackRow{Include: true, Amount: math.MaxInt64, BudgetID: "dest", Raise: true}, "scale raise", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			creates, updates, err := PlanTrackUntracked(cands[:1], map[string]TrackRow{"c": tc.row}, []domain.Budget{tc.dest}, "USD", usd(), "", nil)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || creates != nil || updates != nil {
					t.Fatal(creates, updates, err)
				}
				return
			}
			if err != nil || len(updates) != 1 || updates[0].Limit.Amount != tc.want {
				t.Fatal(updates, err)
			}
		})
	}
	for _, row := range []TrackRow{{}, {Include: true, Amount: 0}, {Include: true, Amount: -1}} {
		creates, updates, err := PlanTrackUntracked(cands[:1], map[string]TrackRow{"c": row}, nil, "USD", usd(), "", nil)
		if err != nil || len(creates)+len(updates) != 0 {
			t.Fatal(creates, updates, err)
		}
	}
	edits := map[string]TrackRow{"c": {Include: true, Amount: 10, BudgetID: "z", Raise: true}, "d": {Include: true, Amount: 20, BudgetID: "z", Raise: true}}
	_, updates, err := PlanTrackUntracked(cands, edits, []domain.Budget{budget("z", 1), budget("a", 2)}, "USD", usd(), "", nil)
	if err != nil || len(updates) != 1 || updates[0].Limit.Amount != 31 || !reflect.DeepEqual(updates[0].CategoryIDs, []string{"c", "d"}) {
		t.Fatal(updates, err)
	}
}

func TestPreviewTrack(t *testing.T) {
	rows := []TrackRow{{Cand: untracked.Candidate{CategoryID: "a"}, Include: true, Amount: 50, BudgetID: "b"}, {Include: false, Amount: 500}}
	dest := budget("b", 100)
	dest.Name = "Food"
	statuses := []budgeting.Status{{Budget: dest, Spent: money.New(80, "USD"), Limit: dest.Limit}}
	preview, err := PreviewTrack(rows, 200, 10, 100, 5, statuses, []domain.Budget{dest}, map[string]string{"c": "Meals"})
	if err != nil || preview.Effect.Categories != 1 || preview.Effect.ToAssignBeforeMinor != 105 || preview.Effect.ToAssignAfterMinor != 105 || !reflect.DeepEqual(preview.RiskNames, []string{"Food \u00b7 Meals"}) {
		t.Fatal(preview, err)
	}
	for _, tc := range []struct {
		income, rolled, limit, savings int64
		rows                           []TrackRow
		statuses                       []budgeting.Status
	}{
		{math.MaxInt64, 1, 0, 0, nil, nil}, {0, 0, math.MaxInt64, 1, nil, nil}, {math.MinInt64, 0, 1, 0, nil, nil},
		{0, 0, 0, 0, []TrackRow{{Include: true, Amount: math.MaxInt64}, {Include: true, Amount: 1}}, nil},
		{0, 0, 0, 0, []TrackRow{{Include: true, Amount: 1, BudgetID: "b"}}, []budgeting.Status{{Budget: dest, Spent: money.New(math.MaxInt64, "USD")}}},
	} {
		_, err := PreviewTrack(tc.rows, tc.income, tc.rolled, tc.limit, tc.savings, tc.statuses, nil, nil)
		if !errors.Is(err, money.ErrOverflow) {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, cat, want string }{{"", "Cat", "Cat"}, {"Food", "food", "Food"}, {"Food", "Meals", "Food \u00b7 Meals"}, {"Food", "", "Food"}} {
		if BudgetTitle(tc.name, tc.cat) != tc.want {
			t.Fatal(tc)
		}
	}
	all := []domain.Budget{budget("g", 0), budget("m", 0), budget("other", 0)}
	all[1].OwnerID = "m"
	all[2].OwnerID = "other"
	for _, tc := range []struct {
		member string
		n      int
	}{{"", 3}, {"m", 2}, {"nobody", 1}} {
		if got := VisibleDestinations(all, tc.member); len(got) != tc.n {
			t.Fatal(got)
		}
	}
}

func TestTrackInputsAndScan(t *testing.T) {
	for _, tc := range []struct {
		raw, base string
		want      int64
	}{{"0", "USD", 0}, {"1.23", "USD", 123}, {"-1", "USD", 99}, {"", "USD", 99}, {"no", "USD", 99}, {"9223372036854775808", "JPY", 99}, {"123", "JPY", 123}} {
		if got := TrackAmountEdit(TrackRow{Amount: 99}, tc.raw, tc.base); got.Amount != tc.want {
			t.Fatal(tc, got)
		}
	}
	if CandidateDate(time.Time{}) != "" || CandidateDate(time.Date(2024, 2, 29, 23, 0, 0, 0, time.FixedZone("west", -8*3600))) != "2024-02-29" {
		t.Fatal("date")
	}
	for _, raw := range []string{"", "bad", "2023-02-29"} {
		if !ParsePayAnchor(raw).IsZero() {
			t.Fatal(raw)
		}
	}
	if ParsePayAnchor("2024-02-29") != day(2024, 2, 29) || TrackScanRange(day(2024, 2, 29)) != day(2023, 3, 1) {
		t.Fatal("anchor or historical lookback")
	}
	end := day(2026, 11, 1)
	start := day(2026, 10, 1)
	cats := []domain.Category{{ID: "c", Name: "Food", Kind: domain.KindExpense}}
	txns := []domain.Transaction{}
	for m := 7; m <= 10; m++ {
		txns = append(txns, domain.Transaction{CategoryID: "c", Date: day(2026, time.Month(m), 10), Amount: money.New(-100, "USD")})
	}
	for _, tc := range []struct {
		ledger budgeting.NettedLedger
		hint   bool
	}{{budgeting.NewNettedLedger(txns), true}, {nil, false}} {
		got, err := ScanUntracked(txns, tc.ledger, cats, nil, start, end, usd(), "USD", time.Monday, time.Time{})
		if err != nil || len(got) != 1 || got[0].FromHint != tc.hint {
			t.Fatal(got, err)
		}
	}
	bad := []domain.Transaction{{CategoryID: "c", Date: start, Amount: money.New(-1, "EUR")}}
	if got, err := ScanUntracked(bad, nil, cats, nil, start, end, usd(), "USD", time.Monday, time.Time{}); err != nil || len(got) != 0 {
		t.Fatal("missing FX must keep historical omission", got, err)
	}
}

func FuzzTrackCodec(f *testing.F) {
	f.Add(int64(0), true, false, "c", "b")
	f.Add(int64(math.MinInt64), false, true, "cat", "budget")
	f.Fuzz(func(t *testing.T, amount int64, include, raise bool, cat, dest string) {
		if cat == "" || strings.ContainsAny(cat+dest, "~|") {
			t.Skip()
		}
		r := TrackRow{Amount: amount, Include: include, Raise: raise, BudgetID: dest}
		got := ParseTrackRows(PutTrackRow("", cat, r))
		if got[cat] != r {
			t.Fatal(got, r)
		}
	})
}

func TestTrackRowSelectionEdits(t *testing.T) {
	for _, initial := range []TrackRow{{}, {Cand: untracked.Candidate{CategoryID: "c"}, Include: true, Amount: 123, BudgetID: "old", Raise: true}} {
		for _, included := range []bool{true, false} {
			got := initial.WithIncluded(included)
			want := initial
			want.Include = included
			if got != want {
				t.Fatal(got, want)
			}
		}
		for _, id := range []string{"", "new"} {
			got := initial.WithDestination(id)
			want := initial
			want.BudgetID = id
			if got != want {
				t.Fatal(got, want)
			}
		}
		got := initial.ToggleRaise()
		if got.Raise == initial.Raise || got.ToggleRaise() != initial {
			t.Fatal(got, initial)
		}
	}
}
