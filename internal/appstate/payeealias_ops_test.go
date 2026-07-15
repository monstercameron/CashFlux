// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestPutPayeeAliasIdempotentMerge(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := a.PutPayeeAlias(domain.PayeeAlias{RawPayee: "AMZN Mktp US*2K4RT0", Display: "Amazon"}); err != nil {
		t.Fatalf("put 1: %v", err)
	}
	// Same raw payee, different casing → should update in place, not duplicate.
	if err := a.PutPayeeAlias(domain.PayeeAlias{RawPayee: "amzn mktp us*2k4rt0", Display: "Amazon Books"}); err != nil {
		t.Fatalf("put 2: %v", err)
	}
	got := a.PayeeAliases()
	if len(got) != 1 {
		t.Fatalf("want 1 alias, got %d: %+v", len(got), got)
	}
	if got[0].Display != "Amazon Books" {
		t.Fatalf("display = %q, want Amazon Books", got[0].Display)
	}

	// Resolver reflects the learned alias and rule-pack fallback.
	r := a.PayeeResolver()
	if r.Resolve("AMZN Mktp US*2K4RT0") != "Amazon Books" {
		t.Errorf("learned resolve = %q", r.Resolve("AMZN Mktp US*2K4RT0"))
	}
	if a.ResolvePayee("SQ *BLUE BOTTLE") != "Blue Bottle" {
		t.Errorf("rule pack resolve = %q", a.ResolvePayee("SQ *BLUE BOTTLE"))
	}

	// Validation.
	if err := a.PutPayeeAlias(domain.PayeeAlias{RawPayee: "  ", Display: "X"}); err == nil {
		t.Error("expected error for blank raw payee")
	}
}
