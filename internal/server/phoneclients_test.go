// SPDX-License-Identifier: MIT

package server

import (
	"testing"
	"time"
)

func TestListPhoneClientsExcludesUnverifiedAttempt(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)

	// An abandoned attempt: ensurePhoneUser's upsert runs the moment someone
	// merely requests a code, before ever verifying.
	if err := s.UpsertUser(User{ID: phoneUserID("+15550000001"), Provider: "phone", Subject: "+15550000001", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser (unverified): %v", err)
	}

	// A genuinely enrolled account.
	verifiedID := phoneUserID("+15550000002")
	if err := s.UpsertUser(User{ID: verifiedID, Provider: "phone", Subject: "+15550000002", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser (verified): %v", err)
	}
	if err := s.MarkPhoneVerified(verifiedID, now); err != nil {
		t.Fatalf("MarkPhoneVerified: %v", err)
	}

	rows, err := s.ListPhoneClients(10)
	if err != nil {
		t.Fatalf("ListPhoneClients: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (unverified attempt must be excluded): %+v", len(rows), rows)
	}
	if rows[0].PhoneNumber != "+15550000002" {
		t.Fatalf("rows[0].PhoneNumber = %q, want +15550000002", rows[0].PhoneNumber)
	}
	if rows[0].PhoneVerifiedAt.IsZero() {
		t.Fatalf("rows[0].PhoneVerifiedAt is zero, want the verification timestamp")
	}
	if rows[0].Suspended {
		t.Fatalf("rows[0].Suspended = true, want false")
	}
}

func TestListPhoneClientsReportsSuspended(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	userID := phoneUserID("+15550000003")
	if err := s.UpsertUser(User{ID: userID, Provider: "phone", Subject: "+15550000003", CreatedAt: now}); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := s.MarkPhoneVerified(userID, now); err != nil {
		t.Fatalf("MarkPhoneVerified: %v", err)
	}
	if err := s.SetUserSuspended(userID, true, now); err != nil {
		t.Fatalf("SetUserSuspended: %v", err)
	}
	rows, err := s.ListPhoneClients(10)
	if err != nil {
		t.Fatalf("ListPhoneClients: %v", err)
	}
	if len(rows) != 1 || !rows[0].Suspended {
		t.Fatalf("rows = %+v, want exactly one suspended row", rows)
	}
}
