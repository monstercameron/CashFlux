package appstate

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
)

// TestIMV4CommitProfile measures the production staged import, SQL capture and
// ledger refresh independently. Run with -v to retain the timing table.
func TestIMV4CommitProfile(t *testing.T) {
	if os.Getenv("CASHFLUX_IMV4_PROFILE") == "" {
		t.Skip("set CASHFLUX_IMV4_PROFILE=1 for import timings")
	}
	for _, count := range []int{3, 300, 3000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			a := newApp(t, true)
			defer a.Store().Close()
			account := a.Accounts()[0]
			rows := make([]extract.Row, count)
			for i := range rows {
				rows[i] = extract.Row{ID: fmt.Sprintf("imv4-row-%d", i), Date: "2026-10-01", Description: fmt.Sprintf("IMV4 purchase %d", i), Amount: "-1.25"}
			}
			_ = a.Transactions()
			cursor, err := a.Store().ChangeCursor()
			if err != nil {
				t.Fatal(err)
			}
			start := time.Now()
			batch, err := a.Stage(domain.DocCSV, rows, account.ID)
			if err != nil {
				t.Fatal(err)
			}
			staged := time.Now()
			result, err := a.CommitImportBatch(batch, "imv4-run", "")
			if err != nil {
				t.Fatal(err)
			}
			committed := time.Now()
			if result.Imported != count {
				t.Fatalf("imported %d, want %d", result.Imported, count)
			}
			_ = a.Transactions()
			cached := time.Now()
			target, err := a.Store().ChangeCursor()
			if err != nil {
				t.Fatal(err)
			}
			captured := 0
			for cursor < target {
				page, err := a.Store().CaptureChanges(cursor, target, 64)
				if err != nil {
					t.Fatal(err)
				}
				captured += len(page.Changes)
				cursor = page.Through
			}
			t.Logf("IMV4 rows=%d stage=%s commit=%s cache=%s capture=%s transitions=%d", count, staged.Sub(start), committed.Sub(staged), cached.Sub(committed), time.Since(cached), captured)
		})
	}
}
