// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/atrest"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

const datasetCheckpointBackupKey = "cashflux:dataset:checkpoint-backup"
const journalRecoveryPrefix = "cashflux:journal-recovery:"
const datasetRecoverySourceKey = "cashflux:dataset:recovery-source"

var errNoDatasetCheckpoint = errors.New("no authenticated dataset checkpoint is readable")

type checkpointBackup struct {
	Workspace  string `json:"workspace"`
	Checkpoint string `json:"checkpoint"`
	Owner      string `json:"owner,omitempty"`
}

func encodeCheckpointBackup(workspace, checkpoint string, owner ...string) string {
	boundOwner := browserstore.GetString(datasetOwnerKey)
	if len(owner) > 0 {
		boundOwner = owner[0]
	}
	raw, _ := json.Marshal(checkpointBackup{Workspace: workspace, Checkpoint: checkpoint, Owner: boundOwner})
	return string(raw)
}

type durableDatasetSnapshot struct {
	browserstore.SecureSnapshot
	workspace string
}

var hydratedDatasetState struct {
	app      *appstate.App
	snapshot durableDatasetSnapshot
}

func snapshotWorkspace(raw string) (string, error) {
	var registry workspace.Registry
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &registry); err != nil {
			return "", fmt.Errorf("read saved workspace: %w", err)
		}
	}
	if registry.ActiveID != "" {
		return registry.ActiveID, nil
	}
	return defaultWSID, nil
}

// Metadata and immutable journal records are read in one transaction. In
// particular, compaction cannot tombstone a node between head and record reads.
var hydrationWaitLimit = 90 * time.Second

func readDurableDatasetSnapshot() (durableDatasetSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hydrationWaitLimit)
	defer cancel()
	return readDurableDatasetSnapshotContext(ctx)
}

func readDurableDatasetSnapshotContext(ctx context.Context, recovery ...bool) (durableDatasetSnapshot, error) {
	scope := recordWorkspace()
	for range 3 {
		keys := []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey, datasetRecoverySourceKey}
		prefixes := []string{recordWorkspacePrefix(scope)}
		if len(recovery) > 0 && recovery[0] {
			keys = append(keys, datasetCheckpointBackupKey)
		}
		if len(recovery) > 1 && recovery[1] {
			prefixes = append(prefixes, quarantinePrefix, journalRecoveryPrefix)
		}
		snapshot, err := browserstore.ReadSecureSnapshotContext(ctx, keys, prefixes)
		result := durableDatasetSnapshot{snapshot, scope}
		if err != nil {
			return result, err
		}
		for _, key := range []string{workspacesKey, datasetOwnerKey} {
			if err := snapshot.Errors[key]; err != nil {
				return result, err
			}
		}
		actual, err := snapshotWorkspace(snapshot.Values[workspacesKey])
		if err != nil {
			return result, err
		}
		if actual == scope {
			if len(recovery) == 0 && snapshot.Values[datasetStoreKey] == "" {
				// A missing live checkpoint is an actual recovery case. Read
				// its backup coherently instead of seeding over it.
				return readDurableDatasetSnapshotContext(ctx, true)
			}
			return result, nil
		}
		scope = actual
	}
	return durableDatasetSnapshot{}, errors.New("the workspace changed while reading saved data")
}

func authenticatedCheckpoint(raw string) ([]byte, *appstate.ParsedDataset, error) {
	if cryptobox.IsEnvelope([]byte(raw)) {
		if activePasscode == "" {
			return nil, nil, errSyncDatasetLocked
		}
		plain, err := decryptEnvelopeSync([]byte(raw), activePasscode)
		if err != nil {
			return nil, nil, fmt.Errorf("open legacy checkpoint: %w", err)
		}
		raw = string(plain)
	}
	data, _, err := checkpointDatasetJSON([]byte(raw))
	if err != nil {
		return nil, nil, err
	}
	parsed, err := appstate.ParseDataset(data)
	if err != nil {
		return nil, nil, fmt.Errorf("validate saved checkpoint: %w", err)
	}
	return data, parsed, nil
}

