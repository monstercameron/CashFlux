// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func bv6dJournal(t *testing.T, large bool) (*appstate.App, chan solp2Commit) {
	t.Helper()
	solp22Storage(t)
	browserstore.Barrier()
	oldNoticeApp := durableNoticeApp
	oldSnap, oldStack := lastUndoSnap, undoStack
	oldUndoApp, oldUndoRev := lastUndoCaptureApp, lastUndoCaptureRev
	oldApplying, oldPending, oldDurableCursor := applyingUndo, undoPersistencePending, undoDurableCursor
	oldHydrated, oldLocal, oldSuspended := hydrationSucceeded, hadLocalDataset, suspendAutosave
	oldQ, oldQP, oldEnvelope := datasetQuarantined, quarantinePending, pendingEnvelopeRaw
	oldHosted, oldGen, oldApp := currentHostedHydration(), datasetMyGen, appstate.Default
	oldHApp, oldHCursor, oldHRev := hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision
	oldCursor, oldUndoPending, oldFeed := undoCaptureCursor, undoPending, auditview.Feed
	previous := map[string]string{}
	for _, key := range []string{datasetStoreKey, datasetHeadKey, datasetGenKey} {
		previous[key] = browserstore.GetString(key)
	}
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	appstate.Default = a
	hydrationSucceeded, hadLocalDataset, suspendAutosave = true, true, false
	datasetQuarantined, quarantinePending, pendingEnvelopeRaw = false, false, ""
	initializeHostedHydration(false)
	ds := store.SampleDataset()
	ds.Transactions = ds.Transactions[:2]
	if large {
		row := ds.Transactions[0]
		for i := 0; i < 3300; i++ {
			row.ID = fmt.Sprintf("bv6d-%04d", i)
			ds.Transactions = append(ds.Transactions, row)
		}
	}
	if err := a.Store().Load(ds); err != nil {
		t.Fatal(err)
	}
	// Earlier recovery/wipe fixtures intentionally leave empty preference
	// tombstones. Give this household valid, explicit preferences rather than
	// migrating process-global leftovers into its budget-write validation.
	preferenceJSON, _ := json.Marshal(prefs.Default())
	if err := a.SetSettingKV("cashflux:prefs", string(preferenceJSON)); err != nil {
		t.Fatal(err)
	}
	_ = uistate.LoadPrefs()
	data, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	values, expected := prepareWorkspaceCommit(map[string]string{datasetStoreKey: encodeRecordCheckpoint(data)}, readDatasetGen())
	solp22Batch(t, values, expected)
	datasetMyGen = values[datasetGenKey]
	auditview.Feed = auditlog.New(500)
	undoStack = history.NewStack(4 * 1024 * 1024)
	applyingUndo, undoPersistencePending = false, false
	initUndo()
	hydratedRecordApp, hydratedRecordRevision = a, a.Rev()
	hydratedRecordCursor, _ = a.Store().ChangeCursor()
	commits := make(chan solp2Commit, 12)
	save, _ := newDatasetAutosave(a, func(values, expected map[string]string, done func(error)) {
		commits <- solp2Commit{values, expected, done}
	}, nil)
	uistate.CapturePersistNow(save)
	t.Cleanup(func() {
		notifySchedule.epoch++
		if notifySchedule.timer != nil {
			notifySchedule.timer.Stop()
		}
		persistPerfObserve = nil
		uistate.CapturePersistNow(nil)
		uistate.ClearPersistFailure("dataset")
		uistate.ClearPersistFailure("notice")
		durableNoticeApp = oldNoticeApp
		lastUndoSnap, undoStack = oldSnap, oldStack
		lastUndoCaptureApp, lastUndoCaptureRev = oldUndoApp, oldUndoRev
		applyingUndo, undoPersistencePending, undoDurableCursor = oldApplying, oldPending, oldDurableCursor
		solp22Batch(t, previous, nil)
		appstate.Default, datasetMyGen = oldApp, oldGen
		hydrationSucceeded, hadLocalDataset, suspendAutosave = oldHydrated, oldLocal, oldSuspended
		datasetQuarantined, quarantinePending, pendingEnvelopeRaw = oldQ, oldQP, oldEnvelope
		setHostedHydration(oldHosted)
		hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = oldHApp, oldHCursor, oldHRev
		undoCaptureCursor, undoPending, auditview.Feed = oldCursor, oldUndoPending, oldFeed
		a.Store().Close()
	})
	return a, commits
}

