// SPDX-License-Identifier: MIT

package server

import (
	"testing"
	"time"
)

func TestMintInviteCodeReturnsUnexpiredUniqueCode(t *testing.T) {
	s := openTestStore(t)
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	code, expiresAt, err := s.MintInviteCode(now)
	if err != nil {
		t.Fatalf("MintInviteCode: %v", err)
	}
	if len(code) != inviteCodeDigits {
		t.Fatalf("code = %q, want %d digits", code, inviteCodeDigits)
	}
	if !expiresAt.Equal(now.Add(InviteCodeTTL)) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, now.Add(InviteCodeTTL))
	}
	available, err := s.InviteCodeAvailable(code, now)
	if err != nil {
		t.Fatalf("InviteCodeAvailable: %v", err)
	}
	if !available {
		t.Fatalf("freshly minted code reported unavailable")
	}
}

func TestInviteCodeAvailableRejectsNeverMintedCode(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	// The critical asymmetry vs SetupCodeAvailable: a code never minted here
	// must never read as available, since this table (not an env var) is the
	// only source of truth for which invite codes are real.
	available, err := s.InviteCodeAvailable("000000", now)
	if err != nil {
		t.Fatalf("InviteCodeAvailable: %v", err)
	}
	if available {
		t.Fatalf("a never-minted invite code reported available")
	}
}

func TestConsumeInviteCodeIsSingleUse(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	code, _, err := s.MintInviteCode(now)
	if err != nil {
		t.Fatalf("MintInviteCode: %v", err)
	}
	ok, err := s.ConsumeInviteCode(code, now)
	if err != nil {
		t.Fatalf("first ConsumeInviteCode: %v", err)
	}
	if !ok {
		t.Fatalf("first consume = false, want true")
	}
	ok, err = s.ConsumeInviteCode(code, now)
	if err != nil {
		t.Fatalf("second ConsumeInviteCode: %v", err)
	}
	if ok {
		t.Fatalf("second consume of the same code succeeded, want single-use rejection")
	}
	available, err := s.InviteCodeAvailable(code, now)
	if err != nil {
		t.Fatalf("InviteCodeAvailable after consume: %v", err)
	}
	if available {
		t.Fatalf("a consumed code still reports available")
	}
}

func TestInviteCodeExpires(t *testing.T) {
	s := openTestStore(t)
	mintedAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	code, _, err := s.MintInviteCode(mintedAt)
	if err != nil {
		t.Fatalf("MintInviteCode: %v", err)
	}
	afterExpiry := mintedAt.Add(InviteCodeTTL + time.Second)
	available, err := s.InviteCodeAvailable(code, afterExpiry)
	if err != nil {
		t.Fatalf("InviteCodeAvailable: %v", err)
	}
	if available {
		t.Fatalf("an expired code reports available")
	}
	ok, err := s.ConsumeInviteCode(code, afterExpiry)
	if err != nil {
		t.Fatalf("ConsumeInviteCode: %v", err)
	}
	if ok {
		t.Fatalf("consuming an expired code succeeded")
	}
}

func TestListInviteCodesNewestFirst(t *testing.T) {
	s := openTestStore(t)
	base := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	first, _, err := s.MintInviteCode(base)
	if err != nil {
		t.Fatalf("mint first: %v", err)
	}
	second, _, err := s.MintInviteCode(base.Add(time.Minute))
	if err != nil {
		t.Fatalf("mint second: %v", err)
	}
	if _, err := s.ConsumeInviteCode(first, base.Add(2*time.Minute)); err != nil {
		t.Fatalf("consume first: %v", err)
	}
	rows, err := s.ListInviteCodes(10)
	if err != nil {
		t.Fatalf("ListInviteCodes: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].Code != second || rows[1].Code != first {
		t.Fatalf("rows = %+v, want newest (%s) first then %s", rows, second, first)
	}
	if !rows[0].ConsumedAt.IsZero() {
		t.Fatalf("second code ConsumedAt = %v, want zero (still outstanding)", rows[0].ConsumedAt)
	}
	if rows[1].ConsumedAt.IsZero() {
		t.Fatalf("first code ConsumedAt is zero, want a consumption timestamp")
	}
}