// recoverDatasetSnapshot prefers the live checkpoint (the most recently CAS
// published checkpoint), then its authenticated same-workspace backup. It never
// orders UUID generations or wall clocks. Legacy journals are anchored by the
// authenticated head's base hash; new nodes additionally chain generations and
// logical cursors. Unusable branches remain in the retained recovery bundle.
func recoverDatasetSnapshot(s durableDatasetSnapshot) (string, bool, error) {
	portable, _, repair, err := recoverDatasetSnapshotParsed(s)
	return portable, repair, err
}

// The parsed handle is valid only for the original authenticated checkpoint.
// Any replay clears it so hydration cannot accidentally load the journal's base.
func recoverDatasetSnapshotParsed(s durableDatasetSnapshot) (string, *appstate.ParsedDataset, bool, error) {
	checkpoint := s.Values[datasetStoreKey]
	data, parsed, err := authenticatedCheckpoint(checkpoint)
	repair := s.Errors[datasetStoreKey] != nil || s.Errors[datasetGenKey] != nil
	if err != nil || s.Errors[datasetStoreKey] != nil {
		var backup checkpointBackup
		if s.Errors[datasetCheckpointBackupKey] != nil || json.Unmarshal([]byte(s.Values[datasetCheckpointBackupKey]), &backup) != nil || backup.Workspace != s.workspace || backup.Owner != s.Values[datasetOwnerKey] {
			return "", nil, false, fmt.Errorf("%w: %v", errNoDatasetCheckpoint, errors.Join(err, s.Errors[datasetStoreKey]))
		}
		checkpoint = backup.Checkpoint
		data, parsed, err = authenticatedCheckpoint(checkpoint)
		if err != nil {
			return "", nil, false, fmt.Errorf("%w: %v", errNoDatasetCheckpoint, err)
		}
		repair = true
	}
	var head datasetRecordHead
	baseDataset := string(data)
	headRaw := s.Values[datasetHeadKey]
	if s.Errors[datasetHeadKey] != nil || (headRaw != "" && json.Unmarshal([]byte(headRaw), &head) != nil) {
		headRaw, head = "", datasetRecordHead{}
		repair = true
	}
	if headRaw == "" || head.Version != 1 || head.Workspace != s.workspace || head.BaseHash != checkpointHash(checkpoint) {
		if recovered, ok := recoverJournalHead(s, checkpoint); ok {
			head = recovered
			raw, _ := json.Marshal(head)
			headRaw = string(raw)
			repair = true
		}
	}
	if headRaw == "" {
		if generation := checkpointGeneration(checkpoint); generation != "" && generation != s.Values[datasetGenKey] {
			repair = true
		}
		return string(data), parsed, repair || hasUnclaimedJournal(s, nil), nil
	}
	if head.Version == 1 && head.Workspace != s.workspace && head.BaseHash == checkpointHash(checkpoint) {
		// A matching authenticated head binds this checkpoint to the other
		// workspace. Without an independently anchored local chain, do not
		// relabel its household as the newly active workspace.
		return "", nil, false, fmt.Errorf("the saved checkpoint belongs to another workspace: %w", appstate.ErrStaleDataset)
	}
	if head.Generation != s.Values[datasetGenKey] {
		repair = true
	}
	if head.Version != 1 || head.Workspace != s.workspace || head.BaseHash != checkpointHash(checkpoint) {
		return string(data), parsed, true, nil
	}
	if records, err := readDatasetRecords(checkpoint, headRaw, head.Generation, s.workspace, s.Read); err == nil {
		portable, err := portableDatasetFromRecords(baseDataset, records)
		if err == nil {
			keys, keyErr := journalKeysFromHead(head, s.workspace, s.Read)
			claimed := map[string]bool{}
			for _, key := range keys {
				claimed[key] = true
			}
			return portable, nil, repair || keyErr != nil || hasUnclaimedJournal(s, claimed), nil
		}
	}
	repair = true
	// Walk backwards first, then validate/replay oldest to newest. If the tail
	// is damaged, independently authenticated earlier nodes can still be used.
	nodes, seen, complete := readRecoveryNodes(s, head)
	if !complete {
		if recovered, ok := recoverJournalHead(s, checkpoint); ok && recovered.Tail != head.Tail {
			head = recovered
			nodes, seen, _ = readRecoveryNodes(s, head)
		}
	}
	var changes []store.RecordChange
	claimed := map[string]bool{}
	priorGeneration := checkpointGeneration(checkpoint)
	var cursor uint64
	for i := len(nodes) - 1; i >= 0; i-- {
		node := nodes[i]
		if node.Generation != "" {
			if node.Workspace != s.workspace || node.BaseHash != checkpointHash(checkpoint) || node.From != cursor || node.Through != node.From+1 || (priorGeneration != "" && node.ParentGeneration != priorGeneration) {
				repair = true
				break
			}
		}
		var batch []store.RecordChange
		valid := true
		for _, key := range node.Keys {
			if claimed[key] || seen[key] || !strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
				valid = false
				break
			}
			raw, present, err := s.Read(key)
			var record store.RecordChange
			if err != nil || !present || json.Unmarshal([]byte(raw), &record) != nil {
				valid = false
				break
			}
			claimed[key] = true
			batch = append(batch, record)
		}
		if !valid {
			repair = true
			break
		}
		// SQLite validates the complete batch atomically, including tables and
		// row constraints. Keep the preceding valid prefix on a bad later batch.
		candidate := append(append([]store.RecordChange(nil), changes...), batch...)
		portable, err := portableDatasetFromRecords(baseDataset, candidate)
		if err != nil {
			repair = true
			break
		}
		data, changes, parsed = []byte(portable), candidate, nil
		priorGeneration, cursor = node.Generation, node.Through
	}
	if priorGeneration != "" && (priorGeneration != head.Generation || cursor != head.Cursor) {
		repair = true
	}
	for key := range seen {
		claimed[key] = true
	}
	return string(data), parsed, repair || hasUnclaimedJournal(s, claimed), nil
}

