// SPDX-License-Identifier: MIT

package orderimport

import (
	"testing"
	"time"
)

func TestParseMoneyMinor(t *testing.T) {
	cases := map[string]struct {
		want int64
		ok   bool
	}{
		"$45.99":    {4599, true},
		"1,234.56":  {123456, true},
		"$3":        {300, true},
		"-2.00":     {-200, true},
		"(2.50)":    {-250, true},
		"USD 10.10": {1010, true},
		"":          {0, false},
		"free":      {0, false},
	}
	for in, exp := range cases {
		got, ok := parseMoneyMinor(in)
		if ok != exp.ok || (ok && got != exp.want) {
			t.Errorf("parseMoneyMinor(%q) = %d,%v want %d,%v", in, got, ok, exp.want, exp.ok)
		}
	}
}

func TestParseRetailCSVFuzzyHeaders(t *testing.T) {
	// Column names drift and are reordered; the parser maps by fuzzy header.
	csv := "Order Date,Order ID,Product Name,Quantity,Unit Price,Total Owed,Currency\n" +
		"2026-07-01,111-2223334,USB Cable,2,5.99,17.98,USD\n" +
		"2026-07-01,111-2223334,Phone Case,1,6.00,17.98,USD\n" +
		"2026-07-03,111-9998887,Coffee Beans,1,14.50,14.50,USD\n"
	orders, err := ParseRetailCSV(csv, "USD")
	if err != nil {
		t.Fatalf("ParseRetailCSV: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("want 2 orders, got %d: %+v", len(orders), orders)
	}
	o := orders[0]
	if o.ID != "111-2223334" || o.TotalMinor != 1798 || o.Currency != "USD" {
		t.Errorf("order0 = %+v", o)
	}
	if len(o.Items) != 2 || o.Items[0].Name != "USB Cable" || o.Items[0].Qty != 2 || o.Items[0].UnitMinor != 599 {
		t.Errorf("order0 items = %+v", o.Items)
	}
	if o.ItemsSubtotalMinor() != 599*2+600 {
		t.Errorf("subtotal = %d", o.ItemsSubtotalMinor())
	}
	if !o.Date.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date = %v", o.Date)
	}
}

func TestParseRetailCSVNoHeader(t *testing.T) {
	if got, err := ParseRetailCSV("just one line", "USD"); err != nil || got != nil {
		t.Fatalf("want nil,nil got %+v,%v", got, err)
	}
	// Header with no order-id column → nothing dependable to group on.
	if got, _ := ParseRetailCSV("Name,Price\nThing,1.00\n", "USD"); got != nil {
		t.Fatalf("want nil for no order-id column, got %+v", got)
	}
}

func TestParseOrdersPaste(t *testing.T) {
	paste := `Your Orders
ORDER PLACED July 1, 2026
TOTAL $23.98
Order # 111-5556667
USB-C Charging Cable 6ft
Delivered July 3
Bluetooth Speaker
Buy it again

ORDER PLACED July 5, 2026
TOTAL $14.50
Order # 111-1112223
Whole Bean Coffee 2lb
`
	orders := ParseOrdersPaste(paste, "USD")
	if len(orders) != 2 {
		t.Fatalf("want 2 orders, got %d: %+v", len(orders), orders)
	}
	if orders[0].ID != "111-5556667" || orders[0].TotalMinor != 2398 {
		t.Errorf("order0 = %+v", orders[0])
	}
	// "Delivered" and "Buy it again" lines are metadata, not items.
	names := []string{}
	for _, it := range orders[0].Items {
		names = append(names, it.Name)
	}
	if len(names) != 2 || names[0] != "USB-C Charging Cable 6ft" || names[1] != "Bluetooth Speaker" {
		t.Errorf("order0 items = %v", names)
	}
	if orders[1].TotalMinor != 1450 {
		t.Errorf("order1 total = %d", orders[1].TotalMinor)
	}
}

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestMatchOrderSingle(t *testing.T) {
	o := Order{ID: "o1", Date: d(2026, 7, 1), TotalMinor: 4599, Currency: "USD"}
	charges := []Charge{
		{TxnID: "t1", Date: d(2026, 7, 2), AmountMinor: -4599, Currency: "USD"},
		{TxnID: "t2", Date: d(2026, 7, 2), AmountMinor: -1000, Currency: "USD"},
	}
	m := MatchOrder(o, charges, nil)
	if m.Kind != MatchSingle || len(m.TxnIDs) != 1 || m.TxnIDs[0] != "t1" || m.DriftMinor != 0 {
		t.Fatalf("match = %+v", m)
	}
}

func TestMatchOrderMultiShipment(t *testing.T) {
	o := Order{ID: "o1", Date: d(2026, 7, 1), TotalMinor: 5000, Currency: "USD"}
	charges := []Charge{
		{TxnID: "a", Date: d(2026, 7, 1), AmountMinor: -3000, Currency: "USD"},
		{TxnID: "b", Date: d(2026, 7, 2), AmountMinor: -2000, Currency: "USD"},
		{TxnID: "c", Date: d(2026, 7, 3), AmountMinor: -999, Currency: "USD"},
	}
	m := MatchOrder(o, charges, nil)
	if m.Kind != MatchMulti || len(m.TxnIDs) != 2 || m.MatchedMinor != 5000 {
		t.Fatalf("match = %+v", m)
	}
}

func TestMatchOrderGiftCardDrift(t *testing.T) {
	// A $50 order but only a $30 card charge — a gift card covered $20; the drift
	// is stated, not hidden.
	o := Order{ID: "o1", Date: d(2026, 7, 1), TotalMinor: 5000, Currency: "USD"}
	charges := []Charge{{TxnID: "t1", Date: d(2026, 7, 1), AmountMinor: -3000, Currency: "USD"}}
	m := MatchOrder(o, charges, nil)
	if m.Kind != MatchSingle || m.DriftMinor != 2000 {
		t.Fatalf("match = %+v (want single, drift 2000)", m)
	}
}

func TestMatchOrderNoneOutOfWindow(t *testing.T) {
	o := Order{ID: "o1", Date: d(2026, 7, 1), TotalMinor: 4599, Currency: "USD"}
	charges := []Charge{{TxnID: "t1", Date: d(2026, 7, 20), AmountMinor: -4599, Currency: "USD"}}
	if m := MatchOrder(o, charges, nil); m.Kind != MatchNone {
		t.Fatalf("out-of-window charge should not match: %+v", m)
	}
}

func TestMatchOrdersNoDoubleAssign(t *testing.T) {
	orders := []Order{
		{ID: "big", Date: d(2026, 7, 1), TotalMinor: 5000, Currency: "USD"},
		{ID: "small", Date: d(2026, 7, 1), TotalMinor: 3000, Currency: "USD"},
	}
	charges := []Charge{
		{TxnID: "a", Date: d(2026, 7, 1), AmountMinor: -3000, Currency: "USD"},
		{TxnID: "b", Date: d(2026, 7, 1), AmountMinor: -2000, Currency: "USD"},
	}
	res := MatchOrders(orders, charges)
	// The big order (subset a+b) claims both; small is left unmatched.
	if res[0].Kind != MatchMulti {
		t.Fatalf("big order = %+v", res[0])
	}
	if res[1].Kind != MatchNone {
		t.Fatalf("small order should be unmatched (its charge was consumed): %+v", res[1])
	}
}
