// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/history"
	"github.com/monstercameron/CashFlux/internal/keyring"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/undosnap"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type solp2Commit struct {
	values, expected map[string]string
	done             func(error)
}

func waitSOLP2Commit(t *testing.T, commits <-chan solp2Commit) solp2Commit {
	t.Helper()
	select {
	case commit := <-commits:
		return commit
	case <-time.After(5 * time.Second):
		t.Fatal("no commit request")
		return solp2Commit{}
	}
}

func testSOLP2RecordPersistence(t *testing.T) {
	a := solp1App(t)
	oldHydrated, oldLocal, oldSuspended := hydrationSucceeded, hadLocalDataset, suspendAutosave
	oldQ, oldQP, oldPending := datasetQuarantined, quarantinePending, pendingEnvelopeRaw
	oldHosted := currentHostedHydration()
	oldSnap, oldStack, oldFeed := lastUndoSnap, undoStack, auditview.Feed
	oldUndoApp, oldUndoRev, oldCursor, oldUndoPending := lastUndoCaptureApp, lastUndoCaptureRev, undoCaptureCursor, undoPending
	oldHApp, oldHCursor, oldHRev := hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision
	oldGen, oldCheckpoint, oldHead := readDatasetGen(), browserstore.GetString(datasetStoreKey), browserstore.GetString(datasetHeadKey)
	oldMyGen := datasetMyGen
	hydrationSucceeded, hadLocalDataset, suspendAutosave = true, true, false
	datasetQuarantined, quarantinePending, pendingEnvelopeRaw = false, false, ""
	initializeHostedHydration(false)
	undoStack, auditview.Feed = history.NewStack(4*1024*1024), auditlog.New(500)
	// Resolve legacy preference migration before measuring an ordinary write.
	// The full app suite leaves a legacy preferences record; reading it for the
	// first time is itself a real settings write, not an idle autosave.
	// Bypass LoadPrefs' process-wide TTL cache: it can still contain the prior
	// fixture's preferences and defer the legacy-row migration until later.
	_ = uistate.SettingKVGet("cashflux:prefs")
	_ = uistate.LoadPrefs()
	_ = currentDatasetServerID()
	_ = signedInUserID()
	initUndo()
	checkpoint, err := localDatasetExport(a)
	if err != nil {
		t.Fatal(err)
	}
	browserstore.SetMemory(datasetStoreKey, encodeRecordCheckpoint(checkpoint))
	browserstore.SetMemory(datasetHeadKey, "")
	hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = a, undoCaptureCursor, a.Rev()
	t.Cleanup(func() {
		committedPushJob++
		hydrationSucceeded, hadLocalDataset, suspendAutosave = oldHydrated, oldLocal, oldSuspended
		datasetQuarantined, quarantinePending, pendingEnvelopeRaw = oldQ, oldQP, oldPending
		setHostedHydration(oldHosted)
		lastUndoSnap, undoStack, auditview.Feed = oldSnap, oldStack, oldFeed
		lastUndoCaptureApp, lastUndoCaptureRev, undoCaptureCursor, undoPending = oldUndoApp, oldUndoRev, oldCursor, oldUndoPending
		hydratedRecordApp, hydratedRecordCursor, hydratedRecordRevision = oldHApp, oldHCursor, oldHRev
		browserstore.SetMemory(datasetStoreKey, oldCheckpoint)
		browserstore.SetMemory(datasetHeadKey, oldHead)
		browserstore.SetMemory(datasetGenKey, oldGen)
		datasetMyGen = oldMyGen
		persistPerfObserve = nil
	})
	commits := make(chan solp2Commit, 8)
	pushes := make(chan []byte, 8)
	wk, err := keyring.NewWorkspaceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer wk.Destroy()
	save, force := newDatasetAutosave(a, func(values, expected map[string]string, done func(error)) {
		for key, value := range values {
			envelope, err := atrest.SealRecord(wk, keyring.LocalScope(""), "solp2-fixture", key, []byte(value))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := envelope.Encode(); err != nil {
				t.Fatal(err)
			}
			plain, err := atrest.OpenRecord(wk, keyring.LocalScope(""), "solp2-fixture", key, envelope)
			if err != nil || string(plain) != value {
				t.Fatal("ENC3 record round-trip failed")
			}
			if _, err := atrest.OpenRecord(wk, keyring.LocalScope(""), "other-workspace", key, envelope); err == nil {
				t.Fatal("record opened under another scope")
			}
		}
		commits <- solp2Commit{values, expected, done}
	}, func(data []byte, _ time.Time) { pushes <- data })
	exports := 0
	persistPerfObserve = func(stage string) {
		if stage == "dataset-export" {
			exports++
		}
	}
	durable := map[string]string{datasetStoreKey: encodeRecordCheckpoint(checkpoint), datasetGenKey: oldGen}
	finish := func(c solp2Commit) {
		for key, want := range c.expected {
			if durable[key] != want {
				t.Fatalf("invalid CAS for %s", key)
			}
		}
		for key, value := range c.values {
			durable[key] = value
			browserstore.SetMemory(key, value)
		}
		c.done(nil)
	}
	row := a.Transactions()[0]
	row.CategoryID = a.Categories()[1].ID
	start := time.Now()
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	completed := make(chan appstate.DurableResult, 4)
	a.RequestDurable(func(r appstate.DurableResult) { completed <- r })
	save()
	save()
	c := waitSOLP2Commit(t, commits)
	t.Logf("SOLP2 recategorise capture/request: %.3f ms", float64(time.Since(start).Microseconds())/1000)
	if _, full := c.values[datasetStoreKey]; full {
		t.Fatal("single record write exported the dataset")
	}
	if exports != 0 || len(c.values) > 6 {
		t.Fatalf("unbounded write: exports=%d records=%d", exports, len(c.values))
	}
	select {
	case <-completed:
		t.Fatal("ack before strict commit")
	default:
	}
	if undoStack.CanUndo() {
		t.Fatal("Undo published before commit")
	}
	c.done(errors.New("strict commit rejected"))
	if result := <-completed; result.Err == nil {
		t.Fatal("failed commit acknowledged")
	}
	if undoStack.CanUndo() {
		t.Fatal("failed commit published Undo")
	}
	a.RequestDurable(func(r appstate.DurableResult) { completed <- r })
	retry := waitSOLP2Commit(t, commits)
	finish(retry)
	if result := <-completed; result.Err != nil {
		t.Fatal(result.Err)
	}
	if !undoStack.CanUndo() {
		t.Fatal("committed change missing Undo")
	}
	if exports != 0 {
		t.Fatalf("retry exported dataset %d times", exports)
	}
	records, err := readDatasetRecords(durable[datasetStoreKey], durable[datasetHeadKey], durable[datasetGenKey], recordWorkspace(), func(key string) (string, bool, error) { v, ok := durable[key]; return v, ok, nil })
	if err != nil {
		t.Fatal(err)
	}
	copyStore, err := store.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer copyStore.Close()
	ds, err := store.Import(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := copyStore.Load(ds); err != nil {
		t.Fatal(err)
	}
	if err := copyStore.ReplayChanges(records); err != nil {
		t.Fatal(err)
	}
	got, ok, err := copyStore.GetTransaction(row.ID)
	if err != nil || !ok || got.CategoryID != row.CategoryID {
		t.Fatalf("reload lost row: %+v %v", got, err)
	}
	select {
	case data := <-pushes:
		ds, err := store.Import(data)
		if err != nil || ds.Transactions[0].CategoryID != row.CategoryID {
			t.Fatal("sync payload does not match durable records")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no committed sync payload")
	}
	save()
	save()
	time.Sleep(10 * time.Millisecond)
	select {
	case extra := <-commits:
		var rows []string
		for _, value := range extra.values {
			var record store.RecordChange
			if json.Unmarshal([]byte(value), &record) == nil && record.Table != "" {
				rows = append(rows, record.Table+":"+record.ID)
			}
		}
		t.Fatalf("idle save wrote records: %v", rows)
	default:
	}
	undoResult := make(chan error, 1)
	var undoEvents []appstate.TransactionDurability
	stopUndoEvents := a.OnTransactionDurability(func(event appstate.TransactionDurability) { undoEvents = append(undoEvents, event) })
	defer stopUndoEvents()
	undoLastChangeThen(false, func(err error) { undoResult <- err })
	undoCommit := waitSOLP2Commit(t, commits)
	if len(undoEvents) != 1 || !undoEvents[0].Pending || len(undoEvents[0].TransactionIDs) != 1 || undoEvents[0].TransactionIDs[0] != row.ID {
		t.Fatal("undo omitted row pending-state")
	}
	if _, full := undoCommit.values[datasetStoreKey]; full {
		t.Fatal("undo exported dataset")
	}
	select {
	case <-undoResult:
		t.Fatal("undo completed before durability")
	default:
	}
	finish(undoCommit)
	if err := <-undoResult; err != nil {
		t.Fatal(err)
	}
	if len(undoEvents) != 2 || undoEvents[1].Pending || undoEvents[1].Result.Err != nil {
		t.Fatal("undo row was not completed by its durable result")
	}
	if exports != 0 {
		t.Fatal("undo exported dataset")
	}
	notifySchedule.epoch++
	if notifySchedule.timer != nil {
		notifySchedule.timer.Stop()
	}
	// Explicit migration rewrites remain durable, including a second force
	// request arriving during the first rewrite.
	force()
	f1 := waitSOLP2Commit(t, commits)
	force()
	finish(f1)
	f2 := waitSOLP2Commit(t, commits)
	finish(f2)
	t.Logf("SOLP2 guard: recategorise exports=0; undo exports=0; commits/action=1; retry exports=0; idle commits=0; force rewrites=2")
	// Exercise the actual edit/split acknowledgement sites over the same 3,300
	// rows and real per-record encryption, with a controlled strict commit.
	baselineExports := exports
	f := render.New(t)
	defer f.Cleanup()
	editClosed := 0
	f.Render(ui.CreateElement(screens.TransactionEditForm, screens.TransactionEditFormProps{App: a, TxnID: row.ID, OnDone: func() { editClosed++ }}))
	f.ByID("txn-edit-amount").Input("13.00")
	editStart := time.Now()
	f.ByID("txn-edit-form").Submit()
	editCommit := waitSOLP2Commit(t, commits)
	t.Logf("SOLP2 real edit submit to commit request: %.3f ms", float64(time.Since(editStart).Microseconds())/1000)
	if editClosed != 0 {
		t.Fatal("edit closed before durable commit")
	}
	if _, full := editCommit.values[datasetStoreKey]; full || exports != baselineExports {
		t.Fatal("edit save exported the dataset")
	}
	finish(editCommit)
	if editClosed != 1 {
		t.Fatal("durable edit did not close")
	}
	expenseCategories := []string{}
	for _, category := range a.Categories() {
		if category.Kind == domain.KindExpense {
			expenseCategories = append(expenseCategories, category.ID)
		}
	}
	if len(expenseCategories) < 2 {
		t.Fatal("split fixture needs two expense categories")
	}
	uistate.SetTxnSplitProposal(row.ID, []domain.CategorySplit{{CategoryID: expenseCategories[0], Amount: money.New(-650, "USD")}, {CategoryID: expenseCategories[1], Amount: money.New(-650, "USD")}}, "")
	splitClosed := 0
	f.Render(ui.CreateElement(screens.TransactionSplitForm, screens.TransactionSplitFormProps{TxnID: row.ID, OnDone: func() { splitClosed++ }}))
	splitStart := time.Now()
	f.ByID(screens.SplitModalFormID).Submit()
	splitCommit := waitSOLP2Commit(t, commits)
	t.Logf("SOLP2 real split submit to commit request: %.3f ms", float64(time.Since(splitStart).Microseconds())/1000)
	if splitClosed != 0 {
		t.Fatal("split closed before durable commit")
	}
	if _, full := splitCommit.values[datasetStoreKey]; full || exports != baselineExports {
		t.Fatal("split save exported the dataset")
	}
	finish(splitCommit)
	if splitClosed != 1 {
		t.Fatal("durable split did not close")
	}
	// A second mutation arriving during an older commit cannot inherit that
	// commit's acknowledgement, even when persistence is already in flight.
	rowA, _ := a.TransactionByID(row.ID)
	rowA.Note = "First pending write"
	if err := a.PutTransaction(rowA); err != nil {
		t.Fatal(err)
	}
	firstResult, secondResult := make(chan appstate.DurableResult, 1), make(chan appstate.DurableResult, 1)
	a.RequestDurable(func(result appstate.DurableResult) { firstResult <- result })
	firstCommit := waitSOLP2Commit(t, commits)
	rowB := a.Transactions()[1]
	rowB.Note = "Second pending write"
	if err := a.PutTransaction(rowB); err != nil {
		t.Fatal(err)
	}
	a.RequestDurable(func(result appstate.DurableResult) { secondResult <- result })
	finish(firstCommit)
	if result := <-firstResult; result.Err != nil {
		t.Fatal(result.Err)
	}
	select {
	case <-secondResult:
		t.Fatal("older commit acknowledged a newer row")
	default:
	}
	secondCommit := waitSOLP2Commit(t, commits)
	finish(secondCommit)
	if result := <-secondResult; result.Err != nil {
		t.Fatal(result.Err)
	}
	if exports != baselineExports {
		t.Fatal("an overlapping single-record write exported the dataset")
	}
	t.Log("SOLP2 guard: real edit exports=0; real split exports=0; overlapping request waits for its own committed cursor")
	// A complete authorized server pull replaces the base and adopts generation.
	// The writer must rebase rather than attach its previous journal to that base.
	replacement, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	durable[datasetStoreKey], durable[datasetGenKey], durable[datasetHeadKey] = string(replacement), "accepted-pull", ""
	browserstore.SetMemory(datasetStoreKey, string(replacement))
	browserstore.SetMemory(datasetHeadKey, "")
	browserstore.SetMemory(datasetGenKey, "accepted-pull")
	datasetMyGen = "accepted-pull"
	a.RequestDurable(func(result appstate.DurableResult) { firstResult <- result })
	rebase := waitSOLP2Commit(t, commits)
	if rebase.expected[datasetStoreKey] != string(replacement) || rebase.values[datasetHeadKey] != "" || rebase.values[datasetStoreKey] == "" {
		t.Fatal("accepted full replacement retained its obsolete journal")
	}
	finish(rebase)
	if result := <-firstResult; result.Err != nil {
		t.Fatal(result.Err)
	}
	priorGeneration := readDatasetGen()
	browserstore.SetMemory(datasetGenKey, "newer-tab-generation")
	a.RequestDurable(func(result appstate.DurableResult) { firstResult <- result })
	if result := <-firstResult; result.Err == nil {
		t.Fatal("stale tab received an idle success acknowledgement")
	}
	browserstore.SetMemory(datasetGenKey, priorGeneration)
	// Even an otherwise idle writer must not acknowledge an obsolete App scope.
	appstate.Default = nil
	a.RequestDurable(func(result appstate.DurableResult) { firstResult <- result })
	if result := <-firstResult; result.Err == nil {
		t.Fatal("obsolete app received an idle success acknowledgement")
	}
	appstate.Default = a
	rowB.Note = "Cancelled app scope"
	if err := a.PutTransaction(rowB); err != nil {
		t.Fatal(err)
	}
	a.RequestDurable(func(result appstate.DurableResult) { firstResult <- result })
	appstate.Default = nil
	select {
	case result := <-firstResult:
		if result.Err == nil {
			t.Fatal("obsolete scheduled app write succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("obsolete scheduled app write did not finish")
	}
	appstate.Default = a
	select {
	case <-commits:
		t.Fatal("obsolete app dispatched a commit")
	default:
	}
}

func TestSOLP2CheckpointRejectsOldSnapshotReaders(t *testing.T) {
	dataset, err := store.Export(store.SampleDataset())
	if err != nil {
		t.Fatal(err)
	}
	raw := encodeRecordCheckpoint(dataset)
	if _, err := store.Import([]byte(raw)); err == nil {
		t.Fatal("old snapshot reader could open the stale base beneath a record journal")
	}
	decoded, recordFormat, err := checkpointDatasetJSON([]byte(raw))
	if err != nil || !recordFormat {
		t.Fatal("new reader rejected its record checkpoint")
	}
	if _, err := store.Import(decoded); err != nil {
		t.Fatal(err)
	}
	if _, _, err := checkpointDatasetJSON([]byte(`{"schemaVersion":2,"cashfluxRecordCheckpoint":2,"dataset":{}}`)); err == nil {
		t.Fatal("accepted an unknown future journal format")
	}
}

func TestSOLP2PortableWorkspaceCopyUnwrapsCheckpoint(t *testing.T) {
	dataset, err := store.Export(store.SampleDataset())
	if err != nil {
		t.Fatal(err)
	}
	copy, err := portableDatasetFromRecords(encodeRecordCheckpoint(dataset), nil)
	if err != nil {
		t.Fatal(err)
	}
	if copy != string(dataset) {
		t.Fatal("workspace copy retained a device-only checkpoint wrapper")
	}
	if _, err := store.Import([]byte(copy)); err != nil {
		t.Fatalf("portable workspace copy cannot be imported: %v", err)
	}
}

func TestSOLP2CheckpointReplacementAtomicallyInvalidatesJournal(t *testing.T) {
	input := map[string]string{datasetStoreKey: "portable replacement", datasetHeadKey: "obsolete head"}
	updates, expected := prepareWorkspaceCommit(input, "prior-generation")
	if updates[datasetHeadKey] != "" || updates[datasetGenKey] == "" || expected[datasetGenKey] != "prior-generation" {
		t.Fatal("replacement does not atomically replace the checkpoint, journal head and generation with CAS")
	}
	if input[datasetHeadKey] != "obsolete head" || len(input) != 2 {
		t.Fatal("replacement mutated caller's batch")
	}
	updates, expected = prepareWorkspaceCommit(map[string]string{"layout": "same"}, "prior-generation")
	if len(updates) != 1 || len(expected) != 0 {
		t.Fatal("unrelated settings changed the dataset entitlement")
	}
}

func TestSOLP2RecoveryKeepsCheckpointAndJournalWithoutMixingWorkspaces(t *testing.T) {
	dataset, _ := store.Export(store.EmptyDataset())
	checkpoint := encodeRecordCheckpoint(dataset)
	prefix := recordWorkspacePrefix("scope-a")
	foreign := recordWorkspacePrefix("scope-b") + "record"
	values := map[string]string{datasetHeadKey: `{"version":1,"tail":"unreadable"}`, prefix + "one": "exact unreadable row bytes", foreign: "another household"}
	read := func(key string) (string, bool, error) { value, ok := values[key]; return value, ok, nil }
	raw := datasetRecoveryCopy(checkpoint, "scope-a", []string{prefix + "one", foreign}, read)
	var copy recordRecovery
	if err := json.Unmarshal([]byte(raw), &copy); err != nil {
		t.Fatal(err)
	}
	if copy.Checkpoint != checkpoint || copy.Head != values[datasetHeadKey] || copy.Records[prefix+"one"] != values[prefix+"one"] {
		t.Fatal("recovery download lost journal source bytes")
	}
	if _, mixed := copy.Records[foreign]; mixed {
		t.Fatal("recovery mixed another workspace's rows")
	}
	if _, err := store.Import([]byte(raw)); err == nil {
		t.Fatal("diagnostic recovery bundle imported as an empty household")
	}
	if datasetRecoveryCopy(raw, "scope-a", nil, read) != raw {
		t.Fatal("recovery copy recursively wrapped itself")
	}
}

func TestSOLP2RecordChainRejectsDamageAndScopeMixing(t *testing.T) {
	checkpoint := `{"schemaVersion":1}`
	workspace := "workspace-a"
	prefix := recordWorkspacePrefix(workspace) + "commit:"
	head, _ := json.Marshal(datasetRecordHead{Version: 1, Workspace: workspace, BaseHash: checkpointHash(checkpoint), Generation: "gen", Tail: prefix + "batch"})
	node, _ := json.Marshal(datasetRecordNode{Keys: []string{prefix + "0"}})
	record := `{"table":"transactions","id":"row","after":{"id":"row"}}`
	for _, scenario := range []string{"valid", "missing record", "wrong workspace", "wrong checkpoint", "cycle", "record error", "generation mismatch", "obsolete generation"} {
		t.Run(scenario, func(t *testing.T) {
			values := map[string]string{prefix + "batch": string(node), prefix + "0": record}
			raw, scope, generation := checkpoint, workspace, "gen"
			switch scenario {
			case "missing record":
				delete(values, prefix+"0")
			case "wrong workspace":
				scope = "workspace-b"
			case "wrong checkpoint":
				raw = "{}"
			case "cycle":
				values[prefix+"batch"] = `{"parent":"` + prefix + `batch","keys":[]}`
			case "obsolete generation":
				generation = "new"
				raw = `{"schemaVersion":1,"accounts":[]}`
			case "generation mismatch":
				generation = "new"
			}
			got, err := readDatasetRecords(raw, string(head), generation, scope, func(key string) (string, bool, error) {
				if scenario == "record error" {
					return "", false, errors.New("ciphertext authentication failed")
				}
				v, ok := values[key]
				return v, ok, nil
			})
			if scenario == "valid" {
				if err != nil || len(got) != 1 {
					t.Fatalf("%v %+v", err, got)
				}
				return
			}
			if scenario == "obsolete generation" {
				if err != nil || len(got) != 0 {
					t.Fatal("replayed superseded checkpoint")
				}
				return
			}
			if err == nil {
				t.Fatal("accepted damaged or mis-scoped records")
			}
		})
	}
}

func TestSOLP2SlicedMirrorEncodingAndCancellation(t *testing.T) {
	a := solp1App(t)
	data, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := undosnap.ToSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	slices := 0
	encoded, err := encodeSnapshotSliced(snapshot, func() bool { slices++; return true })
	if err != nil {
		t.Fatal(err)
	}
	want, _ := store.Import(data)
	got, err := store.Import(encoded)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("sliced portable snapshot changed data")
	}
	if slices < 100 {
		t.Fatalf("3300 rows used only %d slices", slices)
	}
	if _, err := encodeSnapshotSliced(snapshot, func() bool { return false }); err == nil {
		t.Fatal("obsolete snapshot published")
	}
}

func TestSOLP2NotifyJobProfile3300(t *testing.T) {
	a := solp1App(t)
	job := newNotifyCandidateJob(a, time.Now(), notify.RuleConfig{})
	var longest time.Duration
	steps := 0
	for {
		start := time.Now()
		done := job.Step()
		longest = max(longest, time.Since(start))
		steps++
		if done {
			break
		}
	}
	t.Logf("SOLP2 notification producers: steps=%d longest=%.3f ms", steps, float64(longest.Microseconds())/1000)
	if longest > 50*time.Millisecond {
		t.Fatalf("notification slice exceeded 50 ms: %s", longest)
	}
	if steps < 10 {
		t.Fatal("candidate producers were not separated")
	}
	// No dropped or renamed candidates: the preview and scheduled runner share
	// this exact producer list, including owner routing.
	for _, candidate := range job.candidates {
		if strings.TrimSpace(candidate.RuleID) == "" {
			t.Fatal("candidate lost its rule")
		}
	}
}

func TestSOLP2NotificationObserverDefersAndCancelsObsoleteWork(t *testing.T) {
	a := solp1App(t)
	oldSuspended := suspendAutosave
	suspendAutosave = false
	_ = a.Transactions()
	oldObserve := notifySliceObserve
	t.Cleanup(func() {
		suspendAutosave = oldSuspended
		notifySchedule.epoch++
		if notifySchedule.timer != nil {
			notifySchedule.timer.Stop()
		}
		notifySliceObserve = oldObserve
	})
	notifySchedule.epoch++
	if notifySchedule.timer != nil {
		notifySchedule.timer.Stop()
	}
	steps := make(chan uint64, 4)
	notifySliceObserve = func(time.Duration) {
		select {
		case steps <- notifySchedule.epoch:
		default:
		}
	}
	a.OnTxnMutated(runNotifyCatchUp)
	row := a.Transactions()[0]
	row.Desc = "First revision"
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	select {
	case <-steps:
		t.Fatal("candidate generation ran in the transaction callback")
	default:
	}
	first := notifySchedule.epoch
	row.Desc = "Latest revision"
	if err := a.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	want := notifySchedule.epoch
	if want <= first {
		t.Fatal("new mutation did not supersede the old job")
	}
	select {
	case epoch := <-steps:
		if epoch < want {
			t.Fatal("obsolete notification job executed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deferred job did not start")
	}
}

func TestSOLP2NotificationHighExpenseSlices3300(t *testing.T) {
	a := solp1App(t)
	rows := a.Transactions()
	for i := range rows {
		rows[i].Amount.Amount = -50000
	}
	if err := a.Store().PutTransactionsAtomic(rows); err != nil {
		t.Fatal(err)
	}
	job := newNotifyCandidateJob(a, time.Now(), notify.RuleConfig{})
	var longest time.Duration
	for {
		start := time.Now()
		done := job.Step()
		longest = max(longest, time.Since(start))
		if done {
			break
		}
	}
	t.Logf("SOLP2 high-expense notifications: rows=3300 longest=%s candidates=%d", longest, len(job.candidates))
	if longest > 50*time.Millisecond {
		t.Fatalf("notification slice exceeded 50 ms: %s", longest)
	}
}
