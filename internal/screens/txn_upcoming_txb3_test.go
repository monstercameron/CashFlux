// SPDX-License-Identifier: MIT

package screens

import (
	"testing"
	"time"
)

func TestTxnUpcomingSettlementDateWindow(t *testing.T) {
	monthStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		date time.Time
		want bool
	}{
		{name: "before current month", date: time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC), want: false},
		{name: "current month on or before today", date: time.Date(2026, time.October, 18, 0, 0, 0, 0, time.UTC), want: true},
		{name: "future date later this month", date: time.Date(2026, time.October, 25, 0, 0, 0, 0, time.UTC), want: false},
		{name: "future date next month", date: time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnUpcomingSettlementDate(tt.date, monthStart, today); got != tt.want {
				t.Errorf("txnUpcomingSettlementDate(%s) = %t, want %t", tt.date.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}
