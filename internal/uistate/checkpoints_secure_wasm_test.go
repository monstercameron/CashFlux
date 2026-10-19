//go:build js && wasm

package uistate

import (
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

func TestDurableCheckpointRequiresAnOpenHousehold(t *testing.T) {
	prior := appstate.Default
	appstate.Default = nil
	t.Cleanup(func() { appstate.Default = prior })
	called := false
	SaveCheckpointThen("Before changes", func(id string, err error) {
		called = true
		if id != "" || err == nil {
			t.Errorf("unavailable checkpoint acknowledged: %q %v", id, err)
		}
	})
	if !called {
		t.Fatal("missing-household failure was not reported")
	}
}

func TestDurableCheckpointDoesNotAcknowledgeUnavailableStorage(t *testing.T) {
	a, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Store().Close() })
	prior := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = prior })
	done := make(chan error, 1)
	SaveCheckpointThen("Before changes", func(id string, err error) {
		if id != "" {
			t.Errorf("failed checkpoint returned a saved ID: %q", id)
		}
		done <- err
	})
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("unavailable storage was acknowledged")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("checkpoint failure was not reported")
	}
}
