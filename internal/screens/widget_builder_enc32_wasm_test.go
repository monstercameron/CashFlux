// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
)

func TestWidgetCanvasStorageKeepsPendingValuesWithoutBridge(t *testing.T) {
	for _, want := range []string{
		"window.__wbPendingStorage",
		"window.__wbStorageMemory",
		"Canvas changes are waiting to save.",
		"cashfluxDataSet",
		"cashfluxStoreSet",
	} {
		if !strings.Contains(vbDragShimJS, want) {
			t.Errorf("widget drag shim is missing %q", want)
		}
	}
	if strings.Contains(vbDragShimJS, "localStorage.setItem(") || strings.Contains(vbDragShimJS, "sessionStorage.setItem(") {
		t.Fatal("widget drag shim writes around encrypted browserstore")
	}
}