func bv6dFinish(t *testing.T, commit solp2Commit) {
	t.Helper()
	err := enc31Wait(t, func(done func(error)) {
		browserstore.SetSecureBatch(commit.values, commit.expected, done)
	})
	commit.done(err)
	if err != nil {
		t.Fatal(err)
	}
}

func bv6dWait(t *testing.T, f *render.Fixture, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		f.Stabilize()
		if predicate() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("feedback did not reach its expected state")
}

func bv6dText(f *render.Fixture) string {
	var text strings.Builder
	for _, node := range f.AllByTag("span") {
		text.WriteString(node.Text())
	}
	return text.String()
}

func TestBV6DDensityLimitCreateDurableSequence(t *testing.T) {
	a, commits := bv6dJournal(t, true)
	exports := 0
	persistPerfObserve = func(stage string) {
		if stage == "dataset-export" {
			exports++
		}
	}
	beforeCursor, _ := a.Store().ChangeCursor()
	beforeRev, beforeGen := a.Rev(), readDatasetGen()
	for _, density := range []string{uistate.BudgetDensityCompact, uistate.BudgetDensityComfortable} {
		start := time.Now()
		uistate.PersistBudgetDensity(density)
		elapsed := time.Since(start)
		t.Logf("BV6D density handler %s: %.3f ms", density, float64(elapsed.Microseconds())/1000)
		if elapsed >= 100*time.Millisecond {
			t.Fatalf("density blocked for %s", elapsed)
		}
	}
	uistate.PersistRailCollapsed(true)
	uistate.PersistCollapsedToolGroups(map[string]bool{"money": false})
	if err := enc31Wait(t, uistate.RequestPreferenceDurable); err != nil {
		t.Fatal(err)
	}
	afterCursor, _ := a.Store().ChangeCursor()
	if beforeCursor != afterCursor || beforeRev != a.Rev() || beforeGen != readDatasetGen() || len(commits) != 0 {
		t.Fatal("presentation preference changed household records or generation")
	}
	budget := a.Budgets()[0]
	budget.Limit.Amount += 2000
	if err := a.PutBudget(budget); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 2; step++ {
		if step == 1 {
			budget.ID, budget.Name = "bv6d-created", "BV6D budget"
			if err := a.PutBudget(budget); err != nil {
				t.Fatal(err)
			}
		}
		completed := make(chan appstate.DurableResult, 1)
		a.RequestDurable(func(r appstate.DurableResult) { completed <- r })
		c := waitSOLP2Commit(t, commits)
		if _, full := c.values[datasetStoreKey]; full {
			t.Fatal("budget write exported a whole dataset")
		}
		if c.expected[datasetGenKey] != readDatasetGen() {
			t.Fatal("single-tab write used a stale generation")
		}
		select {
		case <-completed:
			t.Fatal("budget acknowledged before strict completion")
		default:
		}
		bv6dFinish(t, c)
		if r := <-completed; r.Err != nil {
			t.Fatal(r.Err)
		}
		if readDatasetGen() != datasetMyGen || readDatasetGen() == beforeGen {
			t.Fatal("strict completion did not adopt its own generation")
		}
		beforeGen = readDatasetGen()
	}
	portable, _, err := materializeLocalDataset()
	if err != nil {
		t.Fatal(err)
	}
	dataset, err := store.Import([]byte(portable))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, saved := range dataset.Budgets {
		if saved.ID == budget.ID && saved.Limit == budget.Limit {
			found = true
		}
	}
	if !found || len(dataset.Transactions) != 3302 || exports != 0 {
		t.Fatalf("reload: budget=%v transactions=%d exports=%d", found, len(dataset.Transactions), exports)
	}
}

