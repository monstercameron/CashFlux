package txntable

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"reflect"
	"testing"
)

func TestEqualAndRetainRows(t *testing.T) {
	type value struct {
		ID      string
		Values  []int
		OnClick func()
	}
	a := value{ID: "a", Values: []int{1}, OnClick: func() {}}
	b := value{ID: "a", Values: []int{1}, OnClick: func() {}}
	if !EqualRowValues(a, b) || !EqualRowValues(2, 2) || EqualRowValues(2, 3) {
		t.Fatal("equality")
	}
	b.Values = []int{2}
	if EqualRowValues(a, b) {
		t.Fatal("changed data")
	}
	if a.OnClick == nil || b.OnClick == nil {
		t.Fatal("mutated callbacks")
	}
	first, rows, order := RetainRow(nil, nil, "a", a, 2)
	same, rows, order := RetainRow(rows, order, "a", a, 2)
	if same != first {
		t.Fatal("lost identity")
	}
	changed, rows, order := RetainRow(rows, order, "a", b, 2)
	if changed == first || !reflect.DeepEqual(order, []string{"a"}) {
		t.Fatal("unchanged")
	}
	for i := 0; i < 300; i++ {
		_, rows, order = RetainRow(rows, order, fmt.Sprint(i), a, 2)
	}
	if len(rows) != 2 || len(order) != 2 || rows["a"] != nil {
		t.Fatal("unbounded")
	}
	_, rows, order = RetainRow(rows, order, "z", a, -1)
	if len(rows) != 0 || len(order) != 0 {
		t.Fatal("negative limit")
	}
	rows = map[string]*value{"a": nil}
	if got, _, _ := RetainRow(rows, []string{"a"}, "a", a, 2); got == nil {
		t.Fatal("nil entry")
	}
}

func TestMenuDecisions(t *testing.T) {
	for mask := 0; mask < 512; mask++ {
		s := MenuState{Automatic: mask&1 != 0, Transfer: mask&2 != 0, Split: mask&4 != 0, Excluded: mask&8 != 0, Income: mask&16 != 0, Refund: mask&32 != 0, Refunded: mask&64 != 0, BillMatched: mask&128 != 0, GroupSize: mask % 3,
			Confirm: mask&256 != 0, Suggest: true, OpenSplit: true, ToggleExclude: true, Delete: true, OpenLink: true, PairRefund: true, Unpair: true, Ungroup: true, UnlinkBill: true}
		got := DecideMenu(s)
		want := Menu{Confirm: s.Automatic && s.Confirm, Suggest: !s.Transfer, Split: !s.Transfer, Exclude: true, Delete: true, Link: true, Pair: s.Income && !s.Refund && !s.Transfer, Unpair: s.Refund || s.Refunded, Ungroup: s.GroupSize > 1, UnlinkBill: s.BillMatched, SplitChecked: s.Split, Include: s.Excluded}
		if got != want {
			t.Fatalf("%+v / %+v", got, want)
		}
	}
	if DecideMenu(MenuState{}) != (Menu{}) {
		t.Fatal("nil callbacks")
	}
}

func TestReadFrameAndSuggestionEligibility(t *testing.T) {
	c := ReadFrame(domain.NewFrame(domain.Field{Name: "id", Values: []any{"a", "b"}}, domain.Field{Name: "date", Values: []any{int64(1790812800)}}, domain.Field{Name: "amount", Values: []any{int64(-123)}}, domain.Field{Name: "currency", Values: []any{"USD"}}, domain.Field{Name: "cleared", Values: []any{true, "wrong"}}))
	got := c.RowAt(0)
	if got.ID != "a" || got.Amount != money.New(-123, "USD") || !got.Cleared || got.Date.Location().String() != "UTC" {
		t.Fatal(got)
	}
	for _, i := range []int{-1, 1, 99} {
		if c.RowAt(i).Cleared {
			t.Fatal(i)
		}
	}
	if i, ok := c.IndexOf("b"); i != 1 || !ok {
		t.Fatal(i, ok)
	}
	if _, ok := c.IndexOf("missing"); ok {
		t.Fatal("missing")
	}
	if ReadFrame(domain.Frame{}).RowAt(0).ID != "" {
		t.Fatal("empty frame")
	}
	for _, tt := range []struct {
		row                  domain.Transaction
		ready, enabled, want bool
	}{
		{domain.Transaction{}, true, true, true}, {domain.Transaction{}, false, true, false}, {domain.Transaction{}, true, false, false},
		{domain.Transaction{CategoryID: "c"}, true, true, false}, {domain.Transaction{TransferAccountID: "b"}, true, true, false},
		{domain.Transaction{Splits: []domain.CategorySplit{{Amount: money.New(1, "USD")}, {Amount: money.New(1, "USD")}}}, true, true, false},
	} {
		if SuggestionEligible(tt.row, tt.ready, tt.enabled) != tt.want {
			t.Fatal(tt)
		}
	}
}
