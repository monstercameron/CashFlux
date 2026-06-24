// SPDX-License-Identifier: MIT

package history

import (
	"encoding/json"
	"testing"
)

func snap(m map[string]map[string]string) Snapshot {
	out := make(Snapshot, len(m))
	for coll, rows := range m {
		r := make(map[string]json.RawMessage, len(rows))
		for id, v := range rows {
			r[id] = json.RawMessage(v)
		}
		out[coll] = r
	}
	return out
}

// snapEq compares two snapshots, treating an absent collection and an empty one as
// equal, and comparing rows by their JSON bytes.
func snapEq(a, b Snapshot) bool {
	norm := func(s Snapshot) map[string]map[string]string {
		out := map[string]map[string]string{}
		for coll, rows := range s {
			if len(rows) == 0 {
				continue
			}
			m := map[string]string{}
			for id, raw := range rows {
				m[id] = string(raw)
			}
			out[coll] = m
		}
		return out
	}
	ab, _ := json.Marshal(norm(a))
	bb, _ := json.Marshal(norm(b))
	return string(ab) == string(bb)
}

func TestDiffOps(t *testing.T) {
	before := snap(map[string]map[string]string{
		"txn": {"t1": `{"amt":10}`, "t2": `{"amt":20}`},
	})
	after := snap(map[string]map[string]string{
		"txn":  {"t1": `{"amt":10}`, "t2": `{"amt":99}`, "t3": `{"amt":5}`}, // t2 updated, t3 added, none deleted yet
		"acct": {"a1": `{"name":"x"}`},                                      // new collection (add)
	})
	cs := Diff(before, after)

	got := map[string]Op{}
	for _, c := range cs.Changes {
		got[c.Collection+"/"+c.ID] = c.Op
	}
	want := map[string]Op{"txn/t2": OpUpdate, "txn/t3": OpAdd, "acct/a1": OpAdd}
	if len(got) != len(want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
	for k, op := range want {
		if got[k] != op {
			t.Errorf("%s op = %q, want %q", k, got[k], op)
		}
	}
	// t1 unchanged → no change recorded.
	if _, ok := got["txn/t1"]; ok {
		t.Error("unchanged row t1 should produce no change")
	}
}

func TestDiffDeterministicOrder(t *testing.T) {
	before := snap(map[string]map[string]string{"z": {"b": `1`, "a": `1`}, "a": {"q": `1`}})
	after := Snapshot{} // delete everything
	cs := Diff(before, after)
	wantOrder := []string{"a/q", "z/a", "z/b"}
	if len(cs.Changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(cs.Changes))
	}
	for i, key := range wantOrder {
		if got := cs.Changes[i].Collection + "/" + cs.Changes[i].ID; got != key {
			t.Errorf("change[%d] = %q, want %q (sorted)", i, got, key)
		}
	}
}

func TestRoundTripForwardAndInverse(t *testing.T) {
	before := snap(map[string]map[string]string{
		"txn": {"keep": `{"v":1}`, "edit": `{"v":2}`, "gone": `{"v":3}`},
	})
	after := snap(map[string]map[string]string{
		"txn": {"keep": `{"v":1}`, "edit": `{"v":22}`, "new": `{"v":4}`}, // edit changed, gone deleted, new added
	})
	cs := Diff(before, after)

	if fwd := cs.Apply(before); !snapEq(fwd, after) {
		t.Errorf("forward apply did not reach after:\n got %v\n want %v", fwd, after)
	}
	if back := cs.Invert().Apply(after); !snapEq(back, before) {
		t.Errorf("inverse apply did not restore before:\n got %v\n want %v", back, before)
	}
}

func TestApplyDoesNotMutateInput(t *testing.T) {
	before := snap(map[string]map[string]string{"txn": {"t1": `{"v":1}`}})
	cs := ChangeSet{Changes: []Change{{Collection: "txn", ID: "t1", Op: OpUpdate, Before: json.RawMessage(`{"v":1}`), After: json.RawMessage(`{"v":2}`)}}}
	_ = cs.Apply(before)
	if string(before["txn"]["t1"]) != `{"v":1}` {
		t.Errorf("Apply mutated the input snapshot: %s", before["txn"]["t1"])
	}
}

func TestDiffNoOp(t *testing.T) {
	s := snap(map[string]map[string]string{"txn": {"t1": `{"v":1}`}})
	if cs := Diff(s, s.Clone()); !cs.IsEmpty() {
		t.Errorf("diff of identical snapshots should be empty, got %+v", cs)
	}
}
