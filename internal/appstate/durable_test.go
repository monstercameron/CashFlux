// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"io"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestDurableCompletionIsExplicitAndRevisionBound(t *testing.T) {
	for _, scenario := range []string{"unavailable", "delayed", "failure", "stale", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			a, err := New(io.Discard, false)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Store().Close()
			if err := a.Store().PutTransaction(domain.Transaction{ID: "row"}); err != nil {
				t.Fatal(err)
			}
			requested := a.Rev()
			var complete func(DurableResult)
			if scenario != "unavailable" {
				a.SetDurableWriter(func(revision uint64, done func(DurableResult)) {
					if revision != requested {
						t.Fatal("wrong revision")
					}
					complete = done
				})
			}
			calls := 0
			var result DurableResult
			a.RequestDurable(func(r DurableResult) { calls++; result = r })
			if scenario == "unavailable" {
				if calls != 1 || !errors.Is(result.Err, ErrDurabilityUnavailable) {
					t.Fatalf("unavailable writer acknowledged: %+v", result)
				}
				return
			}
			if calls != 0 {
				t.Fatal("request itself acknowledged a write")
			}
			r := DurableResult{Revision: requested}
			switch scenario {
			case "failure":
				r.Err = errors.New("strict commit rejected")
			case "stale":
				r.Revision = requested - 1
			}
			complete(r)
			if calls != 1 {
				t.Fatal("completion missing")
			}
			if (scenario == "failure" || scenario == "stale") != (result.Err != nil) {
				t.Fatalf("unexpected result: %+v", result)
			}
			if scenario == "duplicate" {
				complete(r)
				if calls != 1 {
					t.Fatal("completed twice")
				}
			}
		})
	}
}
