// SPDX-License-Identifier: MIT

package householdview

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var benchmarkHousehold Model

// BenchmarkPrepare measures cold complete model preparation. It is not a UI or
// cache-hit benchmark; fixture construction is outside the measured operation.
func BenchmarkPrepare(b *testing.B) {
	for _, size := range []struct {
		name                            string
		accounts, transactions, members int
	}{{"Empty", 0, 0, 0}, {"20Accounts_2000Transactions_6Members", 20, 2000, 6}, {"100Accounts_10000Transactions_30Members", 100, 10000, 30}} {
		b.Run(size.name, func(b *testing.B) {
			start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
			in := Input{Rates: currency.Rates{Base: "USD"}, PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0)}
			for i := range size.members {
				in.Members = append(in.Members, domain.Member{ID: fmt.Sprintf("member-%03d", i), Name: fmt.Sprintf("Member %d", i)})
			}
			for i := range size.accounts {
				in.Accounts = append(in.Accounts, domain.Account{ID: fmt.Sprintf("account-%03d", i), OwnerID: in.Members[i%size.members].ID, Currency: "USD", OpeningBalance: money.New(100000, "USD")})
			}
			for i := range size.transactions {
				value := -int64(100 + i%500)
				if i%7 == 0 {
					value = -value
				}
				in.Transactions = append(in.Transactions, domain.Transaction{ID: fmt.Sprintf("txn-%05d", i), AccountID: in.Accounts[i%size.accounts].ID, Date: start.AddDate(0, 0, i%30), MemberID: in.Members[i%size.members].ID, Amount: money.New(value, "USD")})
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				benchmarkHousehold = Prepare(in)
			}
		})
	}
}
