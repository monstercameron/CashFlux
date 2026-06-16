package spendsummary

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/extract"
)

func TestSummarizeBucketsAndTotals(t *testing.T) {
	rows := []extract.Row{
		{Date: "2026-01-05", Amount: "-12.50"},  // Jan spend 12.50
		{Date: "2026-01-20", Amount: "-7.50"},   // Jan spend 7.50
		{Date: "2026-01-31", Amount: "1000.00"}, // Jan income 1000
		{Date: "2026-02-02", Amount: "-3.00"},   // Feb spend 3.00
	}
	got := Summarize(rows, 2)
	if len(got) != 2 {
		t.Fatalf("months = %d, want 2: %+v", len(got), got)
	}
	jan := got[0]
	if jan.Month != "2026-01" || jan.Count != 3 || jan.Out != 2000 || jan.In != 100000 {
		t.Errorf("Jan = %+v, want 2026-01 count3 out2000 in100000", jan)
	}
	if jan.Net() != 100000-2000 {
		t.Errorf("Jan.Net = %d, want %d", jan.Net(), 100000-2000)
	}
	feb := got[1]
	if feb.Month != "2026-02" || feb.Out != 300 || feb.In != 0 {
		t.Errorf("Feb = %+v, want 2026-02 out300 in0", feb)
	}
}

func TestSummarizeAscendingOrder(t *testing.T) {
	rows := []extract.Row{
		{Date: "2026-03-01", Amount: "-1.00"},
		{Date: "2026-01-01", Amount: "-1.00"},
		{Date: "2026-02-01", Amount: "-1.00"},
	}
	got := Summarize(rows, 2)
	want := []string{"2026-01", "2026-02", "2026-03"}
	for i, w := range want {
		if got[i].Month != w {
			t.Errorf("month[%d] = %s, want %s", i, got[i].Month, w)
		}
	}
}

func TestSummarizeToleratesDateFormats(t *testing.T) {
	rows := []extract.Row{
		{Date: "01/15/2026", Amount: "-5.00"},
		{Date: "2026/01/16", Amount: "-5.00"},
		{Date: "Jan 17, 2026", Amount: "-5.00"},
		{Date: "17 Jan 2026", Amount: "-5.00"},
	}
	got := Summarize(rows, 2)
	if len(got) != 1 || got[0].Month != "2026-01" || got[0].Count != 4 || got[0].Out != 2000 {
		t.Fatalf("mixed formats = %+v, want one 2026-01 month, count4 out2000", got)
	}
}

func TestSummarizeUndatedAndUnparsableAmounts(t *testing.T) {
	rows := []extract.Row{
		{Date: "2026-01-05", Amount: "-10.00"},
		{Date: "not a date", Amount: "-99.00"},  // undated -> empty-month bucket, sorts last
		{Date: "2026-01-06", Amount: "garbage"}, // counted, but adds nothing
	}
	got := Summarize(rows, 2)
	if len(got) != 2 {
		t.Fatalf("buckets = %d, want 2: %+v", len(got), got)
	}
	// Dated month first, undated (empty Month) last.
	if got[0].Month != "2026-01" || got[0].Count != 2 || got[0].Out != 1000 {
		t.Errorf("dated = %+v, want 2026-01 count2 out1000", got[0])
	}
	if got[1].Month != "" || got[1].Count != 1 || got[1].Out != 9900 {
		t.Errorf("undated = %+v, want empty-month count1 out9900", got[1])
	}
}

func TestSummarizeCleansCurrencySymbols(t *testing.T) {
	rows := []extract.Row{
		{Date: "2026-01-01", Amount: "-$1,234.56"},
		{Date: "2026-01-02", Amount: "$2,000.00"},
	}
	got := Summarize(rows, 2)
	if got[0].Out != 123456 || got[0].In != 200000 {
		t.Errorf("cleaned = %+v, want out123456 in200000", got[0])
	}
}

func TestSummarizeEmpty(t *testing.T) {
	if got := Summarize(nil, 2); len(got) != 0 {
		t.Errorf("empty input = %+v, want no months", got)
	}
}
