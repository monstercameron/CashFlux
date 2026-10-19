//go:build js && wasm

package browserstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/atrest"
)

func bootDescribe(key, raw string) RecoveryDescription {
	group := "quarantine"
	if strings.HasPrefix(key, "cashflux:stale-recovery:") {
		group = "stale"
	}
	return RecoveryDescription{Group: group, Hash: recoveryHash(raw)}
}

func bootRetain(t *testing.T, key, raw string, describe RecoveryDescribe, live RecoveryLive) (string, error) {
	t.Helper()
	done := make(chan error, 1)
	var actual string
	RetainRecovery(key, raw, "synthetic reason", describe, live, func(k string, err error) { actual = k; done <- err })
	err := waitResult(t, done)
	return actual, err
}

func bootPrune(t *testing.T, describe RecoveryDescribe, live RecoveryLive) (RecoveryStats, error) {
	t.Helper()
	done := make(chan error, 1)
	var stats RecoveryStats
	PruneRecovery(describe, live, func(s RecoveryStats, err error) { stats = s; done <- err })
	select {
	case err := <-done:
		return stats, err
	case <-time.After(2 * time.Minute):
		t.Fatal("BOOT background migration did not settle")
		return stats, nil
	}
}

func TestBOOTRecoveryAddCapsDeduplicatesAndCommitsAtomically(t *testing.T) {
	initFixture(t)
	const prefix = "cashflux:quarantine:"
	for i := 1; i <= 4; i++ {
		key := fmt.Sprintf("%s%d", prefix, i)
		if _, err := bootRetain(t, key, fmt.Sprint(i), bootDescribe, RecoveryLive{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{prefix + "1", prefix + "1:reason"} {
		if _, have, _ := GetSecure(key); have {
			t.Fatalf("old copy survived: %s", key)
		}
	}
	actual, err := bootRetain(t, prefix+"5", "4", bootDescribe, RecoveryLive{})
	if err != nil || actual != prefix+"4" {
		t.Fatalf("duplicate=%s %v", actual, err)
	}
	if _, have, _ := GetSecure(prefix + "5"); have {
		t.Fatal("duplicate added another ciphertext")
	}
	// The next add would delete copy 2. A failed strict transaction must retain
	// copy 2, its reason and the previous index, and must not acknowledge copy 6.
	before, _, _ := GetSecure(recoveryIndexKey)
	js.Global().Get("__fakeIDB").Call("failNextWrite", "QuotaExceededError")
	if _, err := bootRetain(t, prefix+"6", "6", bootDescribe, RecoveryLive{}); err == nil {
		t.Fatal("aborted retention acknowledged")
	}
	after, _, _ := GetSecure(recoveryIndexKey)
	if after != before {
		t.Fatal("aborted retention published its index")
	}
	if raw, have, _ := GetSecure(prefix + "2"); !have || raw != "2" {
		t.Fatal("aborted add pruned the previous recovery")
	}
	if _, have, _ := GetSecure(prefix + "6"); have {
		t.Fatal("aborted add was partially committed")
	}
}

func TestBOOTBackgroundMigrationKeepsNewestDistinctCopies(t *testing.T) {
	initFixture(t)
	values := map[string]string{"cashflux:dataset": "MUST NOT CHANGE", "cashflux:dataset:checkpoint-backup": "MUST NOT CHANGE EITHER"}
	padding := strings.Repeat("synthetic", 2048)
	for i := 0; i < 126; i++ {
		values[fmt.Sprintf("cashflux:stale-recovery:legacy:%03d", i)] = padding
	}
	for i := 0; i < 8; i++ {
		values[fmt.Sprintf("cashflux:quarantine:%d", 1700000000+i)] = fmt.Sprintf("q%d%s", i, padding)
	}
	prepared := make(chan error, 1)
	SetSecureBatch(values, nil, func(err error) { prepared <- err })
	select {
	case err := <-prepared:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Minute):
		t.Fatal("synthetic recovery history did not commit")
	}

	before, err := readSecureBatch([]string{"cashflux:dataset", "cashflux:dataset:checkpoint-backup"})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat := make(chan struct{}, 1)
	timer := time.AfterFunc(time.Millisecond, func() { heartbeat <- struct{}{} })
	defer timer.Stop()
	stats, err := bootPrune(t, bootDescribe, RecoveryLive{})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Removed != 130 || stats.BytesFreed == 0 {
		t.Fatalf("migration stats=%+v, want 130 removals", stats)
	}
	select {
	case <-heartbeat:
	default:
		t.Fatal("background migration starved timers")
	}
	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("cashflux:quarantine:%d", 1700000000+i)
		_, have, _ := GetSecure(key)
		if have != (i >= 5) {
			t.Fatalf("quarantine %d present=%v", i, have)
		}
	}
	if _, have, _ := GetSecure("cashflux:stale-recovery:legacy:125"); !have {
		t.Fatal("newest duplicate missing")
	}
	after, err := readSecureBatch([]string{"cashflux:dataset", "cashflux:dataset:checkpoint-backup"})
	if err != nil {
		t.Fatal(err)
	}
	for key, raw := range before {
		if after[key] != raw {
			t.Fatalf("migration touched %s", key)
		}
	}
	stats, err = bootPrune(t, bootDescribe, RecoveryLive{})
	if err != nil || stats.Removed != 0 {
		t.Fatalf("migration was not idempotent: %+v %v", stats, err)
	}
}

func TestBOOTUnrepresentedStaleEditAndOtherOwnerSurvive(t *testing.T) {
	initFixture(t)
	describe := func(key, raw string) RecoveryDescription {
		group := "workspace/owner-a"
		if strings.Contains(key, "owner-b") {
			group = "workspace/owner-b"
		}
		return RecoveryDescription{Group: group, Hash: recoveryHash(raw), Protect: true, Versions: []string{raw}}
	}
	for i := 1; i <= 4; i++ {
		if _, err := bootRetain(t, fmt.Sprintf("cashflux:stale-recovery:owner-a:%d", i), fmt.Sprint(i), describe, RecoveryLive{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := bootRetain(t, "cashflux:stale-recovery:owner-b:1", "1", describe, RecoveryLive{}); err != nil {
		t.Fatal(err)
	}
	stats, err := bootPrune(t, describe, RecoveryLive{Versions: map[string][]string{"workspace/owner-b": {"1"}}})
	if err != nil || stats.Removed != 0 {
		t.Fatalf("unrepresented stale edit pruned: %+v %v", stats, err)
	}
	if raw, have, _ := GetSecure("cashflux:stale-recovery:owner-a:1"); !have || raw != "1" {
		t.Fatal("only copy of stale edit lost")
	}
	stats, err = bootPrune(t, describe, RecoveryLive{Versions: map[string][]string{"workspace/owner-a": {"1"}}})
	if err != nil || stats.Removed != 1 {
		t.Fatalf("live record version did not release oldest copy: %+v %v", stats, err)
	}
	if _, have, _ := GetSecure("cashflux:stale-recovery:owner-b:1"); !have {
		t.Fatal("other owner was pruned")
	}
}

func TestBOOTLiveContainmentProofComparedInsideRetentionTransaction(t *testing.T) {
	initFixture(t)
	describe := func(key, raw string) RecoveryDescription {
		return RecoveryDescription{Group: "scope", Hash: raw, Protect: true, Versions: []string{raw}}
	}
	for i := 1; i <= 4; i++ {
		if _, err := bootRetain(t, fmt.Sprintf("cashflux:stale-recovery:scope:%d", i), fmt.Sprint(i), describe, RecoveryLive{}); err != nil {
			t.Fatal(err)
		}
	}
	put(t, "cashflux:dataset:gen", "before")
	snapshot, err := ReadSecureSnapshot([]string{"cashflux:dataset:gen"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	put(t, "cashflux:dataset:gen", "after")
	stats, err := bootPrune(t, describe, RecoveryLive{Versions: map[string][]string{"scope": {"1"}}, Snapshot: snapshot, Conditions: []string{"cashflux:dataset:gen"}})
	// The stale proof is rejected; retry without proof preserves the unique edit.
	if err != nil || stats.Removed != 0 {
		t.Fatalf("stale containment proof deleted recovery: %+v %v", stats, err)
	}
	if _, have, _ := GetSecure("cashflux:stale-recovery:scope:1"); !have {
		t.Fatal("stale proof pruned an unrepresented edit")
	}
}

func TestBOOTBackgroundScanDoesNotBlockDurableWrites(t *testing.T) {
	initFixture(t)
	put(t, "cashflux:quarantine:1", "old recovery")
	scanning, release, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	describe := func(key, raw string) RecoveryDescription { close(scanning); <-release; return bootDescribe(key, raw) }
	PruneRecovery(describe, RecoveryLive{}, func(_ RecoveryStats, err error) { finished <- err })
	<-scanning
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("background worker failed to stop")
		}
	})
	write := make(chan error, 1)
	SetSecure("cashflux:ordinary-write", "committed while scanning", func(err error) { write <- err })
	select {
	case err := <-write:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("background scan occupied the serial writer")
	}
	close(release)
}

func TestBOOTEmptyJournalCleanupKeepsRecoveryAndCheckpointBytes(t *testing.T) {
	initFixture(t)
	values := map[string]string{"cashflux:journal-recovery:empty": "", "cashflux:journal-recovery:kept": "only recovery edit", "cashflux:dataset:checkpoint-backup": ""}
	if err := jrnlBatch(t, values, nil); err != nil {
		t.Fatal(err)
	}
	before, _, _ := (idbRecords{}).Read("cashflux:dataset:checkpoint-backup")
	stats, err := PruneEmptyJournalRecovery()
	if err != nil || stats.Removed != 1 {
		t.Fatalf("empty cleanup=%+v %v", stats, err)
	}
	if raw, have, _ := GetSecure("cashflux:journal-recovery:kept"); !have || raw != "only recovery edit" {
		t.Fatal("nonempty recovery was removed")
	}
	after, have, _ := (idbRecords{}).Read("cashflux:dataset:checkpoint-backup")
	if !have || after != before {
		t.Fatal("dataset backup was touched")
	}
}

func TestBOOTSnapshotExtensionGuardsDeferredBackup(t *testing.T) {
	initFixture(t)
	if err := jrnlBatch(t, map[string]string{"cashflux:dataset": "checkpoint", "cashflux:dataset:gen": "g1", "cashflux:dataset:checkpoint-backup": "backup", "cashflux:dataset:record:test:one": "row"}, nil); err != nil {
		t.Fatal(err)
	}
	original, err := ReadSecureSnapshot([]string{"cashflux:dataset", "cashflux:dataset:gen"}, []string{"cashflux:dataset:record:test:"})
	if err != nil {
		t.Fatal(err)
	}
	extended, err := ExtendSecureSnapshotContext(context.Background(), original, []string{"cashflux:dataset:checkpoint-backup"}, []string{"cashflux:dataset:gen"})
	if err != nil || extended.Values["cashflux:dataset:checkpoint-backup"] != "backup" || extended.Values["cashflux:dataset:record:test:one"] != "row" {
		t.Fatalf("backup extension=%+v %v", extended, err)
	}
	put(t, "cashflux:dataset:gen", "g2")
	if _, err := ExtendSecureSnapshotContext(context.Background(), original, []string{"cashflux:dataset:checkpoint-backup"}, []string{"cashflux:dataset:gen"}); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("changed live guard accepted: %v", err)
	}
}

func TestBOOTDuplicateObservationRefreshesDistinctRetentionOrder(t *testing.T) {
	initFixture(t)
	for i := 1; i <= 3; i++ {
		if _, err := bootRetain(t, fmt.Sprintf("cashflux:quarantine:%d", i), fmt.Sprint(i), bootDescribe, RecoveryLive{}); err != nil {
			t.Fatal(err)
		}
	}
	key, err := bootRetain(t, "cashflux:quarantine:4", "1", bootDescribe, RecoveryLive{})
	if err != nil || key != "cashflux:quarantine:1" {
		t.Fatalf("duplicate=%s %v", key, err)
	}
	if _, err := bootRetain(t, "cashflux:quarantine:5", "5", bootDescribe, RecoveryLive{}); err != nil {
		t.Fatal(err)
	}
	if _, have, _ := GetSecure("cashflux:quarantine:2"); have {
		t.Fatal("oldest distinct observation was not pruned")
	}
	if raw, have, _ := GetSecure("cashflux:quarantine:1"); !have || raw != "1" {
		t.Fatal("most recently observed content was pruned")
	}
}

func TestBOOTExplicitRecoverySnapshotMigratesColdLegacyBackup(t *testing.T) {
	initFixture(t)
	if err := jrnlBatch(t, map[string]string{"cashflux:dataset": "checkpoint", "cashflux:dataset:gen": "g1"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := (idbRecords{}).Replace("cashflux:dataset:checkpoint-backup", "legacy backup", nil); err != nil {
		t.Fatal(err)
	}
	original, err := ReadSecureSnapshot([]string{"cashflux:dataset", "cashflux:dataset:gen"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	extended, err := ExtendSecureSnapshotContext(context.Background(), original, []string{"cashflux:dataset:checkpoint-backup"}, []string{"cashflux:dataset:gen"})
	if err != nil || extended.Values["cashflux:dataset:checkpoint-backup"] != "legacy backup" {
		t.Fatalf("cold legacy backup not recovered: %+v %v", extended, err)
	}
	stored, have, err := (idbRecords{}).Read("cashflux:dataset:checkpoint-backup")
	if err != nil || !have || !atrest.IsSealed(stored) {
		t.Fatal("explicit recovery did not verify encryption migration")
	}
}

func TestBOOTReadOnlySnapshotConditionIsFetchedOnlyInsideStrictCommit(t *testing.T) {
	initFixture(t)
	const condition = "cashflux:quarantine:read-only-condition"
	put(t, condition, "retained proof")
	snapshot, err := ReadSecureSnapshot([]string{condition}, nil)
	if err != nil {
		t.Fatal(err)
	}
	reads := TraceRecoveryReadsForTest(t)
	done := make(chan error, 1)
	SetSecureSnapshotBatch(map[string]string{"cashflux:dataset:gen": "new"}, snapshot, []string{condition}, func(err error) { done <- err })
	if err := waitResult(t, done); err != nil {
		t.Fatal(err)
	}
	values, all := reads()
	if values != 1 || all != 0 {
		t.Fatalf("read-only condition fetched %d times (getAll %d), want final CAS only", values, all)
	}
	put(t, condition, "another tab changed the proof")
	SetSecureSnapshotBatch(map[string]string{"cashflux:dataset:gen": "must not commit"}, snapshot, []string{condition}, func(err error) { done <- err })
	if err := waitResult(t, done); !errors.Is(err, atrest.ErrRecordChanged) {
		t.Fatalf("changed read-only guard accepted: %v", err)
	}
	generation, _, err := GetSecure("cashflux:dataset:gen")
	if err != nil || generation != "new" {
		t.Fatal("failed read-only CAS published its update")
	}
}
