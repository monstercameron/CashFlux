package conflictcheckpoint

import (
	"reflect"
	"testing"
	"time"
)

func testScope(account string) Scope {
	return Scope{ServerID: "server-1", AccountID: account, WorkspaceID: "workspace-1"}
}

func testCheckpoint(id string, scope Scope, state State) Checkpoint {
	return Checkpoint{
		ID: id, Scope: scope,
		CreatedAt:       RevisionOrder{DatasetRevision: 4, DeviceCounter: 2},
		CreatedWallTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Origin:          OriginRejectedPush, BaseVersion: 3, DeviceLabel: "Laptop",
		RecordCounts: map[string]int{"transaction": 2}, State: state,
	}
}

func TestListAndRestoreStayWithinExactScopeAndKeepMultipleCheckpoints(t *testing.T) {
	a := testScope("account-a")
	b := testScope("account-b")
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	checkpoints := []Checkpoint{
		testCheckpoint("a-1", a, StateKept),
		testCheckpoint("a-2", a, StateKept),
		testCheckpoint("b-1", b, StateKept),
		testCheckpoint("a-old", a, StateDiscarded),
	}
	checkpoints[1].CreatedAt.DeviceCounter = 5

	got := ListForScope(checkpoints, a, now)
	if len(got) != 2 || got[0].ID != "a-1" || got[1].ID != "a-2" {
		t.Fatalf("ListForScope(account A) = %#v, want both kept account A checkpoints", got)
	}
	if testCheckpoint("b-1", b, StateKept).CanRestore(a, now) {
		t.Fatal("checkpoint from account B was restorable in account A")
	}
	for _, otherScope := range []Scope{
		{ServerID: "server-2", AccountID: a.AccountID, WorkspaceID: a.WorkspaceID},
		{ServerID: a.ServerID, AccountID: a.AccountID, WorkspaceID: "workspace-2"},
	} {
		if testCheckpoint("a-1", a, StateKept).CanRestore(otherScope, now) {
			t.Fatalf("checkpoint scope %#v was restorable in %#v", a, otherScope)
		}
	}
	if !got[0].CanRestore(a, now) || len(ListForScope(checkpoints, Scope{ServerID: a.ServerID, WorkspaceID: a.WorkspaceID}, now)) != 0 {
		t.Fatal("exact-scope restore or incomplete-scope rejection failed")
	}
}

