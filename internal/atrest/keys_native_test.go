//go:build !js || !wasm

package atrest

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/keyring"
)

func TestKeyServiceProductionPBKDF2(t *testing.T) {
	store := &memoryKeyStore{}
	s, err := OpenKeyService(store, keysScope, "workspace-a", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Lock()
	envelope, err := s.Seal("cashflux:dataset", []byte("household secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EnableAppLock("original"); err != nil {
		t.Fatal(err)
	}
	entry := s.State().Ring.Current().Wraps[0]
	if entry.Iterations != 600000 || len(entry.Salt) != keyring.PasscodeSaltSize {
		t.Fatal("incorrect production PBKDF2 parameters")
	}
	s.Lock()
	reloaded, err := OpenKeyService(store, keysScope, "workspace-a", true)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Lock()
	if err := reloaded.UnlockPasscode("wrong"); !errors.Is(err, keyring.ErrUnwrapFailed) {
		t.Fatalf("wrong passcode: %v", err)
	}
	if err := reloaded.UnlockPasscode("original"); err != nil {
		t.Fatal(err)
	}
	assertRecord(t, reloaded, envelope)
}
