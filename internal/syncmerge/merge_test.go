// SPDX-License-Identifier: MIT

package syncmerge_test

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/syncmerge"
)

var (
	t0 = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = t0.Add(time.Minute)
	t2 = t0.Add(2 * time.Minute)
)

func fv(v string, t time.Time) syncmerge.FieldValue {
	return syncmerge.FieldValue{Value: v, UpdatedAt: t}
}

// ─── MergeRecord tests ───────────────────────────────────────────────────────

func TestMergeRecord_NoConflict_LocalNewer(t *testing.T) {
	local := syncmerge.Record{"name": fv("Alice", t2)}
	remote := syncmerge.Record{"name": fv("Bob", t1)}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["name"].Value; got != "Alice" {
		t.Errorf("merged name = %q, want Alice", got)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].ChosenSide != "local" {
		t.Errorf("chosen side = %q, want local", conflicts[0].ChosenSide)
	}
}

func TestMergeRecord_NoConflict_RemoteNewer(t *testing.T) {
	local := syncmerge.Record{"name": fv("Alice", t1)}
	remote := syncmerge.Record{"name": fv("Bob", t2)}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["name"].Value; got != "Bob" {
		t.Errorf("merged name = %q, want Bob", got)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].ChosenSide != "remote" {
		t.Errorf("chosen side = %q, want remote", conflicts[0].ChosenSide)
	}
}

func TestMergeRecord_SameValue_NoConflict(t *testing.T) {
	local := syncmerge.Record{"color": fv("green", t1)}
	remote := syncmerge.Record{"color": fv("green", t2)}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["color"].Value; got != "green" {
		t.Errorf("merged color = %q, want green", got)
	}
	// Same value → no conflict entry, but later timestamp wins.
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
	if !merged["color"].UpdatedAt.Equal(t2) {
		t.Errorf("merged timestamp = %v, want %v", merged["color"].UpdatedAt, t2)
	}
}

func TestMergeRecord_TieBreak_RemoteWins(t *testing.T) {
	// Equal timestamps, different values → remote wins.
	local := syncmerge.Record{"currency": fv("USD", t1)}
	remote := syncmerge.Record{"currency": fv("EUR", t1)}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["currency"].Value; got != "EUR" {
		t.Errorf("merged currency = %q, want EUR", got)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].ChosenSide != "remote" {
		t.Errorf("tie-break side = %q, want remote", conflicts[0].ChosenSide)
	}
}

