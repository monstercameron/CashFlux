// SPDX-License-Identifier: MIT

package changes_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/changes"
)

func TestBatchCanonicalRoundTripAndValidation(t *testing.T) {
	b := batchAt(1, "device-a", 100, 2,
		writeOp("transactions", "t-1", map[string]string{"z": ` { "v": 2 } `, "a": `"x"`}),
	)
	first, err := changes.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	second, err := changes.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("Marshal not deterministic:\n%s\n%s", first, second)
	}
	decoded, err := changes.Unmarshal(first)
	if err != nil {
		t.Fatal(err)
	}
	third, err := changes.Marshal(decoded)
	if err != nil || !bytes.Equal(first, third) {
		t.Fatalf("round trip = %s, err %v; want %s", third, err, first)
	}

	bad := b.Clone()
	bad.ChangeID = "not-a-uuid"
	if _, err := changes.Marshal(bad); !errors.Is(err, changes.ErrInvalidBatch) {
		t.Fatalf("invalid ID error = %v, want ErrInvalidBatch", err)
	}
}

func TestUnmarshalBatchRejectsDeepJSONBeforeCanonicalization(t *testing.T) {
	data := deeplyNestedBatch(1500)

	started := time.Now()
	if _, err := changes.UnmarshalBatch(data); !errors.Is(err, changes.ErrInvalidBatch) {
		t.Fatalf("UnmarshalBatch error = %v, want ErrInvalidBatch", err)
	}
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("deep JSON rejection took %s, want under 100ms", elapsed)
	}
}

func deeplyNestedBatch(depth int) []byte {
	return []byte(`{"changeId":"550e8400-e29b-41d4-a716-446655440000","scope":{"serverId":"s","accountId":"a"},"workspaceId":"w","deviceId":"d","hlc":{"wallMillis":1,"counter":0},"schemaVersion":1,"ops":[{"table":"records","recordId":"r","fields":{"value":` +
		strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth) + `}}]}`)
}

func TestNewerSchemaRejectedAtomically(t *testing.T) {
	s := changes.NewState()
	b := batchAt(2, "device-a", 100, 0, writeOp("accounts", "a-1", map[string]string{"name": `"must not apply"`}))
	b.SchemaVersion++
	if err := s.Apply(b); !errors.Is(err, changes.ErrNewerSchema) {
		t.Fatalf("Apply error = %v, want ErrNewerSchema", err)
	}
	mustNotExist(t, s, "accounts", "a-1")
	if s.Seen(b.ChangeID) {
		t.Fatal("rejected batch was recorded as seen")
	}
}

func TestDiffDocuments(t *testing.T) {
	tests := []struct {
		name          string
		before, after []byte
		want          string
		changed       bool
	}{
		{"create", nil, []byte(`{"name":"A","nested":{"x":1}}`), `{"name":"A","nested":{"x":1}}`, true},
		{"nested replacement", []byte(`{"nested":{"x":1,"y":2}}`), []byte(`{"nested":{"x":1,"y":3}}`), `{"nested":{"x":1,"y":3}}`, true},
		{"remove field", []byte(`{"a":1,"b":2}`), []byte(`{"a":1}`), "removed:b", true},
		{"same despite key order", []byte(`{"a":1,"b":{"x":2,"y":3}}`), []byte(` {"b":{"y":3,"x":2},"a":1} `), "", false},
		{"delete", []byte(`{"a":1}`), nil, "deleted", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, ok := changes.DiffDocuments("records", "r-1", tc.before, tc.after)
			if ok != tc.changed {
				t.Fatalf("changed = %v, want %v", ok, tc.changed)
			}
			if !ok {
				return
			}
			switch tc.want {
			case "deleted":
				if !op.Deleted {
					t.Fatalf("op = %#v, want delete", op)
				}
			case "removed:b":
				if len(op.Removed) != 1 || op.Removed[0] != "b" {
					t.Fatalf("removed = %v", op.Removed)
				}
			default:
				got, err := json.Marshal(op.Fields)
				if err != nil || string(got) != tc.want {
					t.Fatalf("fields = %s, err %v; want %s", got, err, tc.want)
				}
			}
		})
	}
}

