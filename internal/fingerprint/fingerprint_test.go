// SPDX-License-Identifier: MIT

package fingerprint_test

import (
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/fingerprint"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ── NormalizePayee ────────────────────────────────────────────────────────────

func TestNormalizePayee(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Whitespace handling
		{
			name:  "trim leading and trailing spaces",
			input: "  Amazon  ",
			want:  "amazon",
		},
		{
			name:  "collapse internal whitespace",
			input: "Whole  Foods   Market",
			want:  "whole foods market",
		},
		{
			name:  "tab and newline collapse",
			input: "Starbucks\t #123",
			want:  "starbucks #123",
		},

		// Case normalisation
		{
			name:  "uppercase input lowercased",
			input: "WALMART SUPERCENTER",
			want:  "walmart supercenter",
		},
		{
			name:  "mixed case lowercased",
			input: "McDonalds",
			want:  "mcdonalds",
		},

		// POS prefix stripping
		{
			name:  "hash prefix stripped",
			input: "# STARBUCKS 0042",
			want:  "starbucks 0042",
		},
		{
			name:  "asterisk prefix stripped",
			input: "*NETFLIX.COM",
			want:  "netflix.com",
		},
		{
			name:  "double asterisk prefix stripped",
			input: "**NETFLIX.COM",
			want:  "netflix.com",
		},
		{
			name:  "slash prefix stripped",
			input: "//MERCHANT",
			want:  "merchant",
		},
		{
			name:  "hash-space prefix stripped",
			input: "#  WHOLE FOODS",
			want:  "whole foods",
		},
		{
			name:  "no prefix left untouched",
			input: "Spotify",
			want:  "spotify",
		},
		{
			name:  "all-noise string becomes empty",
			input: "###***",
			want:  "",
		},
		{
			name:  "empty string stays empty",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace becomes empty",
			input: "   ",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := fingerprint.NormalizePayee(tc.input)
			if got != tc.want {
				t.Errorf("NormalizePayee(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ── Fingerprint ───────────────────────────────────────────────────────────────

func TestFingerprint(t *testing.T) {
	t.Parallel()

	baseDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	const baseAmount = int64(-4200)
	const basePayee = "Starbucks"
	const baseAccount = "acc-1"

	base := fingerprint.Fingerprint(baseDate, baseAmount, basePayee, baseAccount)

	t.Run("length is 16 hex chars", func(t *testing.T) {
		t.Parallel()
		if len(base) != 16 {
			t.Errorf("Fingerprint length = %d, want 16", len(base))
		}
		for _, ch := range base {
			if !('0' <= ch && ch <= '9') && !('a' <= ch && ch <= 'f') {
				t.Errorf("Fingerprint %q contains non-hex char %q", base, ch)
			}
		}
	})

	t.Run("deterministic — same inputs produce same output", func(t *testing.T) {
		t.Parallel()
		got := fingerprint.Fingerprint(baseDate, baseAmount, basePayee, baseAccount)
		if got != base {
			t.Errorf("Fingerprint not deterministic: %q vs %q", got, base)
		}
	})

	t.Run("POS prefix stripped before comparison", func(t *testing.T) {
		t.Parallel()
		fp1 := fingerprint.Fingerprint(baseDate, baseAmount, "# STARBUCKS", baseAccount)
		fp2 := fingerprint.Fingerprint(baseDate, baseAmount, "STARBUCKS", baseAccount)
		if fp1 != fp2 {
			t.Errorf("POS-prefixed and clean payee produced different fingerprints: %q vs %q", fp1, fp2)
		}
	})

	t.Run("case-insensitive payee matching", func(t *testing.T) {
		t.Parallel()
		fp1 := fingerprint.Fingerprint(baseDate, baseAmount, "starbucks", baseAccount)
		fp2 := fingerprint.Fingerprint(baseDate, baseAmount, "STARBUCKS", baseAccount)
		if fp1 != fp2 {
			t.Errorf("Case difference produced different fingerprints: %q vs %q", fp1, fp2)
		}
	})

	t.Run("different date changes fingerprint", func(t *testing.T) {
		t.Parallel()
		otherDate := baseDate.AddDate(0, 0, 1)
		got := fingerprint.Fingerprint(otherDate, baseAmount, basePayee, baseAccount)
		if got == base {
			t.Error("Different date produced same fingerprint")
		}
	})

	t.Run("different amount changes fingerprint", func(t *testing.T) {
		t.Parallel()
		got := fingerprint.Fingerprint(baseDate, baseAmount+1, basePayee, baseAccount)
		if got == base {
			t.Error("Different amount produced same fingerprint")
		}
	})

	t.Run("sign matters — income vs expense differ", func(t *testing.T) {
		t.Parallel()
		income := fingerprint.Fingerprint(baseDate, -baseAmount, basePayee, baseAccount)
		expense := fingerprint.Fingerprint(baseDate, baseAmount, basePayee, baseAccount)
		if income == expense {
			t.Error("Positive and negative amounts produced same fingerprint")
		}
	})

	t.Run("different payee changes fingerprint", func(t *testing.T) {
		t.Parallel()
		got := fingerprint.Fingerprint(baseDate, baseAmount, "Dunkin", baseAccount)
		if got == base {
			t.Error("Different payee produced same fingerprint")
		}
	})

	t.Run("different account changes fingerprint", func(t *testing.T) {
		t.Parallel()
		got := fingerprint.Fingerprint(baseDate, baseAmount, basePayee, "acc-2")
		if got == base {
			t.Error("Different account produced same fingerprint")
		}
	})
}

// ── TxFingerprint ─────────────────────────────────────────────────────────────

func TestTxFingerprint(t *testing.T) {
	t.Parallel()

	baseDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	baseTx := domain.Transaction{
		ID:        "tx-1",
		AccountID: "acc-1",
		Date:      baseDate,
		Payee:     "Starbucks",
		Desc:      "POS #1234 STARBUCKS",
		Amount:    money.New(-4200, "USD"),
	}

	t.Run("matches direct Fingerprint call with Payee", func(t *testing.T) {
		t.Parallel()
		want := fingerprint.Fingerprint(baseDate, -4200, "Starbucks", "acc-1")
		got := fingerprint.TxFingerprint(baseTx)
		if got != want {
			t.Errorf("TxFingerprint = %q, want %q", got, want)
		}
	})

	t.Run("falls back to Desc when Payee is empty", func(t *testing.T) {
		t.Parallel()
		tx := baseTx
		tx.Payee = ""
		want := fingerprint.Fingerprint(baseDate, -4200, "POS #1234 STARBUCKS", "acc-1")
		got := fingerprint.TxFingerprint(tx)
		if got != want {
			t.Errorf("TxFingerprint with empty Payee = %q, want %q", got, want)
		}
	})

	t.Run("Payee takes precedence over Desc", func(t *testing.T) {
		t.Parallel()
		fpWithPayee := fingerprint.TxFingerprint(baseTx)
		txDescOnly := baseTx
		txDescOnly.Payee = ""
		fpDescOnly := fingerprint.TxFingerprint(txDescOnly)
		// They should differ because Payee != Desc.
		if fpWithPayee == fpDescOnly {
			t.Error("Payee and Desc are different strings but produced the same fingerprint")
		}
	})
}

// ── GroupDuplicates ───────────────────────────────────────────────────────────

func TestGroupDuplicates(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	makeTx := func(id, accountID, payee string, amount int64, date time.Time) domain.Transaction {
		return domain.Transaction{
			ID:        id,
			AccountID: accountID,
			Date:      date,
			Payee:     payee,
			Amount:    money.New(amount, "USD"),
		}
	}

	t.Run("no duplicates returns empty slice", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "Starbucks", -4200, day),
			makeTx("b", "acc-1", "Amazon", -9999, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 0 {
			t.Errorf("expected no groups, got %d", len(groups))
		}
	})

	t.Run("empty input returns empty slice", func(t *testing.T) {
		t.Parallel()
		groups := fingerprint.GroupDuplicates(nil)
		if len(groups) != 0 {
			t.Errorf("expected no groups, got %d", len(groups))
		}
	})

	t.Run("two identical transactions form one group", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "Starbucks", -4200, day),
			makeTx("b", "acc-1", "Starbucks", -4200, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0]) != 2 {
			t.Errorf("expected 2 members in group, got %d", len(groups[0]))
		}
		ids := []string{groups[0][0].ID, groups[0][1].ID}
		sort.Strings(ids)
		if ids[0] != "a" || ids[1] != "b" {
			t.Errorf("unexpected group member IDs: %v", ids)
		}
	})

	t.Run("three identical transactions form one group of three", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "Netflix", -1599, day),
			makeTx("b", "acc-1", "Netflix", -1599, day),
			makeTx("c", "acc-1", "Netflix", -1599, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0]) != 3 {
			t.Errorf("expected 3 members in group, got %d", len(groups[0]))
		}
	})

	t.Run("same payee/amount/date on different accounts are NOT duplicates", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "Spotify", -999, day),
			makeTx("b", "acc-2", "Spotify", -999, day), // different account
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 0 {
			t.Errorf("cross-account same charge should not be grouped, got %d group(s)", len(groups))
		}
	})

	t.Run("different amounts are not duplicates", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "Amazon", -5000, day),
			makeTx("b", "acc-1", "Amazon", -5001, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 0 {
			t.Errorf("different amounts should not be grouped, got %d group(s)", len(groups))
		}
	})

	t.Run("different dates are not duplicates", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "Gym", -4000, day),
			makeTx("b", "acc-1", "Gym", -4000, day.AddDate(0, 0, 1)),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 0 {
			t.Errorf("different dates should not be grouped, got %d group(s)", len(groups))
		}
	})

	t.Run("group order is deterministic by first member's input position", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			// First duplicate pair (positions 0, 1)
			makeTx("a1", "acc-1", "Uber", -2500, day),
			makeTx("a2", "acc-1", "Uber", -2500, day),
			// Second duplicate pair (positions 2, 3)
			makeTx("b1", "acc-1", "Lyft", -2200, day),
			makeTx("b2", "acc-1", "Lyft", -2200, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 2 {
			t.Fatalf("expected 2 groups, got %d", len(groups))
		}
		// First group should be Uber (appears first in input).
		if groups[0][0].Payee != "Uber" {
			t.Errorf("first group should be Uber, got %q", groups[0][0].Payee)
		}
		if groups[1][0].Payee != "Lyft" {
			t.Errorf("second group should be Lyft, got %q", groups[1][0].Payee)
		}
	})

	t.Run("POS-prefixed payee matches clean payee", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("a", "acc-1", "# STARBUCKS", -4200, day),
			makeTx("b", "acc-1", "STARBUCKS", -4200, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 1 {
			t.Fatalf("POS-prefixed and clean payee should be grouped; got %d group(s)", len(groups))
		}
	})

	t.Run("input order preserved within each group", func(t *testing.T) {
		t.Parallel()
		txns := []domain.Transaction{
			makeTx("first", "acc-1", "Costco", -8000, day),
			makeTx("second", "acc-1", "Costco", -8000, day),
			makeTx("third", "acc-1", "Costco", -8000, day),
		}
		groups := fingerprint.GroupDuplicates(txns)
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		wantOrder := []string{"first", "second", "third"}
		for i, tx := range groups[0] {
			if tx.ID != wantOrder[i] {
				t.Errorf("group[0][%d].ID = %q, want %q", i, tx.ID, wantOrder[i])
			}
		}
	})
}

