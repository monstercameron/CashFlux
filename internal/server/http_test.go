package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	valid := Config{Addr: ":0", DataDir: t.TempDir(), AuthMode: "token"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	invalid := valid
	invalid.AuthMode = "magic"
	if err := invalid.Validate(); err == nil {
		t.Fatal("unsupported auth mode accepted")
	}
	invalid = valid
	invalid.MasterKey = "short"
	if err := invalid.Validate(); err == nil {
		t.Fatal("short master key accepted")
	}
}

func TestHealthReadyAndVersionEndpoints(t *testing.T) {
	h := NewMux(Config{AuthMode: "token", Billing: false})
	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("%s status = %d, want 204", path, rr.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("version status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var body VersionResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if body.APIVersion != APIVersion || body.MinClientAPIVersion != MinClientAPIVersion {
		t.Fatalf("version body = %+v", body)
	}
	if body.AuthMode != "token" || body.BillingEnabled {
		t.Fatalf("mode flags = %+v", body)
	}
}
