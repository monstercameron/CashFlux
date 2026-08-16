// SPDX-License-Identifier: MIT

package screenlint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── C560–C572 Transactions-audit ratchets ───────────────────────────────────
//
// The browser tests in e2e/regression/txn_audit.spec.mjs prove these behaviours
// hold TODAY. The guards here are about tomorrow: each one blocks the specific
// shape of code that reintroduced the bug, in a place a future author would not
// think to look. They are cheap source scans, not behaviour tests, and every one
// names the failure it exists to prevent.

// readInternal returns every non-test .go file under internal/, keyed by its
// slash-separated path relative to internal/.
//
// It walks the tree itself rather than reusing a sibling ratchet's helpers: these
// guards must keep working if a neighbouring lint file is moved or retired, and a
// shared helper is a dependency between two files that have nothing else to do
// with each other.
func readInternal(t *testing.T) map[string]string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Join(wd, "..")
	out := map[string]string{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %q: %v", root, err)
	}
	if len(out) == 0 {
		t.Fatalf("no Go files found under %q — the guards below would pass vacuously", root)
	}
	return out
}

// TestTransactionsHasOnePeriodState keeps the ledger's date scope singular.
//
// C560: the page carried three — the top bar's period.Window, the ledger's
// TxFilter.From/To, and a private calendar-month atom. They could not be
// reconciled (a Window cannot express a single day or a hand-typed range), so
// the period label sat over rows from a different month and stepping it moved
// nothing. A second store is the bug; this fails if one comes back.
func TestTransactionsHasOnePeriodState(t *testing.T) {
	files := readInternal(t)
	for rel, text := range files {
		if strings.Contains(text, "UseTxnCalMonth") && !strings.Contains(text, "Removed (C560)") {
			t.Errorf("%s references a private calendar-month atom.\n"+
				"The calendar's visible month is DERIVED from the ledger's own From/To via "+
				"internal/txnscope. A second period store cannot be kept in agreement with the "+
				"rows — that is exactly the C560 defect.", rel)
		}
	}
}

// TestTransactionsIsNotPeriodAware keeps the top bar's reporting pill off the
// ledger. The pill snaps to whole units, so on a page whose scope can be one day
// it can only ever display something the rows contradict (C560).
func TestTransactionsIsNotPeriodAware(t *testing.T) {
	shell, ok := readInternal(t)["app/shell.go"]
	if !ok {
		t.Fatal("app/shell.go not found")
	}
	i := strings.Index(shell, "periodAware := map[string]bool{")
	if i < 0 {
		t.Fatal("the periodAware map has moved; update this guard rather than deleting it")
	}
	end := strings.Index(shell[i:], "}[curPath]")
	if end < 0 {
		t.Fatal("could not delimit the periodAware map literal")
	}
	if strings.Contains(shell[i:i+end], `"/transactions"`) {
		t.Error("/transactions is period-aware again.\n" +
			"The top bar's pill stores a snapped period.Window and cannot represent the ledger's " +
			"date scope (a calendar day-click, a hand-typed range), so the two can never agree. " +
			"The ledger owns its own period bar — screens.txnScopeBar (C560).")
	}
}

// TestCategoryPickersUseQualifiedPaths keeps every picker naming the full path.
//
// C570: quick-add, bulk categorize and the split editor listed bare leaf names
// while Review and Edit showed "Housing > Mortgage", so two categories called
// "Gas" were indistinguishable at the moments a charge actually gets filed. The
// shape that produced it is building an option label straight off `c.Name`.
func TestCategoryPickersUseQualifiedPaths(t *testing.T) {
	// The leaf-name option shapes seen in the wild, paired with the files that
	// are allowed to use them. A picker that shows ONE category (an entity's own
	// name, a tree row that already renders its parent) is not a flat picker.
	banned := []string{
		"SelectedIf(catID.Get() == c.ID), c.Name",
		"SelectedIf(catSel == c.ID), c.Name",
		"SelectedIf(bulkCat.Get() == c.ID), c.Name",
		"SelectedIf(f.Category == c.ID), c.Name",
	}
	// transactions.go is the retired legacy ledger, kept only for reference; it
	// renders nothing. Everything live must qualify.
	allowed := map[string]bool{"screens/transactions.go": true}
	var offenders []string
	for rel, text := range readInternal(t) {
		if allowed[rel] {
			continue
		}
		for _, b := range banned {
			if strings.Contains(text, b) {
				offenders = append(offenders, rel+": "+b)
			}
		}
	}
	if len(offenders) > 0 {
		t.Errorf("flat category picker(s) listing bare leaf names: %v.\n"+
			"Use screens.CategoryPickOptions (catname.Path + natural order) so duplicate leaf "+
			"names are distinguishable everywhere a charge gets filed (C570).", offenders)
	}
}

// TestDestructiveRowActionsConfirmOrUndo keeps the two row actions that change
// more than the row they sit on behind a confirmation.
//
// C562: excluding a transaction from reports changed nothing visible about the
// row while rewriting every budget and report it fed, and it fired on the first
// click. The guard is that its handler still reaches a confirm.
func TestExcludeFromReportsConfirms(t *testing.T) {
	text, ok := readInternal(t)["screens/transactions_widget.go"]
	if !ok {
		t.Fatal("screens/transactions_widget.go not found")
	}
	i := strings.Index(text, "applyExclude := func(")
	if i < 0 {
		i = strings.Index(text, "toggleExclude := func(")
	}
	if i < 0 {
		t.Fatal("the exclude handler has moved; update this guard rather than deleting it")
	}
	// Scan from the first exclude handler through the end of the toggle that calls
	// it — the confirmation and the undo capture live in the pair, not in one func,
	// so bounding to a single body would fail on a correct implementation.
	body := text[i:]
	if j := strings.Index(body, "\n\tdeleteRow := "); j > 0 {
		body = body[:j]
	}
	if !strings.Contains(body, "ConfirmModal") {
		t.Error("excluding a transaction from reports no longer confirms.\n" +
			"It leaves the row's own figures untouched while changing budgets, spending and " +
			"reports, so an accidental click is invisible until a total quietly stops adding " +
			"up (C562).")
	}
	if !strings.Contains(body, "postUndoStory") {
		t.Error("excluding/including no longer captures an undo point.\n" +
			"Both directions must be reversible from Ctrl+Z / Activity (C562).")
	}
}