// ── MergeResolve ─────────────────────────────────────────────────────────────

func TestMergeResolve(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	dayBefore := day.AddDate(0, 0, -1)

	base := domain.Transaction{
		ID:        "tx-a",
		AccountID: "acc-1",
		Date:      day,
		Payee:     "Starbucks",
		Desc:      "SBX",
		Amount:    money.New(-4200, "USD"),
		Cleared:   false,
	}

	t.Run("a's ID is always kept", func(t *testing.T) {
		t.Parallel()
		b := base
		b.ID = "tx-b"
		merged := fingerprint.MergeResolve(base, b)
		if merged.ID != "tx-a" {
			t.Errorf("merged.ID = %q, want %q", merged.ID, "tx-a")
		}
	})

	t.Run("earlier date is kept", func(t *testing.T) {
		t.Parallel()
		b := base
		b.Date = dayBefore
		merged := fingerprint.MergeResolve(base, b)
		if !merged.Date.Equal(dayBefore) {
			t.Errorf("merged.Date = %v, want %v (earlier)", merged.Date, dayBefore)
		}
	})

	t.Run("later date in a not overridden when a is earlier", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Date = dayBefore
		merged := fingerprint.MergeResolve(a, base) // a is earlier
		if !merged.Date.Equal(dayBefore) {
			t.Errorf("merged.Date = %v, want %v (earlier)", merged.Date, dayBefore)
		}
	})

	t.Run("non-empty payee from b wins when a is empty", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Payee = ""
		b := base
		b.Payee = "Starbucks Corp"
		merged := fingerprint.MergeResolve(a, b)
		if merged.Payee != "Starbucks Corp" {
			t.Errorf("merged.Payee = %q, want %q", merged.Payee, "Starbucks Corp")
		}
	})

	t.Run("a's payee preserved when both set", func(t *testing.T) {
		t.Parallel()
		b := base
		b.Payee = "Starbucks Corp"
		merged := fingerprint.MergeResolve(base, b) // a.Payee = "Starbucks"
		if merged.Payee != "Starbucks" {
			t.Errorf("merged.Payee = %q, want a's value %q", merged.Payee, "Starbucks")
		}
	})

	t.Run("longer Desc wins", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Desc = "short"
		b := base
		b.Desc = "much longer raw bank narrative"
		merged := fingerprint.MergeResolve(a, b)
		if merged.Desc != "much longer raw bank narrative" {
			t.Errorf("merged.Desc = %q, want longer value", merged.Desc)
		}
	})

	t.Run("a's Desc kept when both same length", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Desc = "abc"
		b := base
		b.Desc = "xyz"
		merged := fingerprint.MergeResolve(a, b)
		if merged.Desc != "abc" {
			t.Errorf("merged.Desc = %q, want a's value %q", merged.Desc, "abc")
		}
	})

	t.Run("set CategoryID from b wins when a is empty", func(t *testing.T) {
		t.Parallel()
		a := base
		a.CategoryID = ""
		b := base
		b.CategoryID = "cat-food"
		merged := fingerprint.MergeResolve(a, b)
		if merged.CategoryID != "cat-food" {
			t.Errorf("merged.CategoryID = %q, want %q", merged.CategoryID, "cat-food")
		}
	})

	t.Run("a's CategoryID preserved when both set", func(t *testing.T) {
		t.Parallel()
		a := base
		a.CategoryID = "cat-coffee"
		b := base
		b.CategoryID = "cat-food"
		merged := fingerprint.MergeResolve(a, b)
		if merged.CategoryID != "cat-coffee" {
			t.Errorf("merged.CategoryID = %q, want a's value %q", merged.CategoryID, "cat-coffee")
		}
	})

	t.Run("Cleared true wins", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Cleared = false
		b := base
		b.Cleared = true
		merged := fingerprint.MergeResolve(a, b)
		if !merged.Cleared {
			t.Error("merged.Cleared should be true when b is cleared")
		}
	})

	t.Run("Cleared false preserved when neither cleared", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Cleared = false
		b := base
		b.Cleared = false
		merged := fingerprint.MergeResolve(a, b)
		if merged.Cleared {
			t.Error("merged.Cleared should be false when neither cleared")
		}
	})

	t.Run("Tags are unioned and deduplicated", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Tags = []string{"import", "coffee"}
		b := base
		b.Tags = []string{"coffee", "recurring"}
		merged := fingerprint.MergeResolve(a, b)
		want := []string{"coffee", "import", "recurring"}
		if len(merged.Tags) != len(want) {
			t.Fatalf("merged.Tags = %v, want %v", merged.Tags, want)
		}
		for i, tag := range merged.Tags {
			if tag != want[i] {
				t.Errorf("merged.Tags[%d] = %q, want %q", i, tag, want[i])
			}
		}
	})

	t.Run("nil Tags from both stays nil", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Tags = nil
		b := base
		b.Tags = nil
		merged := fingerprint.MergeResolve(a, b)
		if merged.Tags != nil {
			t.Errorf("merged.Tags should be nil, got %v", merged.Tags)
		}
	})

	t.Run("Reviewed true wins", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Reviewed = false
		b := base
		b.Reviewed = true
		merged := fingerprint.MergeResolve(a, b)
		if !merged.Reviewed {
			t.Error("merged.Reviewed should be true when b is reviewed")
		}
	})

	t.Run("Attachments unioned by ArtifactID", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Attachments = []domain.AttachmentRef{
			{ArtifactID: "art-1", Name: "receipt.jpg"},
		}
		b := base
		b.Attachments = []domain.AttachmentRef{
			{ArtifactID: "art-1", Name: "receipt.jpg"}, // duplicate
			{ArtifactID: "art-2", Name: "statement.pdf"},
		}
		merged := fingerprint.MergeResolve(a, b)
		if len(merged.Attachments) != 2 {
			t.Errorf("merged.Attachments len = %d, want 2 (deduped by ArtifactID); got %v",
				len(merged.Attachments), merged.Attachments)
		}
	})

	t.Run("Custom map unioned with a winning on conflicts", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Custom = map[string]any{"src": "manual", "note": "from a"}
		b := base
		b.Custom = map[string]any{"src": "csv", "extra": "only in b"}
		merged := fingerprint.MergeResolve(a, b)
		if merged.Custom["src"] != "manual" {
			t.Errorf("merged.Custom[src] = %v, want %q (a wins)", merged.Custom["src"], "manual")
		}
		if merged.Custom["extra"] != "only in b" {
			t.Errorf("merged.Custom[extra] = %v, want %q", merged.Custom["extra"], "only in b")
		}
		if merged.Custom["note"] != "from a" {
			t.Errorf("merged.Custom[note] = %v, want %q", merged.Custom["note"], "from a")
		}
	})

	t.Run("both nil Custom stays nil", func(t *testing.T) {
		t.Parallel()
		a := base
		a.Custom = nil
		b := base
		b.Custom = nil
		merged := fingerprint.MergeResolve(a, b)
		if merged.Custom != nil {
			t.Errorf("merged.Custom should be nil, got %v", merged.Custom)
		}
	})
}
