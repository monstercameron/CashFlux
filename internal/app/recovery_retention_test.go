package app

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/browserstore"
)

func TestBOOTRecoveryRecordVersionsPreserveIntegerMoneyAndDeletions(t *testing.T) {
	a := `{"schemaVersion":2,"transactions":[{"id":"one","amount":9007199254740993,"desc":"saved"}],"settings":{"baseCurrency":"USD"}}`
	reordered := `{"settings":{"baseCurrency":"USD"},"transactions":[{"desc":"saved","amount":9007199254740993,"id":"one"}],"schemaVersion":2}`
	if !reflect.DeepEqual(recoveryRecordVersions(a), recoveryRecordVersions(reordered)) {
		t.Fatal("same records got different versions")
	}
	changed := `{"schemaVersion":2,"transactions":[{"id":"one","amount":9007199254740992,"desc":"saved"}],"settings":{"baseCurrency":"USD"}}`
	if reflect.DeepEqual(recoveryRecordVersions(a), recoveryRecordVersions(changed)) {
		t.Fatal("integer money version was rounded")
	}
	deleted := `{"schemaVersion":2,"transactions":[],"settings":{"baseCurrency":"USD"}}`
	// Membership versions ensure a stale deletion is not mistaken for an edit
	// already represented by a live dataset containing additional rows.
	old := browserstore.RecoveryEntry{Key: "deleted", Order: 1, RecoveryDescription: browserstore.RecoveryDescription{Group: "scope", Hash: "deleted", Protect: true, Versions: recoveryRecordVersions(deleted)}}
	entries := []browserstore.RecoveryEntry{old}
	for i := uint64(2); i <= 4; i++ {
		entries = append(entries, browserstore.RecoveryEntry{Key: string(rune('a' + i)), Order: i, RecoveryDescription: browserstore.RecoveryDescription{Group: "scope", Hash: string(rune('a' + i)), Protect: true}})
	}
	keep, remove := browserstore.PlanRecoveryRetention(entries, map[string][]string{"scope": recoveryRecordVersions(a)})
	if len(keep) != 4 || len(remove) != 0 {
		t.Fatal("stale deletion was discarded")
	}
	if recoveryRecordVersions("unreadable") != nil {
		t.Fatal("unreadable payload got containment proof")
	}
}

func TestBOOTRecoveryScopeNeverAdoptsCurrentOwnerForLegacyCopies(t *testing.T) {
	a := describeRecoveryPayload("cashflux:stale-recovery:workspace-hash:owner-a:1", `{"schemaVersion":2}`)
	b := describeRecoveryPayload("cashflux:stale-recovery:workspace-hash:owner-b:2", `{"schemaVersion":2}`)
	legacy := describeRecoveryPayload("cashflux:stale-recovery:workspace-hash:old-uuid", `{"schemaVersion":2}`)
	if a.Group == b.Group || a.Group == legacy.Group || b.Group == legacy.Group {
		t.Fatal("recovery owners mixed")
	}
	if !a.Protect || !legacy.Protect {
		t.Fatal("stale copies lost edit protection")
	}
}

func TestBOOTQuarantineScopeBoundEvenForOpaquePayload(t *testing.T) {
	a := describeRecoveryPayload("cashflux:quarantine:1700000000:workspace-hash:owner-a:random", "unreadable bytes")
	b := describeRecoveryPayload("cashflux:quarantine:1700000000:workspace-hash:owner-b:random", "unreadable bytes")
	if a.Group == b.Group {
		t.Fatal("opaque quarantine copies mixed account scopes")
	}
}
