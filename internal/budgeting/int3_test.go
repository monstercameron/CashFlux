// SPDX-License-Identifier: MIT

package budgeting

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestINT3StatusCarriesViewedPeriodAndBase(t *testing.T) {
	for _, rollover := range []bool{false, true} {
		for _, period := range []domain.Period{domain.PeriodMonthly, domain.PeriodWeekly, domain.PeriodYearly} {
			t.Run(string(period)+map[bool]string{false: "/plain", true: "/rollover"}[rollover], func(t *testing.T) {
				in := viewTestInput()
				in.Period.Anchor, in.Period.Now = mustDate("2026-01-15"), mustDate("2026-10-15")
				b := cmtBudget(period, 60000)
				b.Rollover = rollover
				start, end := PeriodRangeAnchored(period, in.Period.Anchor, in.Period.WeekStart, time.Time{})
				b.RolloverStart = start
				b.LimitHistory = map[string]int64{"2026-01-01": 50000, "2026-10-01": 60000}
				got := EvaluateViewBudget(b, in)
				if !got.Valid {
					t.Fatalf("view: %+v", got.Error)
				}
				if !got.Status.Start.Equal(start) || !got.Status.End.Equal(end) {
					t.Fatalf("period = [%v,%v), want [%v,%v)", got.Status.Start, got.Status.End, start, end)
				}
				if got.Status.Base.Amount != 50000 || got.Status.Budget.Limit.Amount != 60000 {
					t.Fatalf("viewed base=%+v standing=%+v", got.Status.Base, got.Status.Budget.Limit)
				}
			})
		}
	}
}