func readRecoveryNodes(s durableDatasetSnapshot, head datasetRecordHead) ([]datasetRecordNode, map[string]bool, bool) {
	var nodes []datasetRecordNode
	seen := map[string]bool{}
	for key := head.Tail; key != ""; {
		if seen[key] || !strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
			return nil, seen, false
		}
		seen[key] = true
		raw, have, err := s.Read(key)
		var node datasetRecordNode
		if err != nil || !have || raw == "" || json.Unmarshal([]byte(raw), &node) != nil {
			return nil, seen, false
		}
		nodes = append(nodes, node)
		key = node.Parent
	}
	return nodes, seen, true
}

// New nodes carry their own authenticated checkpoint and causal links. With no
// usable head, replay only the unique prefix; a fork stops at its common parent.
func recoverJournalHead(s durableDatasetSnapshot, checkpoint string) (datasetRecordHead, bool) {
	base := checkpointHash(checkpoint)
	head := datasetRecordHead{Version: 1, Workspace: s.workspace, BaseHash: base, Generation: checkpointGeneration(checkpoint)}
	for {
		var next string
		var chosen datasetRecordNode
		for key, raw := range s.Values {
			if !strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) || !strings.HasSuffix(key, ":batch") || raw == "" {
				continue
			}
			var node datasetRecordNode
			if json.Unmarshal([]byte(raw), &node) != nil || node.Generation == "" || node.BaseHash != base || node.Workspace != s.workspace || node.Parent != head.Tail || node.From != head.Cursor || node.Through != node.From+1 || (head.Generation != "" && node.ParentGeneration != head.Generation) {
				continue
			}
			if next != "" {
				return head, head.Tail != ""
			}
			next, chosen = key, node
		}
		if next == "" {
			return head, head.Tail != ""
		}
		head.Tail, head.Generation, head.Cursor = next, chosen.Generation, chosen.Through
	}
}

func hasUnclaimedJournal(s durableDatasetSnapshot, claimed map[string]bool) bool {
	for key, value := range s.Values {
		if strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) && value != "" && !claimed[key] {
			return true
		}
	}
	for key := range s.Errors {
		if strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
			return true
		}
	}
	return false
}

