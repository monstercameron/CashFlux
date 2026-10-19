// SPDX-License-Identifier: MIT

package importdraft

import (
	"github.com/monstercameron/CashFlux/internal/extract"
	"testing"
)

func TestSessionMetadataIsIsolatedAndIdentityScoped(t *testing.T) {
	m := New(nil)
	m.Append("pdf", []extract.Row{{ID: "a"}})
	m.Errors["a"] = "Fix date"
	meta := m.Metadata()
	meta.Sources["missing"] = "receipt"
	meta.Errors["missing"] = "wrong row"
	restored := New(m.Rows)
	restored.RestoreMetadata(meta)
	if restored.Kind() != "pdf" || restored.Errors["a"] != "Fix date" || restored.Errors["missing"] != "" {
		t.Fatal(restored)
	}
	meta.Errors["a"] = "changed"
	if m.Errors["a"] != "Fix date" || restored.Errors["a"] != "Fix date" {
		t.Fatal("metadata alias")
	}
	restored.RestoreMetadata(Metadata{})
}