func TestDeleteEditAndFieldRemovalConverge(t *testing.T) {
	cases := []struct {
		name    string
		batches []changes.Batch
		want    string
		exists  bool
	}{
		{"delete beats older edit", []changes.Batch{
			batchAt(10, "device-a", 2, 0, writeOp("accounts", "a", map[string]string{"name": `"old"`})),
			batchAt(11, "device-b", 3, 0, deleteOp("accounts", "a")),
		}, "", false},
		{"new edit revives with only new fields", []changes.Batch{
			batchAt(12, "device-a", 2, 0, writeOp("accounts", "a", map[string]string{"name": `"old"`, "balance": `10`})),
			batchAt(13, "device-b", 3, 0, deleteOp("accounts", "a")),
			batchAt(14, "device-a", 4, 0, writeOp("accounts", "a", map[string]string{"name": `"new"`})),
		}, `{"name":"new"}`, true},
		{"same-millisecond device tie uses device id", []changes.Batch{
			batchAt(15, "device-a", 5, 0, writeOp("accounts", "a", map[string]string{"name": `"edit"`})),
			batchAt(16, "device-b", 5, 0, deleteOp("accounts", "a")),
		}, "", false},
		{"field removal beats old write", []changes.Batch{
			batchAt(17, "device-a", 2, 0, writeOp("accounts", "a", map[string]string{"name": `"old"`})),
			batchAt(18, "device-b", 3, 0, removeOp("accounts", "a", "name")),
			batchAt(19, "device-a", 2, 1, writeOp("accounts", "a", map[string]string{"name": `"late"`})),
		}, `{}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := changes.NewState()
			mustApply(t, s, tc.batches...)
			got, ok := s.Materialize("accounts", "a")
			if ok != tc.exists || string(got) != tc.want {
				t.Fatalf("Materialize = %s, %v; want %s, %v", got, ok, tc.want, tc.exists)
			}
		})
	}
}

func TestApplyIdempotentAndFieldClockSnapshotRoundTrip(t *testing.T) {
	batches := []changes.Batch{
		batchAt(20, "device-a", 10, 0, writeOp("records", "r", map[string]string{"a": `1`, "b": `{"x":true}`})),
		batchAt(21, "device-b", 11, 0, removeOp("records", "r", "a")),
	}
	one, twice := changes.NewState(), changes.NewState()
	mustApply(t, one, batches...)
	mustApply(t, twice, batches...)
	mustApply(t, twice, batches...)
	if a, _ := one.Materialize("records", "r"); string(a) != `{ "b":{"x":true}}` && string(a) != `{"b":{"x":true}}` {
		t.Fatalf("unexpected one-pass document %s", a)
	}
	a, _ := one.Materialize("records", "r")
	b, _ := twice.Materialize("records", "r")
	if !bytes.Equal(a, b) {
		t.Fatalf("idempotent replay differs: %s vs %s", a, b)
	}
	clocks := one.ExportFieldClocks()
	restored := changes.NewState()
	if err := restored.ImportFieldClocks(clocks); err != nil {
		t.Fatal(err)
	}
	r, ok := restored.Materialize("records", "r")
	if !ok || !bytes.Equal(a, r) {
		t.Fatalf("snapshot materialize = %s, %v; want %s", r, ok, a)
	}
}

func TestStateScopeGate(t *testing.T) {
	s := changes.NewScopedState(changes.Scope{ServerID: "s1", AccountID: "a1"}, "w1")
	b := batchAt(22, "device-a", 1, 0, writeOp("records", "r", map[string]string{"v": `1`}))
	b.Scope.AccountID = "a2"
	if err := s.Apply(b); !errors.Is(err, changes.ErrScopeMismatch) {
		t.Fatalf("Apply error = %v, want ErrScopeMismatch", err)
	}
	mustNotExist(t, s, "records", "r")
}

func TestRandomBatchPermutationsConverge(t *testing.T) {
	const permutationCount = 10_000
	rng := rand.New(rand.NewSource(0x5eedcafe))
	var batches []changes.Batch
	seq := 1000
	for device := 0; device < 3; device++ {
		for n := 0; n < 8; n++ {
			seq++
			table, id := "records", fmt.Sprintf("r-%d", rng.Intn(4))
			wall := int64(100 + rng.Intn(12))
			var op changes.Op
			switch rng.Intn(5) {
			case 0:
				op = deleteOp(table, id)
			case 1:
				op = removeOp(table, id, fmt.Sprintf("f%d", rng.Intn(4)))
			default:
				op = writeOp(table, id, map[string]string{fmt.Sprintf("f%d", rng.Intn(4)): fmt.Sprintf(`{"device":%d,"n":%d}`, device, n)})
			}
			batches = append(batches, batchAt(seq, fmt.Sprintf("device-%d", device), wall, uint32(rng.Intn(3)), op))
		}
	}
	baseline := changes.NewState()
	mustApply(t, baseline, batches...)
	want := captureState(t, baseline)
	for i := 0; i < permutationCount; i++ {
		order := append([]changes.Batch(nil), batches...)
		rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		state := changes.NewState()
		mustApply(t, state, order...)
		if got := captureState(t, state); !equalDocuments(got, want) {
			t.Fatalf("permutation %d diverged:\n got %#v\nwant %#v", i, got, want)
		}
	}
}

func captureState(t *testing.T, s *changes.State) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, id := range s.RecordIDs("records") {
		data, ok := s.Materialize("records", id)
		if !ok {
			t.Fatalf("RecordIDs returned absent record %q", id)
		}
		out[id] = string(data)
	}
	return out
}

func equalDocuments(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func FuzzUnmarshalBatch(f *testing.F) {
	valid := batchAt(50, "device-a", 123, 4, writeOp("records", "r", map[string]string{"name": `"seed"`}))
	seed, _ := changes.Marshal(valid)
	f.Add(seed)
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schemaVersion":999}`))
	f.Add(deeplyNestedBatch(1500))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Keep mutation inputs small enough that a single fuzz case cannot
		// spend the entire campaign allocating an adversarial batch. Larger
		// payload rejection is covered by Unmarshal's explicit byte limit.
		if len(data) > 4096 {
			return
		}
		done := make(chan error, 1)
		go func() {
			batch, err := changes.Unmarshal(data)
			if err != nil {
				done <- nil
				return
			}
			encoded, err := changes.Marshal(batch)
			if err != nil {
				done <- fmt.Errorf("Marshal after successful Unmarshal: %w", err)
				return
			}
			if _, err := changes.Unmarshal(encoded); err != nil {
				done <- fmt.Errorf("round trip Unmarshal: %w", err)
				return
			}
			done <- nil
		}()
		timer := time.NewTimer(2 * time.Second)
		select {
		case err := <-done:
			timer.Stop()
			if err != nil {
				t.Fatal(err)
			}
		case <-timer.C:
			t.Fatalf("UnmarshalBatch did not return within 2s: len=%d hex=%s", len(data), hex.EncodeToString(data))
		}
	})
}

func BenchmarkApply10000SingleFieldBatches(b *testing.B) {
	batches := make([]changes.Batch, 10_000)
	for i := range batches {
		batches[i] = batchAt(i+1, "device-a", int64(i+1), 0,
			writeOp("records", fmt.Sprintf("r-%d", i), map[string]string{"value": fmt.Sprintf("%d", i)}))
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		s := changes.NewState()
		for _, batch := range batches {
			if err := s.Apply(batch); err != nil {
				b.Fatal(err)
			}
		}
	}
}