func snapshotRecoveryCopy(s durableDatasetSnapshot) string {
	keys := make([]string, 0, len(s.Values)+len(s.Errors))
	for key := range s.Values {
		keys = append(keys, key)
	}
	for key := range s.Errors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var copy recordRecovery
	_ = json.Unmarshal([]byte(datasetRecoveryCopy(s.Values[datasetStoreKey], s.workspace, keys, s.Read)), &copy)
	copy.SchemaVersion, copy.Format, copy.Workspace = store.SchemaVersion+1, "v1", s.workspace
	copy.Checkpoint, copy.Head, copy.Generation, copy.Backup = s.Values[datasetStoreKey], s.Values[datasetHeadKey], s.Values[datasetGenKey], s.Values[datasetCheckpointBackupKey]
	copy.Owner = s.Values[datasetOwnerKey]
	if copy.ReadErrors == nil {
		copy.ReadErrors = map[string]string{}
	}
	copy.ProtectedRecords = map[string]string{}
	for key, err := range s.Errors {
		if key == datasetStoreKey || key == datasetHeadKey || key == datasetGenKey || key == datasetCheckpointBackupKey || strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
			copy.ReadErrors[key] = err.Error()
			if raw, present := s.EncodedRecord(key); present {
				copy.ProtectedRecords[key] = raw
			}
		}
	}
	raw, _ := json.Marshal(copy)
	return string(raw)
}