// TestDuplicateReviewDoesNotClaimIrreversibility keeps the duplicate-review copy
// honest.
//
// C571: the delete confirmation read "This can't be undone" while the handler
// beneath it captured an undo point and posted an undoable toast. A confirmation
// that overstates the risk teaches people that confirmations are noise.
func TestDuplicateReviewDoesNotClaimIrreversibility(t *testing.T) {
	for rel, text := range readInternal(t) {
		if !strings.HasPrefix(rel, "i18n/") {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			if !strings.Contains(line, "duplicates.") || !strings.Contains(line, "onfirm") {
				continue
			}
			if strings.Contains(line, "can't be undone") || strings.Contains(line, "cannot be undone") {
				t.Errorf("%s: a duplicate-review confirmation claims the action is irreversible:\n  %s\n"+
					"Both merge and delete capture an undo point and post an undoable toast, so the "+
					"claim is false and it trains users to click through confirmations (C571).",
					rel, strings.TrimSpace(line))
			}
		}
	}
}

// TestSplitDraftVerdictIsSharedNotReimplemented keeps the split editor's footer
// text and its Save button reading ONE verdict.
//
// C566: they were computed separately, so the footer said "Balanced" beside an
// enabled Save on a draft that could only fail validation. The rule now lives in
// internal/split (pure, tested); the editor must consume it rather than
// recomputing a second opinion in view code.
func TestSplitDraftVerdictIsSharedNotReimplemented(t *testing.T) {
	text, ok := readInternal(t)["screens/split_editor.go"]
	if !ok {
		t.Fatal("screens/split_editor.go not found")
	}
	if !strings.Contains(text, "split.Classify(") || !strings.Contains(text, ".Saveable(") {
		t.Error("the split editor no longer reads its verdict from internal/split.\n" +
			"Classify/Saveable is what keeps the remainder line and the Save button from " +
			"disagreeing about whether the draft is a valid split (C566).")
	}
}

// TestRowSelectorIsQualifiedByTr keeps the ledger's row selector meaning ROWS.
//
// `[data-testid^="txn-row-"]` is how the whole suite picks a transaction row —
// but several per-row CHILD controls share that prefix (tags, note, receipt), so
// the bare prefix matches rows AND their contents interleaved in document order.
// `.nth(6)` then stops meaning "the seventh row" and `.toHaveCount(1)` counts a
// row plus its own glyphs. It bit for real when C563 put an Edit control on every
// row: 25 rows, 58 matches, and splits.spec.mjs timed out waiting for a kebab
// inside what it thought was a row.
//
// The prefix is load-bearing in the committed coverage manifest and in scratch
// scripts, so the fix is not to rename the children — it is to require the `tr`
// qualifier at every call site, which is correct regardless of what ids exist.
func TestRowSelectorIsQualifiedByTr(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := filepath.Join(wd, "..", "..", "e2e", "regression")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no e2e/regression directory (%v) — nothing to guard", err)
	}
	var offenders []string
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".spec.mjs") {
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, e.Name()))
		if rerr != nil {
			t.Fatalf("read %q: %v", e.Name(), rerr)
		}
		checked++
		for i, line := range strings.Split(string(data), "\n") {
			// The bare prefix, not preceded by the `tr` element qualifier.
			for _, quote := range []string{`'[data-testid^="txn-row-"]`, "`[data-testid^=\"txn-row-\"]"} {
				if strings.Contains(line, quote) {
					offenders = append(offenders, fmt.Sprintf("%s:%d", e.Name(), i+1))
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no spec files scanned — this guard would pass vacuously")
	}
	if len(offenders) > 0 {
		t.Errorf("unqualified ledger-row selector at %v.\n"+
			"Use tr[data-testid^=\"txn-row-\"]. The bare prefix also matches per-row child "+
			"controls (tags, note, receipt), so nth(i) and toHaveCount(n) silently stop "+
			"counting rows.", offenders)
	}
}

// TestLedgerPeriodLabelIsDerived keeps the period bar's label a FUNCTION of the
// filter rather than a second copy of it. A stored label is a state that can
// drift; a derived one cannot (C560).
func TestLedgerPeriodLabelIsDerived(t *testing.T) {
	text, ok := readInternal(t)["screens/transactions_scopebar.go"]
	if !ok {
		t.Fatal("screens/transactions_scopebar.go not found")
	}
	if !strings.Contains(text, "txnscope.Of(f.From, f.To") {
		t.Error("the ledger's period label is no longer derived from the filter's own dates.\n" +
			"txnscope.Of(From, To, now) is what makes the label and the rows the same fact " +
			"rendered twice instead of two states hoping to agree (C560).")
	}
	cal, ok := readInternal(t)["screens/transactions_calendar.go"]
	if !ok {
		t.Fatal("screens/transactions_calendar.go not found")
	}
	if !strings.Contains(cal, "txnscope.Of(f.From, f.To") {
		t.Error("the calendar's visible month is no longer derived from the ledger's dates (C560).")
	}
}
