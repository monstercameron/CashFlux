// SPDX-License-Identifier: MIT

package unusual

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func jobFixture(n int) Input {
	in := Input{Now: testNow, Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.2}}}
	for i := 0; i < n; i++ {
		t := txn(fmt.Sprint(i), fmt.Sprintf("Merchant %d", i%19), fmt.Sprintf("category %d", i%5), int64(1000+(i*37)%8000), i%700)
		t.AccountID = fmt.Sprintf("account %d", i%3)
		if i%17 == 0 {
			t.Amount = money.New(-45000, "USD")
		}
		if i%23 == 0 {
			t.Amount.Currency = "EUR"
		}
		if i%37 == 0 {
			t.ExcludeFromReports = true
		}
		if i%47 == 0 {
			t.GoalID = "goal"
		}
		in.Transactions = append(in.Transactions, t)
	}
	in.Goals = []domain.Goal{{ID: "goal", Name: "Trip"}}
	return in
}

func TestJobMatchesReferenceForEveryStepBudget(t *testing.T) {
	for _, n := range []int{0, 1, 8, 80, 330} {
		in := jobFixture(n)
		want := referenceDetect(in)
		for _, budget := range []int{1, 7, 32, 257} {
			t.Run(fmt.Sprintf("rows%d/work%d", n, budget), func(t *testing.T) {
				job := NewJob(in)
				if job.Step(0) || job.Findings() != nil {
					t.Fatal("empty budget performed work")
				}
				for !job.Step(budget) {
					if job.Findings() != nil {
						t.Fatal("published incomplete findings")
					}
				}
				if got := job.Findings(); !reflect.DeepEqual(got, want) {
					t.Fatalf("job differs from reference: got %+v want %+v", got, want)
				}
				if !job.Step(budget) || !reflect.DeepEqual(job.Findings(), want) {
					t.Fatal("completed job changed")
				}
			})
		}
	}
}

func TestSOLP2UnusualSlices3300(t *testing.T) {
	in := jobFixture(3300)
	job := NewJob(in)
	var longest time.Duration
	steps := 0
	for {
		start := time.Now()
		done := job.Step(32)
		longest = max(longest, time.Since(start))
		steps++
		if done {
			break
		}
	}
	if got, want := job.Findings(), referenceDetect(in); !reflect.DeepEqual(got, want) {
		t.Fatal("3300-row findings/evidence differ from the original detector")
	}
	t.Logf("SOLP2 unusual: rows=3300 slices=%d longest=%s findings=%d", steps, longest, len(job.Findings()))
	if longest > 50*time.Millisecond {
		t.Fatalf("detector slice exceeded 50 ms: %s", longest)
	}
}

func TestSOLP3UnusualSlices10k(t *testing.T) {
	in := jobFixture(10000)
	job := NewJob(in)
	if job.Step(0) || job.Findings() != nil {
		t.Fatal("empty budget performed work")
	}
	steps := 0
	for {
		done := job.Step(32)
		steps++
		if !done && job.Findings() != nil {
			t.Fatal("published incomplete findings")
		}
		if done {
			break
		}
	}
	if steps < len(in.Transactions)/32 {
		t.Fatalf("10k rows completed in only %d slices; want incremental progress", steps)
	}
}
