// SPDX-License-Identifier: MIT

package taxgather

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// BenchmarkNetIncomeWithLinks measures cold native preparation, not browser
// navigation or a warm cache. Fixtures mix FX, split lines and cross-window
// refunds; refund counterparts are additional to the named primary-row count.
func BenchmarkNetIncomeWithLinks(b *testing.B) {
	for _, count := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprintf("rows_%d", count), func(b *testing.B) {
			in := netIncomeBenchmarkInput(count)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := NetIncomeWithLinks(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func netIncomeBenchmarkInput(count int) NetIncomeInput {
	in := netIncomeFixture()
	in.From = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	in.To = in.From.AddDate(1, 0, 0)
	in.Rates = currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2, "JPY": 0.01}}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("row-%d", i)
		at := in.From.AddDate(0, 0, i%300)
		row := netIncomeRow(id, "", "USD", at, 10000)
		switch i % 5 {
		case 1:
			row = netIncomeRow(id, "ded", "EUR", at, -1000)
		case 2:
			row = netIncomeRow(id, "other", "USD", at, -1000)
			row.Splits = []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-600, "USD")}, {CategoryID: "other", Amount: money.New(-400, "USD")}}
		case 3:
			row = netIncomeRow(id, "other", "JPY", at, -1000)
		case 4:
			row = netIncomeRow(id, "ded", "USD", at, -500)
			refundID := id + "-refund"
			in.Transactions = append(in.Transactions, netIncomeRow(refundID, "ded", "USD", in.To, 100))
			in.Links = append(in.Links, domain.TxnLink{ID: id + "-link", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{id, refundID}})
		}
		in.Transactions = append(in.Transactions, row)
	}
	return in
}