func TestBV6DProductionDensityToggleAndRenderUnder100ms(t *testing.T) {
	a, commits := bv6dJournal(t, true)
	f := render.New(t)
	defer f.Cleanup()
	toolbar, ok := widgetrender.Render("budget-toolbar", widgetrender.RenderCtx{App: a, Base: "USD"})
	if !ok {
		t.Fatal("production budget toolbar is not registered")
	}
	f.Render(Body(toolbar))
	f.Stabilize()
	cursor, _ := a.Store().ChangeCursor()
	for i := 0; i < 2; i++ {
		button := findAppTestID(f, "button", "budgets-density")
		if button == nil {
			t.Fatal("production density toggle missing")
		}
		before := button.Attr("data-density")
		start := time.Now()
		button.Click()
		f.Stabilize()
		elapsed := time.Since(start)
		after := findAppTestID(f, "button", "budgets-density").Attr("data-density")
		t.Logf("BV6D production density %s -> %s, click + toolbar render: %.3f ms", before, after, float64(elapsed.Microseconds())/1000)
		if before == after || elapsed >= 100*time.Millisecond {
			t.Fatalf("toggle %s -> %s took %s", before, after, elapsed)
		}
	}
	if err := enc31Wait(t, uistate.RequestPreferenceDurable); err != nil {
		t.Fatal(err)
	}
	afterCursor, _ := a.Store().ChangeCursor()
	if cursor != afterCursor || len(commits) != 0 {
		t.Fatal("production density control wrote household records")
	}
}

func TestBV6DFailedBudgetNoticeBannerRetryAndSupersession(t *testing.T) {
	a, commits := bv6dJournal(t, false)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ui.CreateElement(Toast)))
	f.Stabilize()
	b := a.Budgets()[0]
	b.Limit.Amount += 500
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	uistate.PostUndoable("Limit set to $500.")
	f.Stabilize()
	c := waitSOLP2Commit(t, commits)
	if strings.Contains(bv6dText(f), "Limit set") {
		t.Fatal("success shown before durable completion")
	}
	c.done(errors.New("strict transaction aborted"))
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") != nil })
	if strings.Contains(bv6dText(f), "Limit set") || !strings.Contains(bv6dText(f), "Changes aren't saved yet") {
		t.Fatal("failed write advertised success or lacked plain-English feedback")
	}
	uistate.PostNotice("An unrelated error", true)
	f.Stabilize()
	if findAppTestID(f, "div", "unsaved-changes") == nil {
		t.Fatal("another notice dismissed the durable failure")
	}
	findAppTestID(f, "button", "retry-unsaved-changes").Click()
	retry := waitSOLP2Commit(t, commits)
	bv6dFinish(t, retry)
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") == nil })
	if strings.Contains(bv6dText(f), "Limit set") {
		t.Fatal("old completion replaced a newer notice")
	}
	// The create notice uses exactly the same durable presentation boundary.
	b.ID, b.Name = "bv6d-toast-create", "Toast budget"
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	uistate.PostNotice("Budget created.", false)
	f.Stabilize()
	c = waitSOLP2Commit(t, commits)
	if strings.Contains(bv6dText(f), "Budget created") {
		t.Fatal("create notice preceded durability")
	}
	bv6dFinish(t, c)
	bv6dWait(t, f, func() bool { return strings.Contains(bv6dText(f), "Budget created.") })
	// The periodic writer may recover before the person presses Retry. Its
	// success must release both the persistent warning and the deferred notice.
	b.Limit.Amount++
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	uistate.PostNotice("Budget updated.", false)
	f.Stabilize()
	waitSOLP2Commit(t, commits).done(errors.New("temporary storage failure"))
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") != nil })
	uistate.RequestPersist()
	bv6dFinish(t, waitSOLP2Commit(t, commits))
	bv6dWait(t, f, func() bool {
		return findAppTestID(f, "div", "unsaved-changes") == nil && strings.Contains(bv6dText(f), "Budget updated.")
	})
}

