// SPDX-License-Identifier: MIT

package changes

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Validate checks the batch against this build's supported schema range.
func (b Batch) Validate() error { return b.validate(MaxSupportedSchemaVersion) }

// validate checks the batch against a caller-chosen supported schema
// version. A newer schema yields ErrNewerSchema (not ErrInvalidBatch), so
// callers can quarantine the batch untouched instead of treating it as junk.
func (b Batch) validate(supportedSchema int) error {
	if err := checkNonEmptyID("changeId", b.ChangeID); err != nil {
		return err
	}
	if !IsUUIDv4(b.ChangeID) {
		return fmt.Errorf("changes: changeId %q is not a UUID v4: %w", b.ChangeID, ErrInvalidBatch)
	}
	if err := checkNonEmptyID("scope.serverId", b.Scope.ServerID); err != nil {
		return err
	}
	if err := checkNonEmptyID("scope.accountId", b.Scope.AccountID); err != nil {
		return err
	}
	if err := checkNonEmptyID("workspaceId", b.WorkspaceID); err != nil {
		return err
	}
	if err := checkNonEmptyID("deviceId", b.DeviceID); err != nil {
		return err
	}
	if b.HLC.WallMillis < 0 {
		return fmt.Errorf("changes: hlc.wallMillis %d is negative: %w", b.HLC.WallMillis, ErrInvalidBatch)
	}
	switch {
	case b.SchemaVersion > supportedSchema:
		return fmt.Errorf("changes: batch %s has schemaVersion %d, newest supported is %d: %w",
			b.ChangeID, b.SchemaVersion, supportedSchema, ErrNewerSchema)
	case b.SchemaVersion < MinSchemaVersion:
		return fmt.Errorf("changes: batch %s has schemaVersion %d, oldest supported is %d: %w",
			b.ChangeID, b.SchemaVersion, MinSchemaVersion, ErrInvalidBatch)
	}
	if len(b.Ops) == 0 {
		return fmt.Errorf("changes: batch %s has no ops: %w", b.ChangeID, ErrInvalidBatch)
	}
	if len(b.Ops) > MaxOpsPerBatch {
		return fmt.Errorf("changes: batch %s has %d ops, limit is %d: %w",
			b.ChangeID, len(b.Ops), MaxOpsPerBatch, ErrInvalidBatch)
	}
	seen := make(map[recordKey]struct{}, len(b.Ops))
	for i := range b.Ops {
		op := &b.Ops[i]
		if err := op.validate(); err != nil {
			return fmt.Errorf("changes: batch %s op %d: %w", b.ChangeID, i, err)
		}
		// One op per record per batch. Two ops for the same record at the
		// same stamp would make the result depend on slice order; the
		// capture layer must coalesce them before sealing the batch.
		key := recordKey{table: op.Table, id: op.RecordID}
		if _, dup := seen[key]; dup {
			return fmt.Errorf("changes: batch %s has two ops for %s/%s: %w",
				b.ChangeID, op.Table, op.RecordID, ErrInvalidBatch)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// Validate checks one op in isolation.
func (op Op) Validate() error { return op.validate() }

func (op Op) validate() error {
	if err := checkName("table", op.Table); err != nil {
		return err
	}
	if err := checkName("recordId", op.RecordID); err != nil {
		return err
	}
	if op.Deleted {
		if len(op.Fields) > 0 || len(op.Removed) > 0 {
			return fmt.Errorf("changes: deleted op for %s/%s must not carry fields: %w",
				op.Table, op.RecordID, ErrInvalidBatch)
		}
		return nil
	}
	if len(op.Fields)+len(op.Removed) > MaxFieldsPerOp {
		return fmt.Errorf("changes: op for %s/%s has %d fields, limit is %d: %w",
			op.Table, op.RecordID, len(op.Fields)+len(op.Removed), MaxFieldsPerOp, ErrInvalidBatch)
	}
	for name, raw := range op.Fields {
		if err := checkName("field name", name); err != nil {
			return err
		}
		if len(raw) == 0 || !json.Valid(raw) {
			return fmt.Errorf("changes: op for %s/%s field %q is not valid JSON: %w",
				op.Table, op.RecordID, name, ErrInvalidBatch)
		}
	}
	removedSeen := make(map[string]struct{}, len(op.Removed))
	for _, name := range op.Removed {
		if err := checkName("removed field name", name); err != nil {
			return err
		}
		if _, dup := removedSeen[name]; dup {
			return fmt.Errorf("changes: op for %s/%s removes field %q twice: %w",
				op.Table, op.RecordID, name, ErrInvalidBatch)
		}
		removedSeen[name] = struct{}{}
		if _, both := op.Fields[name]; both {
			return fmt.Errorf("changes: op for %s/%s both writes and removes field %q: %w",
				op.Table, op.RecordID, name, ErrInvalidBatch)
		}
	}
	return nil
}

// checkNonEmptyID validates a scope/workspace/device/change identifier:
// present, bounded, and free of whitespace padding (padded IDs are a classic
// source of "same" identifiers comparing unequal across layers).
func checkNonEmptyID(what, v string) error {
	if v == "" {
		return fmt.Errorf("changes: %s is empty: %w", what, ErrInvalidBatch)
	}
	if len(v) > MaxIDBytes && what != "changeId" {
		return fmt.Errorf("changes: %s is %d bytes, limit is %d: %w", what, len(v), MaxIDBytes, ErrInvalidBatch)
	}
	if v != strings.TrimSpace(v) {
		return fmt.Errorf("changes: %s %q has surrounding whitespace: %w", what, v, ErrInvalidBatch)
	}
	return nil
}

// checkName validates a table, record or field name.
func checkName(what, v string) error {
	if v == "" {
		return fmt.Errorf("changes: %s is empty: %w", what, ErrInvalidBatch)
	}
	if len(v) > MaxNameBytes {
		return fmt.Errorf("changes: %s is %d bytes, limit is %d: %w", what, len(v), MaxNameBytes, ErrInvalidBatch)
	}
	return nil
}
