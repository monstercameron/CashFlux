// SPDX-License-Identifier: MIT

package splitdraft_test

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
	"github.com/monstercameron/CashFlux/internal/splitdraft"
)

func transaction(amount int64, code string) domain.Transaction {
	return domain.Transaction{ID: "parent", CategoryID: "food", MemberID: "payer", Amount: money.New(amount, code)}
}

func view(s splitdraft.State, tx domain.Transaction) splitdraft.View {
	v := s.View(tx)
	lines := make([]split.Line, 0, len(s.Lines))
	for _, line := range s.Lines {
		lines = append(lines, split.Line{CategoryID: line.Cat, Value: line.Amt})
	}
	shape := split.Classify(lines)
	return s.Present(v, shape, shape.Saveable(!v.ParseError, v.Remainder == 0), tx.Amount.Currency)
}

func TestNewAndReseed(t *testing.T) {
	for _, tc := range []struct {
		name string
		tx   domain.Transaction
		want []splitdraft.Line
	}{
		{"unsplit", transaction(-100, "USD"), []splitdraft.Line{{Cat: "food"}, {}}},
		{"zero", transaction(0, "JPY"), []splitdraft.Line{{Cat: "food"}, {}}},
		{"saved", domain.Transaction{Amount: money.New(-1234, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "a", MemberID: "m", Amount: money.New(-1234, "USD")},
		}}, []splitdraft.Line{{Cat: "a", Amt: "12.34", Owner: "m"}}},
		{"zero precision", domain.Transaction{Amount: money.New(12, "JPY"), Splits: []domain.CategorySplit{
			{CategoryID: "a", Amount: money.New(12, "JPY")},
		}}, []splitdraft.Line{{Cat: "a", Amt: "12"}}},
		{"unknown currency retains registry fallback", domain.Transaction{Amount: money.New(-1234, "KWD"), Splits: []domain.CategorySplit{
			{CategoryID: "a", Amount: money.New(-1234, "KWD")},
		}}, []splitdraft.Line{{Cat: "a", Amt: "12.34"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := splitdraft.New(tc.tx, 7)
			if !reflect.DeepEqual(s.Lines, tc.want) || s.Generation != 7 || s.PickerLine != -1 || s.Percent {
				t.Fatalf("New = %+v, want lines %+v", s, tc.want)
			}
			s.Percent, s.ErrorKey, s.SaveAttempted, s.PickerLine = true, "old", true, 1
			if got := s.Reseed(transaction(1000, "USD"), 7); !reflect.DeepEqual(got, s) {
				t.Fatal("same generation discarded in-progress edits")
			}
			next := s.Reseed(transaction(1000, "USD"), 8)
			if next.Percent || next.ErrorKey != "" || next.Generation != 8 || !next.SaveAttempted || next.PickerLine != 1 {
				t.Fatalf("reseed = %+v", next)
			}
		})
	}
}

