// SPDX-License-Identifier: MIT

package backup

import (
	"encoding/json"
	"fmt"
)

// EnvelopeSchemaVersion is the current full-backup envelope version. Bump it when
// the envelope shape changes.
const EnvelopeSchemaVersion = 1

// Appearance holds the device-local appearance side-state that lives outside the
// dataset (its own localStorage keys), so a full backup carries it too. Each field
// is the verbatim stored value (often itself JSON or a data URL); empty = absent.
type Appearance struct {
	Theme  string `json:"theme,omitempty"`
	Fonts  string `json:"fonts,omitempty"`
	Banner string `json:"banner,omitempty"`
	Prefs  string `json:"prefs,omitempty"`
}

// Envelope is the "back up everything" container: every workspace's dataset
// (verbatim JSON strings), the workspace registry value, and the appearance
// side-state. Unlike a single-workspace dataset export, it captures the whole
// install so a migration is lossless (L9). The values are opaque blobs; the app
// gathers/restores them from localStorage and this package frames + versions them.
type Envelope struct {
	SchemaVersion     int        `json:"schemaVersion"`
	Datasets          []string   `json:"datasets"`                    // each workspace's dataset JSON, verbatim
	WorkspaceRegistry string     `json:"workspaceRegistry,omitempty"` // the cashflux:workspaces value
	Appearance        Appearance `json:"appearance"`
}

// MarshalEnvelope serializes a full-backup envelope to indented JSON, stamping the
// current schema version.
func MarshalEnvelope(e Envelope) ([]byte, error) {
	e.SchemaVersion = EnvelopeSchemaVersion
	out, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("backup: marshal envelope: %w", err)
	}
	return out, nil
}

// UnmarshalEnvelope parses a full-backup envelope and migrates it to the current
// schema. Unversioned data (version 0) is treated as the initial version; a
// version newer than supported is rejected rather than silently mishandled.
func UnmarshalEnvelope(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, fmt.Errorf("backup: unmarshal envelope: %w", err)
	}
	if e.SchemaVersion == 0 {
		e.SchemaVersion = 1
	}
	if e.SchemaVersion > EnvelopeSchemaVersion {
		return Envelope{}, fmt.Errorf("backup: envelope schema version %d is newer than supported version %d", e.SchemaVersion, EnvelopeSchemaVersion)
	}
	return e, nil
}

// IsEnvelope reports whether data is a full-backup envelope (it has a datasets
// array) rather than a single-workspace dataset export, so an import can detect
// which it received and restore accordingly.
func IsEnvelope(data []byte) bool {
	var probe struct {
		Datasets *json.RawMessage `json:"datasets"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Datasets != nil
}
