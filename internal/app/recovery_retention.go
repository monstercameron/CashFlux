package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/undosnap"
)

func recoveryFingerprint(raw string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) }

func recoveryScopeGroup(class, workspace, owner string) string {
	raw, _ := json.Marshal([]string{class, workspace, owner})
	return string(raw)
}

// Exact canonical row bytes act as record versions for legacy exports which do
// not carry v2 clocks. Membership fingerprints also protect deletions; merely
// finding every surviving row in another dataset does not prove containment.
func recoveryRecordVersions(raw string) []string {
	var top map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &top) != nil || top["schemaVersion"] == nil {
		return nil
	}
	snap, err := undosnap.ToSnapshot([]byte(raw))
	if err != nil {
		return nil
	}
	versions := make([]string, 0)
	for table, rows := range snap {
		ids := make([]string, 0, len(rows))
		for id, row := range rows {
			var canonical any
			// json.Number preserves integer minor units beyond float64 precision.
			decoder := json.NewDecoder(strings.NewReader(string(row)))
			decoder.UseNumber()
			if decoder.Decode(&canonical) != nil {
				return nil
			}
			encoded, err := json.Marshal(canonical)
			if err != nil {
				return nil
			}
			versions = append(versions, recoveryFingerprint(table+"\x00"+id+"\x00"+string(encoded)))
			ids = append(ids, id)
		}
		sort.Strings(ids)
		membership, _ := json.Marshal(ids)
		versions = append(versions, recoveryFingerprint(table+"\x00membership\x00"+string(membership)))
	}
	sort.Strings(versions)
	return versions
}

func describeRecoveryPayload(key, raw string) browserstore.RecoveryDescription {
	desc := browserstore.RecoveryDescription{Hash: recoveryFingerprint(raw)}
	if strings.HasPrefix(key, "cashflux:stale-recovery:") {
		parts := strings.Split(strings.TrimPrefix(key, "cashflux:stale-recovery:"), ":")
		scope := "unbound"
		if len(parts) >= 3 {
			scope = parts[1]
		}
		workspace := "unknown"
		if len(parts) > 0 {
			workspace = parts[0]
		}
		desc.Group = recoveryScopeGroup("stale", workspace, scope)
		desc.Protect = true
		desc.Versions = recoveryRecordVersions(raw)
		return desc
	}
	parts := strings.Split(strings.TrimPrefix(key, "cashflux:quarantine:"), ":")
	if len(parts) == 4 {
		desc.Group = recoveryScopeGroup("quarantine", parts[1], parts[2])
		return desc
	}
	if strings.HasPrefix(key, "cashflux:quarantine:credvault:") {
		desc.Group = recoveryScopeGroup("credential quarantine", "legacy-unbound", "")
		desc.Protect = true
		return desc
	}
	var copy struct {
		Workspace string `json:"workspace"`
		Owner     string `json:"owner"`
		Format    string `json:"cashfluxRecordRecovery"`
	}
	if json.Unmarshal([]byte(raw), &copy) == nil && copy.Format == "v1" {
		desc.Group = recoveryScopeGroup("quarantine", recoveryFingerprint(copy.Workspace), recoveryFingerprint(copy.Owner))
	} else {
		// Never assign the current account to an old unbound recovery copy.
		desc.Group = recoveryScopeGroup("quarantine", "legacy-unbound", "")
	}
	return desc
}