func TestApplyRowTransitionsAreImmutable(t *testing.T) {
	initial := splitdraft.State{Lines: []splitdraft.Line{{Cat: "a", Amt: "1", Owner: "m"}, {Cat: "b", Amt: "2"}}, PickerLine: 1}
	for _, tc := range []struct {
		name string
		a    splitdraft.Action
		want []splitdraft.Line
		pick int
	}{
		{"category", splitdraft.Action{Kind: splitdraft.SetCategory, Value: "c"}, []splitdraft.Line{{Cat: "c", Amt: "1", Owner: "m"}, {Cat: "b", Amt: "2"}}, 1},
		{"amount", splitdraft.Action{Kind: splitdraft.SetAmount, Value: "3"}, []splitdraft.Line{{Cat: "a", Amt: "3", Owner: "m"}, {Cat: "b", Amt: "2"}}, 1},
		{"owner", splitdraft.Action{Kind: splitdraft.SetOwner, Value: "n"}, []splitdraft.Line{{Cat: "a", Amt: "1", Owner: "n"}, {Cat: "b", Amt: "2"}}, 1},
		{"add", splitdraft.Action{Kind: splitdraft.Add}, []splitdraft.Line{{Cat: "a", Amt: "1", Owner: "m"}, {Cat: "b", Amt: "2"}, {}}, 1},
		{"remove first", splitdraft.Action{Kind: splitdraft.Remove}, []splitdraft.Line{{Cat: "b", Amt: "2"}}, 1},
		{"remove last", splitdraft.Action{Kind: splitdraft.Remove, Index: 1}, []splitdraft.Line{{Cat: "a", Amt: "1", Owner: "m"}}, 1},
		{"pick", splitdraft.Action{Kind: splitdraft.PickCategory, Index: 1, Value: "c"}, []splitdraft.Line{{Cat: "a", Amt: "1", Owner: "m"}, {Cat: "c", Amt: "2"}}, -1},
		{"open", splitdraft.Action{Kind: splitdraft.OpenPicker, Index: 0}, initial.Lines, 0},
		{"close", splitdraft.Action{Kind: splitdraft.ClosePicker}, initial.Lines, -1},
		{"negative index", splitdraft.Action{Kind: splitdraft.Remove, Index: -1}, initial.Lines, 1},
		{"past end", splitdraft.Action{Kind: splitdraft.SetAmount, Index: 2, Value: "3"}, initial.Lines, 1},
		{"stale picker closes", splitdraft.Action{Kind: splitdraft.PickCategory, Index: 2, Value: "c"}, initial.Lines, -1},
		{"unknown", splitdraft.Action{Kind: 255}, initial.Lines, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]splitdraft.Line(nil), initial.Lines...)
			got := initial.Apply(tc.a, transaction(-300, "USD"))
			if !reflect.DeepEqual(got.Lines, tc.want) || got.PickerLine != tc.pick {
				t.Fatalf("transition = %+v, want %+v, picker %d", got, tc.want, tc.pick)
			}
			if !reflect.DeepEqual(initial.Lines, before) {
				t.Fatal("transition changed input backing array")
			}
		})
	}
	if got := (splitdraft.State{}).Apply(splitdraft.Action{Kind: splitdraft.Remove}, domain.Transaction{}); got.Lines != nil {
		t.Fatal("removing from nil must be a no-op")
	}
}

func TestModeConversions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		total       int64
		code        string
		percent     bool
		action      splitdraft.ActionKind
		input, want []string
		wantPct     bool
	}{
		{"exact amount ties", -3, "USD", false, splitdraft.ToPercents, []string{"0.01", "0.01", "0.01"}, []string{"33.34", "33.33", "33.33"}, true},
		{"exact percent ties", -100, "USD", true, splitdraft.ToAmounts, []string{"33.34", "33.33", "33.33"}, []string{"0.34", "0.33", "0.33"}, false},
		{"unbalanced amounts", -300, "USD", false, splitdraft.ToPercents, []string{"1", ""}, []string{"33.33", ""}, true},
		{"unbalanced percents", -300, "USD", true, splitdraft.ToAmounts, []string{"50", ""}, []string{"1.50", ""}, false},
		{"invalid left alone", -100, "USD", false, splitdraft.ToPercents, []string{"bad", "-1", "0", "0.50"}, []string{"bad", "-1", "0", "50.00"}, true},
		{"all blank", -100, "USD", false, splitdraft.ToPercents, []string{"", ""}, []string{"", ""}, true},
		{"all invalid", -100, "USD", false, splitdraft.ToPercents, []string{"bad"}, []string{"bad"}, true},
		{"zero parent", 0, "USD", false, splitdraft.ToPercents, []string{"1"}, []string{"1"}, false},
		{"minimum parent", math.MinInt64, "USD", false, splitdraft.ToPercents, []string{"1"}, []string{"1"}, false},
		{"same amounts", 100, "USD", false, splitdraft.ToAmounts, []string{"1"}, []string{"1"}, false},
		{"same percents", 100, "USD", true, splitdraft.ToPercents, []string{"50"}, []string{"50"}, true},
		{"JPY exact", 3, "JPY", false, splitdraft.ToPercents, []string{"1", "2"}, []string{"33.33", "66.67"}, true},
		{"large exact safe product", math.MaxInt64, "JPY", false, splitdraft.ToPercents, []string{"9223372036854775806", "1"}, []string{"100.00", "0.00"}, true},
		{"sum overflow refused", 1, "JPY", false, splitdraft.ToPercents, []string{"9223372036854775807", "1"}, []string{"9223372036854775807", "1"}, false},
		{"result overflow refused", 1, "JPY", false, splitdraft.ToPercents, []string{"9223372036854775807"}, []string{"9223372036854775807"}, false},
		{"wide percent product", math.MaxInt64, "JPY", true, splitdraft.ToAmounts, []string{"50", "50"}, []string{"4611686018427387904", "4611686018427387903"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := splitdraft.State{Percent: tc.percent}
			for _, value := range tc.input {
				s.Lines = append(s.Lines, splitdraft.Line{Cat: "a", Amt: value, Owner: "m"})
			}
			got := s.Apply(splitdraft.Action{Kind: tc.action}, transaction(tc.total, tc.code))
			if got.Percent != tc.wantPct {
				t.Fatalf("mode = %v, want %v", got.Percent, tc.wantPct)
			}
			for i, want := range tc.want {
				if got.Lines[i].Amt != want || got.Lines[i].Owner != "m" || s.Lines[i].Amt != tc.input[i] {
					t.Errorf("line %d = %+v, want %q; input = %+v", i, got.Lines[i], want, s.Lines[i])
				}
			}
		})
	}
}

