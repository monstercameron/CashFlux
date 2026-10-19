// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"encoding/json"
	"strings"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/workspace"
)

// Record recovery copies retain the checkpoint AND its journal. Copying only
// the older checkpoint would make the existing recovery download incomplete.
// A future outer schema prevents normal dataset import from accepting this
// diagnostic bundle as an empty household.
type recordRecovery struct {
	SchemaVersion int               `json:"schemaVersion"`
	Format        string            `json:"cashfluxRecordRecovery"`
	Workspace     string            `json:"workspace"`
	Checkpoint    string            `json:"checkpoint"`
	Head          string            `json:"head"`
	Records       map[string]string `json:"records,omitempty"`
	ReadErrors    map[string]string `json:"readErrors,omitempty"`
}

func recoveryWorkspaceName() string {
	raw, _, err := browserstore.GetSecure(workspacesKey)
	if err != nil {
		return ""
	}
	var registry workspace.Registry
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &registry); err != nil {
			return ""
		}
	}
	if registry.ActiveID != "" {
		return registry.ActiveID
	}
	return defaultWSID
}

func datasetRecoveryCopy(raw, workspace string, keys []string, read func(string) (string, bool, error)) string {
	var existing struct {
		Format string `json:"cashfluxRecordRecovery"`
	}
	_ = json.Unmarshal([]byte(raw), &existing)
	if existing.Format != "" {
		return raw
	}
	head, _, headErr := read(datasetHeadKey)
	_, recordFormat, _ := checkpointDatasetJSON([]byte(raw))
	if !recordFormat && head == "" && headErr == nil {
		return raw
	}
	copy := recordRecovery{SchemaVersion: store.SchemaVersion + 1, Format: "v1", Workspace: workspace, Checkpoint: raw, Head: head, Records: map[string]string{}, ReadErrors: map[string]string{}}
	if headErr != nil {
		copy.ReadErrors[datasetHeadKey] = headErr.Error()
	}
	if workspace == "" {
		copy.ReadErrors["workspace"] = "The workspace could not be resolved. The original protected rows have been kept."
	}
	prefix := recordWorkspacePrefix(workspace)
	for _, key := range keys {
		if workspace == "" || !strings.HasPrefix(key, prefix) {
			continue
		}
		value, present, err := read(key)
		if err != nil {
			copy.ReadErrors[key] = err.Error()
			continue
		}
		if present {
			copy.Records[key] = value
		}
	}
	encoded, err := json.Marshal(copy)
	if err != nil {
		return raw
	}
	return string(encoded)
}