// hydrateDurableDataset repairs only under the exact checkpoint/head/generation
// CAS it read. The recovery copy and repaired checkpoint commit together before
// hydration releases autosave. A concurrent winner forces a fresh snapshot.
func hydrateDurableDataset(app *appstate.App, s durableDatasetSnapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), hydrationWaitLimit)
	defer cancel()
	for range 3 {
		if appstate.Default != app || recordWorkspace() != s.workspace {
			return appstate.ErrStaleDataset
		}
		app.Log().Info("dataset hydration stage", "stage", "replay", "status", "started", "workspace", s.workspace)
		portable, parsed, repair, err := recoverDatasetSnapshotParsed(s)
		// The backup is deliberately excluded from normal hydration. Only an
		// unreadable checkpoint or a repair needs its bytes and ciphertext CAS.
		if err != nil || repair {
			if _, have := s.EncodedRecord(datasetCheckpointBackupKey); !have && s.Errors[datasetCheckpointBackupKey] == nil {
				extended, readErr := browserstore.ExtendSecureSnapshotContext(ctx, s.SecureSnapshot, []string{datasetCheckpointBackupKey}, []string{datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey})
				if errors.Is(readErr, atrest.ErrRecordChanged) {
					next, retryErr := readDurableDatasetSnapshotContext(ctx)
					if retryErr != nil {
						return retryErr
					}
					if next.workspace != s.workspace || next.Values[datasetOwnerKey] != s.Values[datasetOwnerKey] {
						return appstate.ErrStaleDataset
					}
					s = next
					continue
				}
				if readErr != nil {
					return readErr
				}
				s.SecureSnapshot = extended
				// A valid live checkpoint already supplied the replayed
				// dataset. The added backup is only a guarded repair input.
				if err != nil {
					portable, parsed, repair, err = recoverDatasetSnapshotParsed(s)
				}
			}
		}
		if err != nil {
			return err
		}
		app.Log().Info("dataset hydration stage", "stage", "replay", "status", "complete", "repair", repair)
		if err := ctx.Err(); err != nil {
			return err
		}
		if repair {
			app.Log().Info("dataset hydration stage", "stage", "repair", "status", "started")
			copy := snapshotRecoveryCopy(s)
			key := journalRecoveryPrefix + id.New()
			generation := id.New()
			checkpoint := encodeRecordCheckpointGeneration([]byte(portable), generation)
			values := map[string]string{key: copy, datasetStoreKey: checkpoint, datasetHeadKey: "", datasetGenKey: generation, datasetCheckpointBackupKey: encodeCheckpointBackup(s.workspace, checkpoint, s.Values[datasetOwnerKey])}
			conditions := []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey, datasetCheckpointBackupKey}
			for key, value := range s.Values {
				if strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
					// Existing tombstones need only a ciphertext condition.
					// Re-encrypting empty padding adds no recovery protection
					// and makes repair cost grow with old journal history.
					if value != "" {
						values[key] = ""
					}
					conditions = append(conditions, key)
				}
			}
			for key := range s.Errors {
				if strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
					values[key] = ""
					conditions = append(conditions, key)
				}
			}
			done := make(chan error, 1)
			browserstore.SetSecureSnapshotBatchContext(ctx, values, s.SecureSnapshot, conditions, func(err error) { done <- err })
			if err := awaitHydrationCommit(ctx, done); err != nil {
				if ctx.Err() != nil {
					return fmt.Errorf("journal repair paused: %w", err)
				}
				next, readErr := readDurableDatasetSnapshotContext(ctx)
				if readErr != nil {
					return readErr
				}
				if next.workspace != s.workspace || next.Values[datasetOwnerKey] != s.Values[datasetOwnerKey] {
					return appstate.ErrStaleDataset
				}
				if next.Values[datasetGenKey] == s.Values[datasetGenKey] {
					return fmt.Errorf("retain journal recovery: %w", err)
				}
				s = next
				continue
			}
			app.Log().Info("dataset hydration stage", "stage", "repair", "status", "complete")
			app.Log().Warn("saved dataset recovered; unchained records retained", "key", key, "workspace", s.workspace)
			quarantineRaw, quarantineAvailable = copy, true
			for key, value := range values {
				s.Values[key] = value
				delete(s.Errors, key)
			}
			// The repaired chain is empty; old immutable records are retained only
			// for recovery and must not trigger another repair on every boot.
			for key := range s.Values {
				if strings.HasPrefix(key, recordWorkspacePrefix(s.workspace)) {
					delete(s.Values, key)
				}
			}
		}
		if appstate.Default != app || recordWorkspace() != s.workspace {
			return appstate.ErrStaleDataset
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		app.Log().Info("dataset hydration stage", "stage", "import", "status", "started")
		if parsed != nil {
			err = app.ImportParsedDataset(parsed)
		} else {
			err = app.ImportJSON([]byte(portable))
		}
		if err != nil {
			return fmt.Errorf("load recovered dataset: %w", err)
		}
		app.Log().Info("dataset hydration stage", "stage", "import", "status", "complete")
		hydratedRecordApp = app
		hydratedRecordCursor, _ = app.Store().ChangeCursor()
		hydratedRecordRevision = app.Rev()
		hydratedDatasetState.app, hydratedDatasetState.snapshot = app, s
		datasetMyGen = s.Values[datasetGenKey]
		for _, key := range []string{datasetStoreKey, datasetHeadKey, datasetGenKey} {
			browserstore.SetMemory(key, s.Values[key])
		}
		return nil
	}
	return appstate.ErrStaleDataset
}

func loadJournalRecoveryCopies(snapshots ...durableDatasetSnapshot) {
	// Names are sufficient for the recovery affordance. Authentication and
	// scope checks happen when the user requests the download.
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, journalRecoveryPrefix) {
			quarantineAvailable = true
			showQuarantineBanner()
			return
		}
	}
}

func recoveryCopyMatchesScope(copy recordRecovery, workspace, owner string, hosted bool) bool {
	return copy.Format == "v1" && copy.Workspace == workspace && copy.Owner == owner && (copy.Owner != "" || !hosted)
}