func TestMergeRecord_OnlyLocalHasField(t *testing.T) {
	local := syncmerge.Record{"note": fv("hello", t1)}
	remote := syncmerge.Record{}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["note"].Value; got != "hello" {
		t.Errorf("merged note = %q, want hello", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestMergeRecord_OnlyRemoteHasField(t *testing.T) {
	local := syncmerge.Record{}
	remote := syncmerge.Record{"note": fv("world", t2)}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["note"].Value; got != "world" {
		t.Errorf("merged note = %q, want world", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestMergeRecord_MultipleFields_PartialConflict(t *testing.T) {
	local := syncmerge.Record{
		"name":     fv("Household A", t2), // local newer → wins
		"currency": fv("USD", t1),          // remote newer → loses
		"color":    fv("blue", t1),         // same value → no conflict
	}
	remote := syncmerge.Record{
		"name":     fv("Household B", t1),
		"currency": fv("EUR", t2),
		"color":    fv("blue", t2),
	}
	merged, conflicts := syncmerge.MergeRecord(local, remote)
	if got := merged["name"].Value; got != "Household A" {
		t.Errorf("name = %q, want Household A", got)
	}
	if got := merged["currency"].Value; got != "EUR" {
		t.Errorf("currency = %q, want EUR", got)
	}
	if got := merged["color"].Value; got != "blue" {
		t.Errorf("color = %q, want blue", got)
	}
	// name and currency conflict; color does not.
	if len(conflicts) != 2 {
		t.Errorf("conflicts = %d, want 2", len(conflicts))
	}
}

func TestMergeRecord_ConflictEntryPreservesBothValues(t *testing.T) {
	local := syncmerge.Record{"name": fv("A", t1)}
	remote := syncmerge.Record{"name": fv("B", t2)}
	_, conflicts := syncmerge.MergeRecord(local, remote)
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %d, want 1", len(conflicts))
	}
	c := conflicts[0]
	if c.LocalValue != "A" {
		t.Errorf("local value = %q, want A", c.LocalValue)
	}
	if c.RemoteValue != "B" {
		t.Errorf("remote value = %q, want B", c.RemoteValue)
	}
	if c.ChosenValue != "B" {
		t.Errorf("chosen value = %q, want B", c.ChosenValue)
	}
}

// ─── ThreeWayMerge tests ─────────────────────────────────────────────────────

func TestThreeWayMerge_OnlyLocalChanged(t *testing.T) {
	base := syncmerge.Record{"name": fv("Base", t0)}
	local := syncmerge.Record{"name": fv("Local", t1)}
	remote := syncmerge.Record{"name": fv("Base", t0)} // unchanged
	merged, conflicts := syncmerge.ThreeWayMerge(base, local, remote)
	if got := merged["name"].Value; got != "Local" {
		t.Errorf("name = %q, want Local", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestThreeWayMerge_OnlyRemoteChanged(t *testing.T) {
	base := syncmerge.Record{"name": fv("Base", t0)}
	local := syncmerge.Record{"name": fv("Base", t0)} // unchanged
	remote := syncmerge.Record{"name": fv("Remote", t2)}
	merged, conflicts := syncmerge.ThreeWayMerge(base, local, remote)
	if got := merged["name"].Value; got != "Remote" {
		t.Errorf("name = %q, want Remote", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestThreeWayMerge_BothChangedSameValue_NoConflict(t *testing.T) {
	base := syncmerge.Record{"color": fv("red", t0)}
	local := syncmerge.Record{"color": fv("blue", t1)}
	remote := syncmerge.Record{"color": fv("blue", t2)}
	merged, conflicts := syncmerge.ThreeWayMerge(base, local, remote)
	if got := merged["color"].Value; got != "blue" {
		t.Errorf("color = %q, want blue", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestThreeWayMerge_BothChangedDifferentValues_Conflict_LWW(t *testing.T) {
	base := syncmerge.Record{"currency": fv("USD", t0)}
	local := syncmerge.Record{"currency": fv("GBP", t2)} // newer
	remote := syncmerge.Record{"currency": fv("EUR", t1)}
	merged, conflicts := syncmerge.ThreeWayMerge(base, local, remote)
	if got := merged["currency"].Value; got != "GBP" {
		t.Errorf("currency = %q, want GBP (local newer)", got)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1", len(conflicts))
	}
	if conflicts[0].ChosenSide != "local" {
		t.Errorf("chosen side = %q, want local", conflicts[0].ChosenSide)
	}
}

func TestThreeWayMerge_NeitherChanged(t *testing.T) {
	base := syncmerge.Record{"name": fv("Steady", t0)}
	local := syncmerge.Record{"name": fv("Steady", t0)}
	remote := syncmerge.Record{"name": fv("Steady", t0)}
	merged, conflicts := syncmerge.ThreeWayMerge(base, local, remote)
	if got := merged["name"].Value; got != "Steady" {
		t.Errorf("name = %q, want Steady", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0", len(conflicts))
	}
}

func TestThreeWayMerge_NilBase_DegeneratesToLWW(t *testing.T) {
	local := syncmerge.Record{"x": fv("L", t2)}
	remote := syncmerge.Record{"x": fv("R", t1)}
	merged, conflicts := syncmerge.ThreeWayMerge(nil, local, remote)
	if got := merged["x"].Value; got != "L" {
		t.Errorf("x = %q, want L", got)
	}
	if len(conflicts) != 1 {
		t.Errorf("conflicts = %d, want 1", len(conflicts))
	}
}

func TestThreeWayMerge_RoundTrip(t *testing.T) {
	// Scenario: base has 3 fields; local changes field A; remote changes field B;
	// neither touches C → merged should have localA, remoteB, baseC, no conflicts.
	base := syncmerge.Record{
		"a": fv("base-a", t0),
		"b": fv("base-b", t0),
		"c": fv("base-c", t0),
	}
	local := syncmerge.Record{
		"a": fv("local-a", t1),
		"b": fv("base-b", t0),
		"c": fv("base-c", t0),
	}
	remote := syncmerge.Record{
		"a": fv("base-a", t0),
		"b": fv("remote-b", t2),
		"c": fv("base-c", t0),
	}
	merged, conflicts := syncmerge.ThreeWayMerge(base, local, remote)
	if got := merged["a"].Value; got != "local-a" {
		t.Errorf("a = %q, want local-a", got)
	}
	if got := merged["b"].Value; got != "remote-b" {
		t.Errorf("b = %q, want remote-b", got)
	}
	if got := merged["c"].Value; got != "base-c" {
		t.Errorf("c = %q, want base-c", got)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %d, want 0 — clean 3-way should be conflict-free", len(conflicts))
	}
}
