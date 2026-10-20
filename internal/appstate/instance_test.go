// SPDX-License-Identifier: MIT

package appstate

import (
	"io"
	"testing"
)

func TestInstanceIdentitySeparatesStoresAtSameRevision(t *testing.T) {
	first, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Store().Close() })
	second, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Store().Close() })
	if first.Rev() != second.Rev() {
		t.Fatal("fixture requires both stores at the same global revision")
	}
	if first.InstanceID() == 0 || first.InstanceID() == second.InstanceID() {
		t.Fatalf("store identities are not distinct: %d / %d", first.InstanceID(), second.InstanceID())
	}
	before := first.InstanceID()
	settings := first.Settings()
	settings.BaseCurrency = "EUR"
	if err := first.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if first.InstanceID() != before {
		t.Fatal("mutation changed instance identity")
	}
}