// Old quarantine bundles have no generation field, but their authenticated head
// still binds every legacy record to the original checkpoint. Automatic recovery
// is limited to one unambiguous non-sample household in the current workspace.
// Distinct viable households require a user choice; timestamps never pick one.
func tryRecoverQuarantinedDataset(app *appstate.App, live durableDatasetSnapshot) bool {
	livePortable, _, err := recoverDatasetSnapshot(live)
	if err != nil {
		return false
	}
	liveDataset, err := store.Import([]byte(livePortable))
	if err != nil || liveDataset.KV["cashflux:sampleActive"] != "1" {
		return false
	}
	ctxRead, cancelRead := context.WithTimeout(context.Background(), hydrationWaitLimit)
	defer cancelRead()
	recovery, readErr := readDurableDatasetSnapshotContext(ctxRead, true, true)
	if readErr != nil || recovery.workspace != live.workspace || recovery.Values[datasetGenKey] != live.Values[datasetGenKey] {
		return false
	}
	live = recovery
	var selected, source string
	for key, raw := range live.Values {
		if !strings.HasPrefix(key, quarantinePrefix) || strings.HasSuffix(key, ":reason") || raw == "" || key == live.Values[datasetRecoverySourceKey] {
			continue
		}
		var copy recordRecovery
		if json.Unmarshal([]byte(raw), &copy) != nil || !recoveryCopyMatchesScope(copy, live.workspace, live.Values[datasetOwnerKey], hostedDocument()) {
			continue
		}
		values := map[string]string{datasetStoreKey: copy.Checkpoint, datasetHeadKey: copy.Head, datasetGenKey: copy.Generation, datasetCheckpointBackupKey: copy.Backup}
		for record, value := range copy.Records {
			if strings.HasPrefix(record, recordWorkspacePrefix(live.workspace)) {
				values[record] = value
			}
		}
		s := durableDatasetSnapshot{browserstore.SecureSnapshot{Values: values, Errors: map[string]error{}}, live.workspace}
		portable, _, err := recoverDatasetSnapshot(s)
		if err != nil {
			continue
		}
		if _, err := store.Import([]byte(portable)); err != nil || checkpointHash(portable) == checkpointHash(livePortable) {
			continue
		}
		if selected != "" && checkpointHash(selected) != checkpointHash(portable) {
			app.Log().Warn("multiple saved households are available for recovery; no automatic replacement was made")
			return false
		}
		selected, source = portable, key
	}
	if selected == "" {
		return false
	}
	generation := id.New()
	checkpoint := encodeRecordCheckpointGeneration([]byte(selected), generation)
	values := map[string]string{datasetStoreKey: checkpoint, datasetHeadKey: "", datasetGenKey: generation, datasetCheckpointBackupKey: encodeCheckpointBackup(live.workspace, checkpoint), datasetRecoverySourceKey: source}
	// Preserve the complete sample checkpoint/journal too. It can contain edits
	// made while the old application still displayed the sample-data banner.
	values[journalRecoveryPrefix+id.New()] = snapshotRecoveryCopy(live)
	conditions := []string{datasetStoreKey, datasetHeadKey, datasetGenKey, datasetOwnerKey, workspacesKey, source, datasetRecoverySourceKey}
	ctx, cancel := context.WithTimeout(context.Background(), hydrationWaitLimit)
	defer cancel()
	done := make(chan error, 1)
	browserstore.SetSecureSnapshotBatchContext(ctx, values, live.SecureSnapshot, conditions, func(err error) { done <- err })
	if err := awaitHydrationCommit(ctx, done); err != nil {
		app.Log().Warn("saved quarantine recovery was not applied", "err", err)
		return false
	}
	for key, value := range values {
		live.Values[key] = value
	}
	if err := app.ImportJSON([]byte(selected)); err != nil {
		app.Log().Error("load retained household", "err", err)
		return false
	}
	datasetQuarantined, quarantinePending = false, false
	hadLocalDataset, hydrationSucceeded = true, true
	hydratedRecordApp, hydratedRecordRevision = app, app.Rev()
	hydratedRecordCursor, _ = app.Store().ChangeCursor()
	hydratedDatasetState.app, hydratedDatasetState.snapshot = app, live
	datasetMyGen = generation
	app.Log().Warn("household restored from retained quarantine copy", "key", source, "workspace", live.workspace)
	return true
}

// Cancellation never authorizes a late queued repair: the same context follows
// the job through encryption and into its strict transaction.
func awaitHydrationCommit(ctx context.Context, done <-chan error) error {
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func loadSnapshotQuarantine(snapshot durableDatasetSnapshot) {
	for _, key := range browserstore.Keys() {
		if strings.HasPrefix(key, quarantinePrefix) && !strings.HasSuffix(key, ":reason") {
			quarantineAvailable = true
			showQuarantineBanner()
			return
		}
	}
}
