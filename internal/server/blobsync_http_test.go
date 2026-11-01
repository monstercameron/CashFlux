// SPDX-License-Identifier: MIT

//go:build !js || !wasm

package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestBlobSyncVersionExposesPersistentV2Identity(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "version.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := NewMux(Config{AuthMode: "token"}, store)
	read := func() VersionResponse {
		t.Helper()
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/v1/version", nil))
		if rr.Code != 200 {
			t.Fatalf("version status=%d body=%s", rr.Code, rr.Body.String())
		}
		var got VersionResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	first, second := read(), read()
	if first.ServerID == "" || first.ServerID != second.ServerID {
		t.Fatalf("server ids %q and %q", first.ServerID, second.ServerID)
	}
	if first.APIVersion != "2" || first.MinClientAPIVersion != "1" {
		t.Fatalf("version response=%+v", first)
	}
}
