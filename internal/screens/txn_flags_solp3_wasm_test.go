// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/store"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func solp3FlagsApp(t *testing.T, n int) *appstate.App {
	t.Helper()
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Store().Close() })
	ds := store.SampleDataset()
	ds.Transactions = r3pFixture(n)
	for i := range ds.Transactions {
		ds.Transactions[i].AccountID = ds.Accounts[0].ID
		ds.Transactions[i].CategoryID = ds.Categories[0].ID
	}
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	a.Transactions()
	return a
}

func solp3Wait(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("flag job did not complete")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSOLP3FlagJobMatchesCanonicalDetectorsAndDismissals(t *testing.T) {
	a := solp3FlagsApp(t, 80)
	in := buildSmartInputRaw(a, time.Monday)
	// Include duplicate signatures, another account, cleared state and a transfer.
	first := in.Transactions[0]
	second := first
	second.ID = "duplicate"
	third := first
	third.ID = "other-account"
	third.AccountID = "other"
	fourth := first
	fourth.ID = "cleared"
	fourth.Cleared = !first.Cleared
	transfer := first
	transfer.ID = "transfer"
	transfer.TransferGroupID = "pair"
	in.Transactions = append(in.Transactions, second, third, fourth, transfer)
	for _, test := range []struct {
		name     string
		settings smart.Settings
	}{
		{"default", smart.Settings{}},
		{"disabled", smart.Settings{ExplicitOff: map[string]bool{dupeFlagCode: true, spikeFlagCode: true}}},
		{"muted", smart.Settings{Muted: map[string]bool{dupeFlagCode: true, spikeFlagCode: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := newTxnFlagsWork(in, test.settings)
			if w.done {
				t.Fatal("new job already completed")
			}
			for !w.step() {
			}
			expected := txnRowFlags{}
			if test.settings.IsEnabled(dupeFlagCode) && !test.settings.IsMuted(dupeFlagCode) {
				expected.Duplicate = map[string]bool{}
				for _, group := range dedupe.FindDuplicates(in.Transactions) {
					for _, id := range group.IDs {
						expected.Duplicate[id] = true
					}
				}
			}
			if test.settings.IsEnabled(spikeFlagCode) && !test.settings.IsMuted(spikeFlagCode) {
				expected.Spike = map[string]string{}
				for _, ins := range smartengine.RunPage(in, txnFlagEngineSettings(test.settings), smart.PageTransactions) {
					if ins.Feature == spikeFlagCode && ins.Action != nil && ins.Action.RelatedID != "" {
						expected.Spike[ins.Action.RelatedID] = ins.Detail
					}
				}
			}
			if !reflect.DeepEqual(w.value, expected) {
				t.Fatalf("flags differ: got=%+v want=%+v", w.value, expected)
			}
			if !w.step() {
				t.Fatal("completed job is not idempotent")
			}
		})
	}
	job := smartengine.NewUnusualJob(in)
	for !job.Step(8) {
	}
	settings := smart.Settings{Dismissed: map[string]bool{}}
	for _, ins := range job.Insights() {
		settings.Dismissed[ins.Key] = true
	}
	w := newTxnFlagsWork(in, settings)
	for !w.step() {
	}
	if len(w.value.Spike) != 0 {
		t.Fatal("dismissed insight became a row flag")
	}
}

func TestSOLP3ScheduledFlags3300SliceBudget(t *testing.T) {
	a := solp3FlagsApp(t, 3300)
	settings := smart.Settings{}
	key := makeTxnFlagsKey(a, 0, settings, time.Monday)
	var result txnRowFlags
	published := false
	count := 0
	var longest, total time.Duration
	old := txnPerfObserve
	txnPerfObserve = func(stage string, d time.Duration) {
		if stage == "flag-slice" {
			count++
			total += d
			if d > longest {
				longest = d
			}
		}
	}
	defer func() { txnPerfObserve = old }()
	cancel := scheduleTxnFlags(key, settings, func() bool { return true }, func(flags txnRowFlags) { result = flags; published = true })
	defer cancel()
	solp3Wait(t, func() bool { return published })
	if count < 2 {
		t.Fatalf("whole ledger drained in %d slices", count)
	}
	if longest > 200*time.Millisecond {
		t.Fatalf("flag task exceeded 200ms: %s", longest)
	}
	expected := buildTxnRowFlags(a, settings, time.Monday)
	if !reflect.DeepEqual(result, expected) {
		t.Fatal("scheduled flags differ from complete detector output")
	}
	t.Logf("SOLP3 flags rows=3300 slices=%d max=%.3f ms total CPU=%.3f ms", count, float64(longest.Microseconds())/1000, float64(total.Microseconds())/1000)
}

func TestSOLP3FlagSchedulerCancelsBeforeAndBetweenSlices(t *testing.T) {
	a := solp3FlagsApp(t, 3300)
	key := makeTxnFlagsKey(a, 0, smart.Settings{}, time.Monday)
	for _, between := range []bool{false, true} {
		t.Run(map[bool]string{false: "before timer", true: "between slices"}[between], func(t *testing.T) {
			current := true
			slices, published := 0, 0
			old := txnPerfObserve
			txnPerfObserve = func(stage string, d time.Duration) {
				if stage == "flag-slice" {
					slices++
					current = false
				}
			}
			defer func() { txnPerfObserve = old }()
			cancel := scheduleTxnFlags(key, smart.Settings{}, func() bool { return current }, func(txnRowFlags) { published++ })
			defer cancel()
			if between {
				solp3Wait(t, func() bool { return slices > 0 })
			} else {
				cancel()
				time.Sleep(320 * time.Millisecond)
			}
			time.Sleep(20 * time.Millisecond)
			if published != 0 || slices > 1 {
				t.Fatalf("cancelled job published=%d slices=%d", published, slices)
			}
		})
	}
}

func TestSOLP3FlagHookKeepsPreviousAndCancelsScopeSettingsUnmount(t *testing.T) {
	a := solp3FlagsApp(t, 2)
	rows := a.Transactions()
	row := rows[0]
	row.ID = "dupe"
	if err := a.Store().PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	settings := smart.Settings{ExplicitOff: map[string]bool{spikeFlagCode: true}}
	revision := 0
	currentApp := a
	renderToken := 0
	var observed txnRowFlags
	root := func(_ int) ui.Node {
		observed = useTxnRowFlags(currentApp, revision, settings, time.Monday)
		return Div()
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(root, renderToken))
	solp3Wait(t, func() bool { return observed.Duplicate[row.ID] })
	// Keep completed flags while a changed revision is being computed.
	row.Desc = "different"
	if err := a.Store().PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	revision++
	renderToken++
	f.Rerender(ui.CreateElement(root, renderToken))
	if !observed.Duplicate[row.ID] {
		t.Fatal("previous flags disappeared during recomputation")
	}
	// Changed settings cancel the older job, even if the maps were mutated in place.
	settings.ExplicitOff[dupeFlagCode] = true
	renderToken++
	f.Rerender(ui.CreateElement(root, renderToken))
	solp3Wait(t, func() bool { return !observed.Any() })
	// A workspace/app boundary must not retain the old flags.
	settings.ExplicitOff[dupeFlagCode] = false
	row.Desc = rows[0].Desc
	if err := a.Store().PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	renderToken++
	f.Rerender(ui.CreateElement(root, renderToken))
	solp3Wait(t, func() bool { return observed.Any() })
	other := solp3FlagsApp(t, 1)
	currentApp = other
	renderToken++
	f.Rerender(ui.CreateElement(root, renderToken))
	if observed.Any() {
		t.Fatal("flags crossed App identity")
	}
	currentApp = a
	renderToken++
	f.Rerender(ui.CreateElement(root, renderToken))
	solp3Wait(t, func() bool { return observed.Any() })
	oldRegistry, hadRegistry := browserstore.Get("cashflux:workspaces")
	defer func() {
		if hadRegistry {
			browserstore.Set("cashflux:workspaces", oldRegistry)
		} else {
			browserstore.Remove("cashflux:workspaces")
		}
	}()
	registry, _ := json.Marshal(map[string]any{"activeId": "solp3-other-workspace"})
	browserstore.Set("cashflux:workspaces", string(registry))
	renderToken++
	f.Rerender(ui.CreateElement(root, renderToken))
	if observed.Any() {
		t.Fatal("flags crossed workspace identity")
	}
	slices := 0
	oldObserve := txnPerfObserve
	txnPerfObserve = func(stage string, _ time.Duration) {
		if stage == "flag-slice" {
			slices++
		}
	}
	defer func() { txnPerfObserve = oldObserve }()
	f.Rerender(Div("unmounted"))
	time.Sleep(330 * time.Millisecond)
	if slices != 0 {
		t.Fatal("unmounted hook continued flag calculation")
	}
}
