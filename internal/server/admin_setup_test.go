// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestAdminSetupCreatesFirstOwnerWithRecoveryThenCloses(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "first-owner-key", SessionKey: "admin-setup-test-key"}
	h := NewMux(cfg, store)

	status := adminSessionRequest(t, h, http.MethodGet, "/v1/admin/setup", "", nil, "")
	if status.Code != http.StatusOK {
		t.Fatalf("initial setup status = %d body %q", status.Code, status.Body.String())
	}
	var initial AdminSetupStatusResponse
	if err := json.NewDecoder(status.Body).Decode(&initial); err != nil || !initial.Required {
		t.Fatalf("initial setup response = %+v err=%v", initial, err)
	}

	bad := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/setup",
		`{"setupKey":"wrong","username":"cam","password":"owner-password"}`, nil, "")
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("invalid key status = %d body %q", bad.Code, bad.Body.String())
	}

	created := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/setup",
		`{"setupKey":"first-owner-key","username":"cam","password":"owner-password"}`, nil, "")
	if created.Code != http.StatusOK {
		t.Fatalf("create owner status = %d body %q", created.Code, created.Body.String())
	}
	var result AdminSetupResponse
	if err := json.NewDecoder(created.Body).Decode(&result); err != nil {
		t.Fatalf("decode create owner: %v", err)
	}
	if result.UserID != "local:cam" || result.Username != "cam" || result.RecoveryCode == "" {
		t.Fatalf("create owner response = %+v", result)
	}

	user, recoveryHash, found, err := store.GetLocalRecoveryByUsername("cam")
	if err != nil || !found {
		t.Fatalf("get recovery = found %v err %v", found, err)
	}
	role, err := store.UserRole(user.ID)
	if err != nil || role != RoleOwner {
		t.Fatalf("created role = %q err=%v", role, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(recoveryHash), []byte(result.RecoveryCode)); err != nil {
		t.Fatalf("recovery code does not match persisted hash: %v", err)
	}
	if recoveryHash == result.RecoveryCode {
		t.Fatal("plaintext recovery code was persisted")
	}

	closed := adminSessionRequest(t, h, http.MethodGet, "/v1/admin/setup", "", nil, "")
	var final AdminSetupStatusResponse
	if err := json.NewDecoder(closed.Body).Decode(&final); err != nil || final.Required {
		t.Fatalf("final setup response = %+v err=%v", final, err)
	}
	again := adminSessionRequest(t, h, http.MethodPost, "/v1/admin/setup",
		`{"setupKey":"first-owner-key","username":"other","password":"owner-password"}`, nil, "")
	if again.Code != http.StatusPreconditionFailed {
		t.Fatalf("second setup status = %d body %q", again.Code, again.Body.String())
	}

	cookies, session := loginAdminSession(t, h, "cam", "owner-password")
	if !session.Authenticated || session.Role != RoleOwner || cookies[adminAccessCookie] == nil {
		t.Fatalf("owner login session = %+v cookies=%v", session, cookies)
	}
}

func TestAdminSetupRequiresConsoleOrigin(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{AuthMode: "token", Token: "first-owner-key", SessionKey: "admin-setup-test-key"}
	h := NewMux(cfg, store)
	req := httptest.NewRequest(http.MethodPost, "https://budget.example/v1/admin/setup",
		http.NoBody)
	req.Host = "budget.example"
	req.Header.Set("Origin", "https://attacker.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("cross-origin setup = %d body %q", rr.Code, rr.Body.String())
	}
	hasOwner, err := store.HasOwner()
	if err != nil || hasOwner {
		t.Fatalf("owner after cross-origin attempt = %v err=%v", hasOwner, err)
	}
}

func TestAdminSetupStaysRequiredWhenOnlyMembersExist(t *testing.T) {
	store := openTestStore(t)
	createAdminSessionUser(t, store, "member", "member-password", RoleMember)
	h := NewMux(Config{AuthMode: "token", Token: "first-owner-key"}, store)
	rr := adminSessionRequest(t, h, http.MethodGet, "/v1/admin/setup", "", nil, "")
	var response AdminSetupStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil || !response.Required {
		t.Fatalf("setup status with member = %+v err=%v", response, err)
	}
}