func TestAttemptSave(t *testing.T) {
	for _, tc := range []struct {
		name    string
		total   int64
		percent bool
		lines   []splitdraft.Line
		key     string
	}{
		{"empty", -100, false, nil, "splitEditor.needTwo"},
		{"one", -100, false, []splitdraft.Line{{Cat: "a", Amt: "1"}}, "splitEditor.needTwo"},
		{"blank amount skipped", -100, false, []splitdraft.Line{{Cat: "a"}, {Amt: "1"}}, "splitEditor.needTwo"},
		{"bad", -100, false, []splitdraft.Line{{Cat: "a", Amt: "bad"}}, "splitEditor.badAmount"},
		{"zero", -100, false, []splitdraft.Line{{Cat: "a", Amt: "0"}}, "splitEditor.badAmount"},
		{"negative", -100, false, []splitdraft.Line{{Cat: "a", Amt: "-1"}}, "splitEditor.badAmount"},
		{"oversize input", -100, false, []splitdraft.Line{{Cat: "a", Amt: strings.Repeat("9", 10000)}}, "splitEditor.badAmount"},
		{"bad percent", -100, true, []splitdraft.Line{{Cat: "a", Amt: "bad"}}, "splitEditor.badPercent"},
		{"unbalanced amount", -100, false, []splitdraft.Line{{Cat: "a", Amt: "1"}, {Cat: "b", Amt: "1"}}, "splitEditor.mustBalance"},
		{"unbalanced percent", -100, true, []splitdraft.Line{{Cat: "a", Amt: "30"}, {Cat: "b", Amt: "30"}}, "splitEditor.pctMustBalance"},
		{"too small", -1, true, []splitdraft.Line{{Cat: "a", Amt: "50"}, {Cat: "b", Amt: "50"}}, "splitEditor.pctTooSmall"},
		{"zero parent", 0, true, []splitdraft.Line{{Cat: "a", Amt: "50"}, {Cat: "b", Amt: "50"}}, "splitEditor.pctTooSmall"},
		{"amount sum overflow", -100, false, []splitdraft.Line{{Cat: "a", Amt: "92233720368547758.07"}, {Cat: "b", Amt: "0.01"}}, "splitEditor.mustBalance"},
		{"percent sum overflow", -100, true, []splitdraft.Line{{Cat: "a", Amt: "92233720368547758.07"}, {Cat: "b", Amt: "0.01"}}, "splitEditor.pctMustBalance"},
		{"unrepresentable magnitude", math.MinInt64, false, []splitdraft.Line{{Cat: "a", Amt: "1"}, {Cat: "b", Amt: "1"}}, "splitEditor.badAmount"},
		{"expense", -100, false, []splitdraft.Line{{Cat: "a", Amt: "0.25", Owner: "m"}, {Cat: "b", Amt: "0.75"}}, ""},
		{"income", 100, false, []splitdraft.Line{{Cat: "a", Amt: "0.25"}, {Cat: "b", Amt: "0.75"}}, ""},
		{"percentage", -101, true, []splitdraft.Line{{Cat: "a", Amt: "50"}, {Cat: "b", Amt: "50"}}, ""},
		// Preserve the existing save contract: incomplete rows are skipped, even
		// though their presence is reported separately by the status view.
		{"incomplete ignored", -100, false, []splitdraft.Line{{Cat: "a", Amt: "0.25"}, {Cat: "b", Amt: "0.75"}, {Amt: "1"}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := transaction(tc.total, "USD")
			tx.Date = time.Date(2026, time.October, 31, 23, 59, 0, 0, time.FixedZone("west", -7*60*60))
			s := splitdraft.State{Lines: tc.lines, Percent: tc.percent, ErrorKey: "old"}
			next, built, ok := s.AttemptSave(tx, false)
			if next.ErrorKey != tc.key || ok != (tc.key == "") || !next.SaveAttempted {
				t.Fatalf("save = %+v, %v, want key %q", next, ok, tc.key)
			}
			if !built.Date.Equal(tx.Date) || built.Date.Location() != tx.Date.Location() || built.MemberID != tx.MemberID || built.ID != tx.ID {
				t.Fatal("save changed unrelated transaction fields or time zone")
			}
			if ok {
				var sum int64
				for i, line := range built.Splits {
					sum += line.Amount.Amount
					if line.Amount.Currency != "USD" || line.MemberID != tc.lines[i].Owner || line.CategoryID != tc.lines[i].Cat {
						t.Errorf("saved line = %+v", line)
					}
				}
				if sum != tc.total || tx.Splits != nil {
					t.Fatalf("saved sum = %d, input splits = %+v", sum, tx.Splits)
				}
			} else if !reflect.DeepEqual(built, tx) {
				t.Fatal("failed save returned modified transaction")
			}
			pending, original, ok := s.AttemptSave(tx, true)
			if ok || !reflect.DeepEqual(pending, s) || !reflect.DeepEqual(original, tx) {
				t.Fatal("pending save changed state or proposed a write")
			}
		})
	}
}

