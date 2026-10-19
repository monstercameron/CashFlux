// SPDX-License-Identifier: MIT

package store

import (
	"crypto/sha256"
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/cryptobox"
)

// UnreadableDatasetReason reports why raw dataset bytes cannot safely be
// loaded. It returns an empty string for absent data and valid plaintext JSON
// or encrypted envelopes. The returned reason strings are short and stable so
// callers can use them in recovery messaging and tests.
func UnreadableDatasetReason(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return "empty dataset"
	}
	if cryptobox.IsEnvelope(raw) {
		if _, ok := cryptobox.Parse(raw); !ok {
			return "invalid encryption envelope"
		}
		return ""
	}

	if _, err := Import(raw); err == nil {
		return ""
	}

	var header struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(raw, &header); err == nil && header.SchemaVersion > SchemaVersion {
		return "unsupported schema version"
	}
	return "invalid dataset JSON"
}

// QuarantineRecord describes a saved unreadable dataset copy.
type QuarantineRecord struct {
	Key       string
	Raw       []byte
	CreatedAt int64
	Sequence  int64
}

// FindQuarantineRecord returns the key of an existing record with identical
// bytes. Comparing hashes keeps the decision independent from browser storage.
func FindQuarantineRecord(raw []byte, records []QuarantineRecord) (string, bool) {
	want := sha256.Sum256(raw)
	for _, record := range records {
		if sha256.Sum256(record.Raw) == want {
			return record.Key, true
		}
	}
	return "", false
}

// NewestQuarantineRecord returns the most recently created record.
func NewestQuarantineRecord(records []QuarantineRecord) (QuarantineRecord, bool) {
	if len(records) == 0 {
		return QuarantineRecord{}, false
	}
	newest := records[0]
	for _, record := range records[1:] {
		if record.CreatedAt > newest.CreatedAt ||
			(record.CreatedAt == newest.CreatedAt && record.Sequence > newest.Sequence) {
			newest = record
		}
	}
	return newest, true
}
