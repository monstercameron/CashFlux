//go:build js && wasm

package app

import (
	"flag"
	"fmt"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
)

var imv4LegacyProfile = flag.Bool("imv4-baseline", false, "profile the legacy staged commit")
var imv4EnableProfile = flag.Bool("imv4-profile", false, "run the slow Node IndexedDB import profile")

// TestIMV4DurableImportProfile includes the production undo capture, encrypted
// journal and strict IndexedDB batch, using the isolated wasm storage harness.
func TestIMV4DurableImportProfile(t *testing.T) {
	if js.Global().Get("process").Truthy() && !*imv4EnableProfile {
		t.Skip("use the real-browser harness for latency; Node's IDB mock uses a timer per request")
	}
	if !js.Global().Get("process").Truthy() && browserstore.SecureKeyState() == nil {
		browserstore.Init() // isolated real-browser harness, with real IndexedDB
	}
	for _, count := range []int{3, 300, 3000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			a, commits := bv6dJournal(t, false)
			account := a.Accounts()[0]
			rows := make([]extract.Row, count)
			for i := range rows {
				rows[i] = extract.Row{ID: fmt.Sprintf("imv4-%d", i), Date: "2026-10-01", Description: fmt.Sprintf("IMV4 durable purchase %d", i), Amount: "-1.25"}
			}
			batch, err := a.Stage(domain.DocCSV, rows, account.ID)
			if err != nil {
				t.Fatal(err)
			}
			notifications := 0
			a.OnTxnMutated(func() { notifications++ })
			start := time.Now()
			performance := js.Global().Get("performance")
			lastTick := performance.Call("now").Float()
			maxGap := 0.0
			heartbeat := js.FuncOf(func(js.Value, []js.Value) any {
				now := performance.Call("now").Float()
				if gap := now - lastTick; gap > maxGap {
					maxGap = gap
				}
				lastTick = now
				return nil
			})
			timer := js.Global().Call("setInterval", heartbeat, 5)
			defer func() { js.Global().Call("clearInterval", timer); heartbeat.Release() }()
			completed := make(chan error, 1)
			done := func(result appstate.CSVImportResult, err error) {
				if err == nil && result.Imported != count {
					err = fmt.Errorf("imported %d, want %d", result.Imported, count)
				}
				completed <- err
			}
			if *imv4LegacyProfile {
				a.CommitDurable(batch, "imv4-run", "", done)
			} else {
				a.CommitImportBatchDurable(batch, "imv4-run", "", nil, done)
			}
			handler := time.Since(start)
			commit := waitSOLP2Commit(t, commits)
			if undoStack.Len() != 0 {
				t.Fatal("undo published before durability")
			}
			captured := time.Now()
			bv6dFinish(t, commit)
			if err := <-completed; err != nil {
				t.Fatal(err)
			}
			if notifications != 1 {
				t.Fatalf("notifications=%d, want 1", notifications)
			}
			if undoStack.Len() != 1 {
				t.Fatalf("undo entries=%d, want one whole import", undoStack.Len())
			}
			t.Logf("IMV4 durable legacy=%t rows=%d handler=%s capture=%s strict-write=%s total=%s max-event-loop-gap=%.3fms journal-records=%d notifications=%d", *imv4LegacyProfile, count, handler, captured.Sub(start)-handler, time.Since(captured), time.Since(start), maxGap, len(commit.values), notifications)
		})
	}
}
