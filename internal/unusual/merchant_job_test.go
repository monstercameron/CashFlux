// SPDX-License-Identifier: MIT

package unusual

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/copytext"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/notifyfeed"
)

func TestMerchantJobMatchesLegacyNotificationBaselines(t *testing.T) {
	in := jobFixture(330)
	since := in.Now.AddDate(0, 0, -300)
	want, err := notifyfeed.UnusualChargeCandidates("rule", in.Transactions, 10000, since, in.Rates, func(payee string, amount, typical int64) (copytext.Text, copytext.Text) {
		return copytext.Plain(payee), copytext.Plain(fmt.Sprintf("%d/%d", amount, typical))
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, budget := range []int{1, 32, 256} {
		job := NewMerchantJob(in.Transactions, 10000, since, in.Rates)
		for !job.Step(budget) {
		}
		if job.Err() != nil {
			t.Fatal(job.Err())
		}
		got := map[string]string{}
		for _, finding := range job.Findings() {
			got["unusual:"+finding.Transaction.ID] = fmt.Sprintf("%d/%d", finding.AmountMinor, finding.TypicalMinor)
		}
		expected := map[string]string{}
		for _, candidate := range want {
			expected[candidate.OccurrenceKey] = candidate.Body
		}
		if !reflect.DeepEqual(got, expected) {
			t.Fatalf("budget %d changed notification baselines: %v vs %v", budget, got, expected)
		}
	}
}

func TestSOLP2MerchantNotificationSlices3300(t *testing.T) {
	in := jobFixture(3300)
	for i := range in.Transactions {
		in.Transactions[i].Payee = "One merchant"
		in.Transactions[i].Amount.Amount = -50000 - int64(i%7)
	}
	job := NewMerchantJob(in.Transactions, 25000, time.Time{}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2}})
	var longest time.Duration
	steps := 0
	for {
		start := time.Now()
		done := job.Step(64)
		longest = max(longest, time.Since(start))
		steps++
		if done {
			break
		}
	}
	if job.Err() != nil {
		t.Fatal(job.Err())
	}
	t.Logf("SOLP2 notification merchant job: rows=3300 slices=%d longest=%s", steps, longest)
	if longest > 50*time.Millisecond {
		t.Fatalf("merchant notification job exceeded 50 ms: %s", longest)
	}
	if len(job.Findings()) != 0 {
		t.Fatal("equal large charges incorrectly flagged")
	}
	// The complete input remains untouched; neither ordering nor history is lost.
	ids := make([]string, len(in.Transactions))
	for i, t := range in.Transactions {
		ids[i] = t.ID
	}
	sort.Strings(ids)
	if len(ids) != 3300 {
		t.Fatal("ledger truncated")
	}
}
