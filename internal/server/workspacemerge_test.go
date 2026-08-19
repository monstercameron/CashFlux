// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/datasetmerge"
)

func txnIDs(t *testing.T, raw []byte) map[string]bool {
	t.Helper()
	var doc struct {
		Transactions []struct {
			ID string `json:"id"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	out := map[string]bool{}
	for _, tx := range doc.Transactions {
		out[tx.ID] = true
	}
	return out
}

func TestMergeKeepsBothSidesRecords(t *testing.T) {
	// The situation this exists for: the duplicate account was created partway
	// through, so BOTH copies hold real work. Replace would throw one half away.
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "New browser",
		[]byte(`{"transactions":[{"id":"t2","amount":20},{"id":"t3","amount":30}]}`))
	seedWorkspace(t, store, "dst", "ws-dst", "Original",
		[]byte(`{"transactions":[{"id":"t1","amount":10},{"id":"t2","amount":20}]}`))
	now := time.Now().UTC()

	res, report, err := store.MergeWorkspaceSnapshot("src", "ws-src", "dst", "ws-dst", datasetmerge.PreferTarget, now, false)
	if err != nil {
		t.Fatalf("MergeWorkspaceSnapshot: %v", err)
	}
	snap, ok, err := store.GetSnapshotForUser("dst", "ws-dst")
	if err != nil || !ok {
		t.Fatalf("merged snapshot: ok=%v err=%v", ok, err)
	}
	ids := txnIDs(t, snap.Dataset)
	for _, want := range []string{"t1", "t2", "t3"} {
		if !ids[want] {
			t.Errorf("transaction %s is missing from the merge: %s", want, snap.Dataset)
		}
	}
	if len(ids) != 3 {
		t.Errorf("merged to %d transactions, want 3 — a merge must not duplicate", len(ids))
	}
	if report.TotalAdded != 1 {
		t.Errorf("report added %d, want 1", report.TotalAdded)
	}
	// The version has to move with the contents, or the next client pull decides
	// it is already up to date and never sees the merge performed for it.
	if snap.Version <= 1 {
		t.Errorf("version = %d, want it advanced", snap.Version)
	}
	// And the previous contents are recoverable, same guarantee as a replace.
	if _, err := store.RollbackWorkspaceSnapshot("dst", "ws-dst", res.ArchivedVersion, now); err != nil {
		t.Fatalf("rollback after merge: %v", err)
	}
	back, _, _ := store.GetSnapshotForUser("dst", "ws-dst")
	if ids := txnIDs(t, back.Dataset); ids["t3"] || !ids["t1"] {
		t.Errorf("rollback did not restore the pre-merge snapshot: %s", back.Dataset)
	}
}

func TestMergeCarriesAttachmentLinks(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "Source", []byte(`{"transactions":[{"id":"t1"}]}`))
	seedWorkspace(t, store, "dst", "ws-dst", "Target", []byte(`{"transactions":[{"id":"t2"}]}`))
	now := time.Now().UTC()
	blob, err := store.PutBlob(t.TempDir(), []byte("receipt"), "image/png", "receipt.png", 1024)
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if err := store.LinkWorkspaceBlob("src", "ws-src", blob.Hash); err != nil {
		t.Fatalf("LinkWorkspaceBlob: %v", err)
	}
	if _, _, err := store.MergeWorkspaceSnapshot("src", "ws-src", "dst", "ws-dst", datasetmerge.PreferTarget, now, false); err != nil {
		t.Fatalf("MergeWorkspaceSnapshot: %v", err)
	}
	if ok, err := store.UserWorkspaceBlob("dst", "ws-dst", blob.Hash); err != nil || !ok {
		t.Fatalf("merged data references an attachment the target cannot read: ok=%v err=%v", ok, err)
	}
}

func TestPreviewMergeComputesRatherThanEstimates(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "Source",
		[]byte(`{"transactions":[{"id":"t1","amount":11},{"id":"t9","amount":90}]}`))
	seedWorkspace(t, store, "dst", "ws-dst", "Target",
		[]byte(`{"transactions":[{"id":"t1","amount":10}]}`))

	p, err := store.PreviewMerge("src", "ws-src", "dst", "ws-dst", datasetmerge.PreferSource)
	if err != nil {
		t.Fatalf("PreviewMerge: %v", err)
	}
	if p.Blocked {
		t.Fatalf("a legitimate merge was blocked: %+v", p)
	}
	if p.Merge == nil {
		t.Fatal("a merge preview must report what the merge would do")
	}
	if p.Merge.TotalAdded != 1 || p.Merge.Conflicts != 1 {
		t.Errorf("preview = %+v, want 1 added and 1 conflict", p.Merge)
	}
	if len(p.Warnings) == 0 {
		t.Error("a merge with conflicts must warn before it is confirmed")
	}
	// The preview must not have changed anything.
	snap, _, _ := store.GetSnapshotForUser("dst", "ws-dst")
	if ids := txnIDs(t, snap.Dataset); ids["t9"] {
		t.Errorf("the preview mutated the target: %s", snap.Dataset)
	}
}

func TestMergeRefusesAnEmptySource(t *testing.T) {
	store := openTestStore(t)
	migrateTestUsers(t, store, "src", "dst")
	seedWorkspace(t, store, "src", "ws-src", "Empty", nil)
	seedWorkspace(t, store, "dst", "ws-dst", "Real", []byte(`{"transactions":[{"id":"t1"}]}`))
	if _, _, err := store.MergeWorkspaceSnapshot("src", "ws-src", "dst", "ws-dst", datasetmerge.PreferTarget, time.Now().UTC(), false); err == nil {
		t.Fatal("merging from a workspace with no snapshot must be refused")
	}
}

func TestMergePolicyIsParsedStrictly(t *testing.T) {
	// A mistyped policy silently becoming "keep the target" would quietly
	// discard the incoming records the operator was trying to bring in.
	for _, raw := range []string{"source", "prefer_source", "newest"} {
		if _, ok := parseMergePolicy(raw); ok {
			t.Errorf("parseMergePolicy(%q) accepted an unknown policy", raw)
		}
	}
	if p, ok := parseMergePolicy(""); !ok || p != datasetmerge.PreferTarget {
		t.Errorf("empty policy = (%q, %v), want the conservative default", p, ok)
	}
	if p, ok := parseMergePolicy("prefer-source"); !ok || p != datasetmerge.PreferSource {
		t.Errorf("prefer-source = (%q, %v)", p, ok)
	}
}

func TestMergeModeIsRecognised(t *testing.T) {
	if m, ok := parseMigrationMode("merge"); !ok || m != MigrateMerge {
		t.Fatalf("parseMigrationMode(merge) = (%q, %v)", m, ok)
	}
}

func TestAdminSearchFindsAnAccountByWorkspaceOrDevice(t *testing.T) {
	// The identifiers an operator actually has are a workspace id out of a
	// client error report, or a device id from a pairing request. Neither was
	// searchable, so "which account is this?" meant paging through every user.
	store := openTestStore(t)
	migrateTestUsers(t, store, "acct-one", "acct-two")
	seedWorkspace(t, store, "acct-one", "ws-needle", "Household", []byte(`{}`))
	now := time.Now().UTC()
	deviceID, _, err := store.MintPendingDevice("Cam's laptop", now)
	if err != nil {
		t.Fatalf("MintPendingDevice: %v", err)
	}
	if err := store.SetPendingDeviceAccount(deviceID, "acct-two", ResolvedByAdmin, now); err != nil {
		t.Fatalf("SetPendingDeviceAccount: %v", err)
	}

	for _, tc := range []struct{ query, want string }{
		{"ws-needle", "acct-one"},
		{"Household", "acct-one"},
		{deviceID, "acct-two"},
		{"laptop", "acct-two"},
		{"acct-one", "acct-one"},
	} {
		rows, err := store.ListUsersFiltered(50, 0, tc.query)
		if err != nil {
			t.Fatalf("ListUsersFiltered(%q): %v", tc.query, err)
		}
		found := false
		for _, r := range rows {
			if r.ID == tc.want {
				found = true
			}
		}
		if !found {
			t.Errorf("searching %q did not find %s (got %d rows)", tc.query, tc.want, len(rows))
		}
	}
	// A search that matches nothing still returns nothing, rather than everything.
	if rows, err := store.ListUsersFiltered(50, 0, "no-such-thing"); err != nil || len(rows) != 0 {
		t.Errorf("unmatched search returned %d rows (err %v), want 0", len(rows), err)
	}
}