func TestBV6DPreferenceFailureRetryDoesNotWriteDataset(t *testing.T) {
	_, commits := bv6dJournal(t, false)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ui.CreateElement(Toast)))
	f.Stabilize()
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	uistate.PersistBudgetDensity(uistate.BudgetDensityCompact)
	uistate.PostNotice("Preference saved.", false)
	f.Stabilize()
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") != nil })
	if strings.Contains(bv6dText(f), "Preference saved") {
		t.Fatal("failed preference claimed success")
	}
	findAppTestID(f, "button", "retry-unsaved-changes").Click()
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") == nil })
	if err := enc31Wait(t, uistate.RequestPreferenceDurable); err != nil {
		t.Fatal(err)
	}
	if len(commits) != 0 {
		t.Fatal("preference Retry wrote the dataset")
	}
	bv6dWait(t, f, func() bool { return strings.Contains(bv6dText(f), "Preference saved.") })
}

func TestBV6DAutosaveFailureWithoutWaitersKeepsBannerUntilCommit(t *testing.T) {
	a, commits := bv6dJournal(t, false)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ui.CreateElement(SaveFailureBanner)))
	b := a.Budgets()[0]
	b.Limit.Amount++
	if err := a.PutBudget(b); err != nil {
		t.Fatal(err)
	}
	uistate.RequestPersist()
	c := waitSOLP2Commit(t, commits)
	c.done(errors.New("no space left"))
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") != nil })
	for i := 0; i < 3; i++ {
		findAppTestID(f, "button", "retry-unsaved-changes").Click()
	}
	retry := waitSOLP2Commit(t, commits)
	f.Stabilize()
	if findAppTestID(f, "div", "unsaved-changes") == nil || len(commits) != 0 {
		t.Fatal("Retry cleared the error early or launched duplicate commits")
	}
	retry.done(errors.New("disk still unavailable"))
	f.Stabilize()
	if findAppTestID(f, "div", "unsaved-changes") == nil {
		t.Fatal("second failure lost the persistent banner")
	}
	findAppTestID(f, "button", "retry-unsaved-changes").Click()
	bv6dFinish(t, waitSOLP2Commit(t, commits))
	bv6dWait(t, f, func() bool { return findAppTestID(f, "div", "unsaved-changes") == nil })
}

func TestBV6DBudgetUndoTouchesOnlyBudgetRecords(t *testing.T) {
	a, commits := bv6dJournal(t, true)
	before := a.Budgets()[0]
	after := before
	after.Limit = money.New(before.Limit.Amount+1000, before.Limit.Currency)
	if err := a.PutBudget(after); err != nil {
		t.Fatal(err)
	}
	a.RequestDurable(func(r appstate.DurableResult) {
		if r.Err != nil {
			t.Error(r.Err)
		}
	})
	bv6dFinish(t, waitSOLP2Commit(t, commits))
	exports := 0
	persistPerfObserve = func(stage string) {
		if stage == "dataset-export" {
			exports++
		}
	}
	startCursor, _ := a.Store().ChangeCursor()
	completed := make(chan error, 1)
	start := time.Now()
	undoLastChangeThen(false, func(err error) { completed <- err })
	elapsed := time.Since(start)
	t.Logf("BV6D 3302-transaction budget Undo handler: %.3f ms", float64(elapsed.Microseconds())/1000)
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("Undo blocked for %s", elapsed)
	}
	c := waitSOLP2Commit(t, commits)
	if _, full := c.values[datasetStoreKey]; full || exports != 0 {
		t.Fatal("Undo exported the full dataset")
	}
	endCursor, _ := a.Store().ChangeCursor()
	page, err := a.Store().CaptureChanges(startCursor, endCursor, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range page.Changes {
		if record.Table != "budgets" || record.ID != before.ID {
			t.Fatalf("Undo touched unrelated record %s:%s", record.Table, record.ID)
		}
	}
	if len(page.Changes) != 1 {
		t.Fatalf("Undo changed %d records", len(page.Changes))
	}
	select {
	case <-completed:
		t.Fatal("Undo acknowledged before durability")
	default:
	}
	bv6dFinish(t, c)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	got, ok, err := a.Store().GetBudget(before.ID)
	if err != nil || !ok || got.Limit != before.Limit {
		t.Fatalf("Undo budget: %+v %v", got, err)
	}
}

