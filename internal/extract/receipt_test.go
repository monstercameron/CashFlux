// SPDX-License-Identifier: MIT

package extract

import "testing"

func rows(items ...[2]string) []Row {
	out := make([]Row, 0, len(items))
	for _, it := range items {
		out = append(out, Row{Description: it[0], Amount: it[1], Category: "Groceries"})
	}
	return out
}

func TestReceiptFromRowsDefaultsTotalToLineSum(t *testing.T) {
	r := ReceiptFromRows(rows([2]string{"Milk", "3.50"}, [2]string{"Bread", "2.25"}), "2026-06-01", "Costco", "", 2)
	if r.Total != "5.75" {
		t.Errorf("total = %q, want 5.75 (sum of lines)", r.Total)
	}
	if !r.Reconciles(2) {
		t.Errorf("a receipt built from its own lines should reconcile")
	}
	if r.Merchant != "Costco" || r.Date != "2026-06-01" {
		t.Errorf("metadata not carried: %+v", r)
	}
}

func TestReceiptResidual(t *testing.T) {
	tests := []struct {
		name     string
		total    string
		lines    [][2]string
		residual int64
		recon    bool
	}{
		{
			name:     "lines reconcile to the printed total",
			total:    "12.00",
			lines:    [][2]string{{"Produce", "7.00"}, {"Dairy", "5.00"}},
			residual: 0,
			recon:    true,
		},
		{
			name:     "lines fall short of the total (positive remainder)",
			total:    "20.00",
			lines:    [][2]string{{"Produce", "7.00"}, {"Dairy", "5.00"}},
			residual: 800, // $8.00 unassigned
			recon:    false,
		},
		{
			name:     "discount line nets the splits down to the total",
			total:    "8.00",
			lines:    [][2]string{{"Groceries", "10.00"}, {"Coupon", "-2.00"}},
			residual: 0,
			recon:    true,
		},
		{
			name:     "currency symbols and commas tolerated",
			total:    "$1,234.50",
			lines:    [][2]string{{"Big item", "$1,200.00"}, {"Small item", "$34.50"}},
			residual: 0,
			recon:    true,
		},
		{
			name:     "lines overshoot the total (negative remainder)",
			total:    "5.00",
			lines:    [][2]string{{"A", "4.00"}, {"B", "3.00"}},
			residual: -200,
			recon:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ReceiptFromRows(rows(tc.lines...), "", "", tc.total, 2)
			got, err := r.Residual(2)
			if err != nil {
				t.Fatalf("Residual: %v", err)
			}
			if got != tc.residual {
				t.Errorf("residual = %d, want %d", got, tc.residual)
			}
			if r.Reconciles(2) != tc.recon {
				t.Errorf("Reconciles = %v, want %v", r.Reconciles(2), tc.recon)
			}
		})
	}
}

func TestReceiptUnparsableAmountErrors(t *testing.T) {
	r := ReceiptFromRows(rows([2]string{"Mystery", "abc"}), "", "", "10.00", 2)
	if _, err := r.Residual(2); err == nil {
		t.Error("expected an error for an unparsable line amount")
	}
	if r.Reconciles(2) {
		t.Error("an unparsable receipt must not report as reconciled")
	}
}
