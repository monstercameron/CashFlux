// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// The whole point of WF4-b: as a plain float64 there was no way to say "nobody
// has told us", so every consumer read `<= 0` as unknown — which claims a
// genuine 0% promotional card or a no-interest family loan is missing its rate.
func TestAZeroRateIsNotAMissingRate(t *testing.T) {
	set := Account{}.WithRateAPR(0)
	r, ok := set.RateAPR()
	if !ok {
		t.Error("a recorded 0% read as no rate on file")
	}
	if r != 0 {
		t.Errorf("rate = %v, want 0", r)
	}
	if !set.HasRateAPR() {
		t.Error("HasRateAPR was false for a recorded 0%")
	}

	var unset Account
	if _, ok := unset.RateAPR(); ok {
		t.Error("an account nobody filled in reported a rate")
	}
	if unset.HasRateAPR() {
		t.Error("HasRateAPR was true with nothing on file")
	}
}

func TestRateAPROrZeroIsForArithmeticOnly(t *testing.T) {
	// It answers 0 for both states on purpose — callers that must distinguish
	// them use RateAPR, and this exists so the ones that genuinely do not are
	// explicit rather than dereferencing a pointer.
	if got := (Account{}).RateAPROrZero(); got != 0 {
		t.Errorf("unset = %v, want 0", got)
	}
	if got := (Account{}).WithRateAPR(6.5).RateAPROrZero(); got != 6.5 {
		t.Errorf("set = %v, want 6.5", got)
	}
}

// Two copies of an account must never share one pointer, or editing the rate on
// a working copy reaches back through to the stored one.
func TestCopiesDoNotShareTheRatePointer(t *testing.T) {
	a := Account{ID: "a"}.WithRateAPR(5)
	b := a.WithRateAPR(9)
	if r, _ := a.RateAPR(); r != 5 {
		t.Errorf("the original changed to %v when a copy was given a new rate", r)
	}
	if r, _ := b.RateAPR(); r != 9 {
		t.Errorf("the copy = %v, want 9", r)
	}
	c := a.WithoutRateAPR()
	if !a.HasRateAPR() {
		t.Error("clearing the rate on a copy cleared it on the original")
	}
	if c.HasRateAPR() {
		t.Error("WithoutRateAPR left a rate on file")
	}
}

// A dataset written before this change never stored a zero (the field was a
// float64 with omitempty), so absence decodes to "unknown" — the same thing
// those datasets already meant. Nothing needs migrating.
func TestALegacyDatasetWithoutTheKeyReadsAsNoRate(t *testing.T) {
	var a Account
	if err := json.Unmarshal([]byte(`{"id":"acc-1","name":"Old"}`), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.HasRateAPR() {
		t.Error("an account with no rate key decoded as having one")
	}
}

func TestAnExplicitZeroSurvivesTheRoundTrip(t *testing.T) {
	in := Account{ID: "acc-1", Name: "Family loan"}.WithRateAPR(0)
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(raw), `"interestRateApr":0`) {
		t.Fatalf("a recorded 0%% was not written: %s", raw)
	}
	var out Account
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r, ok := out.RateAPR(); !ok || r != 0 {
		t.Errorf("round-trip lost the recorded zero: rate=%v ok=%v", r, ok)
	}
}

func TestNoRateWritesNoKey(t *testing.T) {
	raw, err := json.Marshal(Account{ID: "acc-1"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(raw), "interestRateApr") {
		t.Errorf("an account with no rate wrote the key: %s", raw)
	}
}
