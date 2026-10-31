// SPDX-License-Identifier: MIT

package acctproject

import (
	"math"
	"testing"
	"time"
)

func TestProjectionOutlook(t *testing.T) {
	asOf := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	day := func(n int) time.Time { return asOf.AddDate(0, 0, n) }
	cases := []struct {
		name     string
		start    int64
		balance  int64
		drivers  []Driver
		wantKind OutlookKind
		wantDay  time.Time
	}{
		{"income only rises", 1000, 1000, []Driver{{Label: "pay", Date: day(5), Amount: 500}}, OutlookRising, time.Time{}},
		{"a bill dips but stays positive", 1000, 1000, []Driver{{Label: "rent", Date: day(5), Amount: -400}, {Label: "pay", Date: day(10), Amount: 500}}, OutlookLow, time.Time{}},
		{"a bill past zero runs out on that day", 300, 300, []Driver{{Label: "rent", Date: day(7), Amount: -400}, {Label: "pay", Date: day(20), Amount: 900}}, OutlookNegative, day(7)},
		{"already overdrawn counts as today", -50, -50, nil, OutlookNegative, asOf},
		{"low is measured against the projection start, not the supplied balance", 1000, 2000, nil, OutlookRising, time.Time{}},
		{"no drivers and matching balance is steady", 1000, 1000, nil, OutlookRising, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Project(tc.start, asOf, tc.drivers, 90)
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.Outlook(tc.balance, asOf)
			if err != nil {
				t.Fatal(err)
			}
			if got.Kind != tc.wantKind || !got.NegativeOn.Equal(tc.wantDay) {
				t.Fatalf("Outlook = %+v, want kind %d on %v", got, tc.wantKind, tc.wantDay)
			}
		})
	}
}

func TestProjectionOutlookReportsOverflow(t *testing.T) {
	asOf := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	p := Projection{Start: math.MaxInt64 - 1, Low: 0, Drivers: []Driver{{Label: "windfall", Date: asOf, Amount: 10}}}
	if _, err := p.Outlook(0, asOf); err == nil {
		t.Fatal("expected the overflow to surface as an error")
	}
}
