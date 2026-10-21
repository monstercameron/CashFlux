// SPDX-License-Identifier: MIT

package credithealth_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

type frozenHistoryPoint struct {
	id      string
	at      time.Time
	percent int64
}

// frozenHistoryPreparation is the previous store-read chronological insertion
// sort plus latest-eight ratio calculation. Display formatting and storage I/O
// are excluded from both measured native paths. Fixtures have integral ratios.
func frozenHistoryPreparation(all []domain.BalanceSnapshot, limit int64) []frozenHistoryPoint {
	snapshots := append([]domain.BalanceSnapshot(nil), all...)
	for i := 1; i < len(snapshots); i++ {
		for j := i; j > 0 && snapshots[j].AsOf.Before(snapshots[j-1].AsOf); j-- {
			snapshots[j], snapshots[j-1] = snapshots[j-1], snapshots[j]
		}
	}
	if len(snapshots) < 2 {
		return nil
	}
	if len(snapshots) > 8 {
		snapshots = snapshots[len(snapshots)-8:]
	}
	points := make([]frozenHistoryPoint, 0, len(snapshots))
	for _, s := range snapshots {
		owed := s.BalanceMinor
		if owed < 0 {
			owed = -owed
		}
		points = append(points, frozenHistoryPoint{s.ID, s.AsOf, owed * 100 / limit})
	}
	return points
}

var benchmarkHistory credithealth.UtilizationHistory
var benchmarkFrozen []frozenHistoryPoint

func BenchmarkUtilizationHistoryPreparation(b *testing.B) {
	for _, n := range []int{32, 512, 4096} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			account := checkedCreditCard("card", "USD", 100000)
			snapshots := make([]domain.BalanceSnapshot, n)
			for i := range snapshots {
				snapshots[i] = domain.BalanceSnapshot{ID: fmt.Sprintf("%08d", i), AccountID: "card", Currency: "USD", BalanceMinor: -int64(i%150+1) * 1000, AsOf: baseNow.AddDate(0, 0, (i*7919)%n)}
			}
			rates := currency.Rates{Base: "USD"}
			old := frozenHistoryPreparation(snapshots, account.CreditLimit.Amount)
			current := credithealth.BuildUtilizationHistory(account, snapshots, rates)
			if current.Err != nil || len(old) != len(current.Points) {
				b.Fatalf("reference failed: %v", current.Err)
			}
			for i, p := range old {
				if p.id != current.Points[i].ID || !p.at.Equal(current.Points[i].AsOf) || p.percent != current.Points[i].UtilPct {
					b.Fatal("frozen reference parity failed")
				}
			}
			b.Run("frozen", func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					benchmarkFrozen = frozenHistoryPreparation(snapshots, account.CreditLimit.Amount)
				}
			})
			b.Run("checked", func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					benchmarkHistory = credithealth.BuildUtilizationHistory(account, snapshots, rates)
				}
			})
		})
	}
}
