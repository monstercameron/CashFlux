// SPDX-License-Identifier: MIT

package store

import "testing"

func TestUnreadableDatasetReason(t *testing.T) {
	validJSON, err := Export(Dataset{})
	if err != nil {
		t.Fatal(err)
	}
	validEnvelope := append([]byte("\x00cf1\x00"), []byte(`{"v":1,"alg":"AES-GCM-PBKDF2","salt":"s","iv":"i","cipher":"c"}`)...)
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "absent", raw: nil, want: ""},
		{name: "empty bytes", raw: []byte{}, want: ""},
		{name: "valid dataset", raw: validJSON, want: ""},
		{name: "valid encryption envelope", raw: validEnvelope, want: ""},
		{name: "whitespace only", raw: []byte(" \n\t "), want: "empty dataset"},
		{name: "corrupt JSON", raw: []byte(`{"accounts":`), want: "invalid dataset JSON"},
		{name: "truncated encryption envelope", raw: []byte("\x00cf1\x00"), want: "invalid encryption envelope"},
		{name: "malformed encryption envelope", raw: []byte("\x00cf1\x00not-json"), want: "invalid encryption envelope"},
		{name: "newer schema", raw: []byte(`{"schemaVersion":9999}`), want: "unsupported schema version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := UnreadableDatasetReason(tc.raw); got != tc.want {
				t.Errorf("UnreadableDatasetReason(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestFindQuarantineRecordDeduplicatesIdenticalBytes(t *testing.T) {
	raw := []byte("unreadable dataset")
	records := []QuarantineRecord{{Key: "cashflux:quarantine:123", Raw: append([]byte(nil), raw...)}}
	key, found := FindQuarantineRecord(raw, records)
	if !found || key != records[0].Key {
		t.Fatalf("FindQuarantineRecord = (%q, %v), want existing record", key, found)
	}
	if _, found := FindQuarantineRecord([]byte("different bytes"), records); found {
		t.Fatal("different bytes must not reuse a quarantine record")
	}
}

func TestNewestQuarantineRecordUsesTimestampAndSequence(t *testing.T) {
	records := []QuarantineRecord{
		{Key: "older", CreatedAt: 10, Sequence: 2},
		{Key: "newest", CreatedAt: 11, Sequence: 1},
		{Key: "same-time-later", CreatedAt: 10, Sequence: 3},
	}
	got, ok := NewestQuarantineRecord(records)
	if !ok || got.Key != "newest" {
		t.Fatalf("NewestQuarantineRecord = (%+v, %v), want newest", got, ok)
	}
	got, ok = NewestQuarantineRecord(records[0:1])
	if !ok || got.Key != "older" {
		t.Fatalf("NewestQuarantineRecord single-record case = (%+v, %v), want older", got, ok)
	}
}