func TestClear(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "pending"}[pending], func(t *testing.T) {
			tx := transaction(-100, "USD")
			tx.Splits = []domain.CategorySplit{{CategoryID: "a", Amount: money.New(-100, "USD")}}
			s := splitdraft.New(tx, 1)
			s.ErrorKey = "old"
			next, built, ok := s.Clear(tx, pending)
			if ok == pending || !reflect.DeepEqual(next.Lines, s.Lines) || len(tx.Splits) != 1 {
				t.Fatal("clear mutated inputs or ignored pending")
			}
			if pending && (next.ErrorKey != "old" || !reflect.DeepEqual(tx, built)) {
				t.Fatal("pending clear changed state")
			}
			if !pending && (built.Splits != nil || next.ErrorKey != "") {
				t.Fatal("clear did not remove splits/error")
			}
		})
	}
}

func TestSavePreservesCalendarAndCustomFields(t *testing.T) {
	for _, tc := range []struct {
		name   string
		date   time.Time
		custom map[string]any
	}{
		{"leap month end and nil map", time.Date(2024, time.February, 29, 23, 59, 0, 0, time.UTC), nil},
		{"month end east of UTC", time.Date(2026, time.January, 31, 0, 1, 0, 0, time.FixedZone("east", 14*60*60)), map[string]any{"receipt": "kept"}},
		{"DST boundary west of UTC", time.Date(2026, time.November, 1, 1, 30, 0, 0, time.FixedZone("west", -7*60*60)), map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := transaction(-300, "JPY")
			tx.Date, tx.Custom, tx.Tags = tc.date, tc.custom, []string{"tag"}
			s := splitdraft.State{Lines: []splitdraft.Line{{Cat: "a", Amt: "100"}, {Cat: "b", Amt: "200"}}}
			_, built, ok := s.AttemptSave(tx, false)
			if !ok {
				t.Fatal("valid JPY draft refused")
			}
			built.Splits = nil
			if !reflect.DeepEqual(built, tx) {
				t.Fatalf("save changed parent metadata: %+v", built)
			}
		})
	}
}

func TestLargeDraft(t *testing.T) {
	s := splitdraft.State{PickerLine: -1}
	for range 100000 {
		s.Lines = append(s.Lines, splitdraft.Line{Cat: "a", Amt: "0.01"})
	}
	tx := transaction(-100000, "USD")
	if v := view(s, tx); !v.CanSave || v.Total != 100000 || v.Shape.Complete != 100000 {
		t.Fatalf("large draft = %+v", v)
	}
	_, built, ok := s.AttemptSave(tx, false)
	if !ok || len(built.Splits) != len(s.Lines) {
		t.Fatal("large draft did not save")
	}
}