func TestRevisionOrderIgnoresWallClock(t *testing.T) {
	cases := []struct {
		name string
		a, b RevisionOrder
		want int
	}{
		{name: "revision first", a: RevisionOrder{DatasetRevision: 2, DeviceCounter: 99}, b: RevisionOrder{DatasetRevision: 3, DeviceCounter: 1}, want: -1},
		{name: "counter tie break", a: RevisionOrder{DatasetRevision: 3, DeviceCounter: 1}, b: RevisionOrder{DatasetRevision: 3, DeviceCounter: 2}, want: -1},
		{name: "same position", a: RevisionOrder{DatasetRevision: 3, DeviceCounter: 2}, b: RevisionOrder{DatasetRevision: 3, DeviceCounter: 2}, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Compare(tc.b); got != tc.want {
				t.Fatalf("Compare() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCheckpointExpiryBoundaryAndLifecycle(t *testing.T) {
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	boundary := created.Add(Retention)
	cases := []struct {
		name  string
		state State
		now   time.Time
		want  bool
	}{
		{name: "before boundary", state: StateKept, now: boundary.Add(-time.Nanosecond), want: false},
		{name: "at boundary", state: StateKept, now: boundary, want: true},
		{name: "after boundary", state: StateKept, now: boundary.Add(time.Hour), want: true},
		{name: "already restored", state: StateRestored, now: boundary, want: false},
		{name: "already discarded", state: StateDiscarded, now: boundary, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkpoint := testCheckpoint("cp", testScope("a"), tc.state)
			checkpoint.CreatedWallTime = created
			if got := checkpoint.IsExpired(tc.now); got != tc.want {
				t.Fatalf("IsExpired() = %v, want %v", got, tc.want)
			}
			changed := checkpoint.Expire(tc.now)
			if changed != tc.want {
				t.Fatalf("Expire() changed = %v, want %v", changed, tc.want)
			}
			if tc.want && checkpoint.State != StateExpired {
				t.Fatalf("state = %q, want expired", checkpoint.State)
			}
			if tc.want && checkpoint.CanRestore(checkpoint.Scope, tc.now) {
				t.Fatal("expired checkpoint remained restorable")
			}
		})
	}
}

func TestCompareAddedChangedDeletedAndEntityCounts(t *testing.T) {
	local := Snapshot{Available: true, Records: []Record{
		{Entity: "transaction", ID: "edit", Label: "Corner Shop · Jan 4 · $12", Values: map[string]string{"amount": "1200", "date": "2026-01-04"}},
		{Entity: "transaction", ID: "delete", Label: "Old Cafe · Jan 2 · $5", Values: map[string]string{"amount": "500"}},
		{Entity: "account", ID: "acct", Label: "Checking", Values: map[string]string{"name": "Checking"}},
	}}
	server := Snapshot{Available: true, Records: []Record{
		{Entity: "transaction", ID: "edit", Label: "Corner Shop · Jan 4 · $15", Values: map[string]string{"amount": "1500", "date": "2026-01-04"}},
		{Entity: "transaction", ID: "add", Label: "Market · Jan 5 · $8", Values: map[string]string{"amount": "800"}},
		{Entity: "account", ID: "acct", Label: "Checking", Values: map[string]string{"name": "Checking"}},
	}}

	got, err := Compare(local, server)
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := map[Side]map[string]int{
		SideLocal:  {"account": 1, "transaction": 2},
		SideServer: {"account": 1, "transaction": 2},
	}
	wantChanges := []RecordChange{
		{Kind: ChangeAdded, Entity: "transaction", ID: "add", AfterLabel: "Market · Jan 5 · $8"},
		{Kind: ChangeDeleted, Entity: "transaction", ID: "delete", BeforeLabel: "Old Cafe · Jan 2 · $5"},
		{Kind: ChangeChanged, Entity: "transaction", ID: "edit", BeforeLabel: "Corner Shop · Jan 4 · $12", AfterLabel: "Corner Shop · Jan 4 · $15"},
	}
	if !got.Available || !reflect.DeepEqual(got.Counts, wantCounts) || !reflect.DeepEqual(got.Changes, wantChanges) {
		t.Fatalf("Compare() = %#v, want available counts %#v and changes %#v", got, wantCounts, wantChanges)
	}
}

func TestCompareReportsUnavailableSideWithoutGuessing(t *testing.T) {
	local := Snapshot{Available: true, Records: []Record{{Entity: "transaction", ID: "one"}}}
	got, err := Compare(local, Snapshot{Available: false, Records: []Record{{Entity: "transaction", ID: "hidden"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Available || !reflect.DeepEqual(got.UnavailableSides, []Side{SideServer}) || got.Changes != nil {
		t.Fatalf("Compare() = %#v, want unavailable server and no guessed changes", got)
	}
	if got.Counts[SideLocal]["transaction"] != 1 || len(got.Counts[SideServer]) != 0 {
		t.Fatalf("unavailable snapshot contributed counts: %#v", got.Counts)
	}
}

func TestCompareRejectsDuplicateOrIncompleteIDs(t *testing.T) {
	cases := []struct {
		name    string
		records []Record
	}{
		{name: "duplicate", records: []Record{{Entity: "transaction", ID: "x"}, {Entity: "transaction", ID: "x"}}},
		{name: "missing id", records: []Record{{Entity: "transaction"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compare(Snapshot{Available: true, Records: tc.records}, Snapshot{Available: true}); err == nil {
				t.Fatal("Compare() error = nil, want invalid snapshot error")
			}
		})
	}
}