// Main's generation writer was separate and optimistic: an aborted same-tab
// generation write leaves its cache ahead of disk. Every later autosave CAS can
// then fail even with no other tab. Ordinary density and recurring SQL writes
// cannot produce this mismatch by themselves.
func TestBV6DMainFailedGenerationWriteRepeatsCASAndAtomicReplacementRecovers(t *testing.T) {
	oldNoticeApp := durableNoticeApp
	t.Cleanup(func() {
		durableNoticeApp = oldNoticeApp
		uistate.ClearPersistFailure("dataset")
		uistate.ClearPersistFailure("notice")
	})
	f := solp22Journal(t)
	old := readDatasetGen()
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	datasetMyGen = bumpDatasetGen()
	browserstore.Barrier()
	poisoned := datasetMyGen
	if readDatasetGen() != poisoned || poisoned == old {
		t.Fatal("legacy optimistic generation sequence was not reproduced")
	}
	for i := 0; i < 3; i++ {
		err := enc31Wait(t, func(done func(error)) {
			browserstore.SetSecureBatch(map[string]string{datasetGenKey: "later-save"}, map[string]string{datasetGenKey: poisoned}, done)
		})
		if !errors.Is(err, atrest.ErrRecordChanged) {
			t.Fatalf("legacy retry %d = %v", i, err)
		}
	}
	// Restore the observed durable generation; never adopt an unknown generation
	// in production to hide a real competing writer. SOLP2.2 replacements publish
	// dataset/head/generation together and adopt the result only after commit.
	browserstore.SetMemory(datasetGenKey, old)
	datasetMyGen = old
	portable, _, err := materializeLocalDataset()
	if err != nil {
		t.Fatal(err)
	}
	values, expected := prepareWorkspaceCommit(map[string]string{datasetStoreKey: string(portable)}, old)
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	err = enc31Wait(t, func(done func(error)) { browserstore.SetSecureBatch(values, expected, done) })
	if err == nil || readDatasetGen() != old || datasetMyGen != old {
		t.Fatal("failed atomic replacement poisoned its generation")
	}
	hydratedRecordApp, hydratedRecordRevision = f.app, f.app.Rev()
	hydratedRecordCursor, _ = f.app.Store().ChangeCursor()
	newDatasetAutosave(f.app, browserstore.SetSecureBatch, nil)
	for _, amount := range []int64{5100, 5200} {
		budget := f.app.Budgets()[0]
		budget.Limit = money.New(amount, budget.Limit.Currency)
		if err := f.app.PutBudget(budget); err != nil {
			t.Fatal(err)
		}
		if err := enc31Wait(t, func(done func(error)) { f.app.RequestDurable(func(r appstate.DurableResult) { done(r.Err) }) }); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBV6DPreferenceReloadRapidChangesAndAccountIsolation(t *testing.T) {
	a, commits := bv6dJournal(t, false)
	const densityKey = "cashflux:budgets:density"
	if err := a.SetKV(densityKey, uistate.BudgetDensityCompact); err != nil {
		t.Fatal(err)
	}
	// A legacy household choice remains readable without migration writes during
	// render; the next explicit choice becomes the independent durable preference.
	cursor, _ := a.Store().ChangeCursor()
	_ = uistate.KVGet(densityKey)
	for i := 0; i < 20; i++ {
		uistate.PersistBudgetDensity(uistate.BudgetDensityCompact)
		uistate.PersistBudgetDensity(uistate.BudgetDensityComfortable)
	}
	if err := enc31Wait(t, uistate.RequestPreferenceDurable); err != nil {
		t.Fatal(err)
	}
	after, _ := a.Store().ChangeCursor()
	if cursor != after || len(commits) != 0 {
		t.Fatal("preference read/write changed the dataset")
	}
	loaded, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Store().Close()
	appstate.Default = loaded
	defer func() { appstate.Default = a }()
	if got := uistate.KVGet(densityKey); got != uistate.BudgetDensityComfortable {
		t.Fatalf("reloaded density = %q", got)
	}
	other, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Store().Close()
	oldUser := browserstore.GetString(syncUserIDKey)
	browserstore.SetMemory(syncUserIDKey, "bv6d-other-account")
	defer browserstore.SetMemory(syncUserIDKey, oldUser)
	appstate.Default = other
	if got := uistate.KVGet(densityKey); got != "" {
		t.Fatalf("another account inherited density %q", got)
	}
}

func TestBV6DToastTimerToleratesAbsentAndPartialDocuments(t *testing.T) {
	for _, mode := range []string{"absent", "partial document", "partial element"} {
		t.Run(mode, func(t *testing.T) {
			previousApp, previousDocument := appstate.Default, js.Global().Get("document")
			a, err := appstate.New(io.Discard, false)
			if err != nil {
				t.Fatal(err)
			}
			appstate.Default = a
			t.Cleanup(func() {
				appstate.Default = previousApp
				js.Global().Set("document", previousDocument)
				a.Store().Close()
			})
			f := render.New(t)
			defer f.Cleanup()
			f.Render(Body(ui.CreateElement(Toast)))
			uistate.PostNotice("A useful notice", false)
			f.Stabilize()
			document := js.Undefined()
			if mode != "absent" {
				document = js.Global().Get("Object").New()
				document.Set("visibilityState", "visible")
				if mode == "partial element" {
					document.Set("querySelector", js.Global().Get("Function").New("return {};"))
				}
			}
			js.Global().Set("document", document)
			// Exercise the installed production interval, not a duplicate timer.
			time.Sleep(2 * toastTickMS * time.Millisecond)
			f.Stabilize()
			if !strings.Contains(bv6dText(f), "A useful notice") {
				t.Fatal("incomplete document removed the active notice")
			}
		})
	}
}

func TestBV6DWipeCompletionCannotReloadAReplacementApp(t *testing.T) {
	a, _ := bv6dJournal(t, false)
	solp23PreserveStorage(t)
	const preferenceKey = "cashflux:ui-preference:bv6d-preserve:cashflux:rail-collapsed"
	solp22Batch(t, map[string]string{preferenceKey: "1"}, nil)
	t.Cleanup(func() {
		if err := enc31Wait(t, func(done func(error)) { browserstore.RemoveSecure(preferenceKey, done) }); err != nil {
			t.Error(err)
		}
	})
	if err := a.Wipe(); err != nil {
		t.Fatal(err)
	}
	generation := datasetMyGen
	continued := false
	wipeFinancialLocalState(func() { continued = true })
	// The strict commit is queued. Replace the App before its completion: an old
	// page must neither adopt its generation into the new App nor reload it.
	appstate.Default = nil
	defer func() { appstate.Default = a }()
	browserstore.Barrier()
	if continued || datasetMyGen != generation {
		t.Fatal("obsolete wipe completion reloaded or changed the replacement App")
	}
	if value, _, err := browserstore.GetSecure(preferenceKey); err != nil || value != "1" {
		t.Fatalf("household wipe removed device presentation preferences: %q %v", value, err)
	}
}
