// SPDX-License-Identifier: MIT

package screenlint

import (
	"go/parser"
	"go/printer"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ─── C341 net-worth-change ratchets ──────────────────────────────────────────
//
// The V-sweep found the same question — "how far has net worth moved this
// month?" — answered three different ways in the first viewport of three money
// pages: the dashboard hero said ▲ $2,840.00, /accounts said "No change this
// month", /reports and /networth said ▲ $1,350.43 (they were quietly reporting
// LAST month's step under a "this period" label). Each surface had hand-rolled
// its own two-cutoff read.
//
// internal/nwchange is now the one definition and screens.nwChangeSub the one
// sentence. These guards fail if a fifth definition grows back.

// mtdDeltaHandRoll matches the shape of the bug: a NetWorthSeries read whose
// cutoff list is built from a month start — i.e. someone deriving the
// month-to-date delta themselves instead of asking nwchange for it.
var mtdDeltaHandRoll = regexp.MustCompile(`NetWorthSeries\([^)]*\[\]time\.Time\{[^}]*MonthStart`)

// nwSentenceHandRoll matches a net-worth-change sentence being ASSEMBLED — an
// arrow glyph and the words "this month" inside one string literal, one
// Sprintf format, or one concatenation chain. The four surfaces that disagreed
// each built their own. Comments are stripped before scanning, so a doc comment
// may still quote the shape it is describing.
var nwSentenceHandRoll = regexp.MustCompile(
	`"[▲▼■][^"\n]*this month|"[▲▼■][^"\n]*"\s*\+[^\n]*"[^"\n]*this month`)

// codeOnly returns the source with every comment removed, so a guard scanning
// for a code shape is not tripped by prose describing that shape (this file's
// own header, and the doc comments on the surfaces that were fixed, both quote
// the sentence they exist to talk about). Falls back to the raw source if the
// file does not parse — a parse failure is someone else's test to fail.
func codeOnly(t *testing.T, path, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0) // no ParseComments → comments dropped
	if err != nil {
		return src
	}
	var buf strings.Builder
	if err := printer.Fprint(&buf, fset, f); err != nil {
		return src
	}
	return buf.String()
}

// nwchangeOwners are the files allowed to compute a month-to-date net-worth
// delta from cutoffs directly: the seam itself, and nothing else.
var nwchangeOwners = map[string]bool{
	"nwchange/nwchange.go": true,
}

// TestMonthToDateNetWorthDeltaHasOneDefinition keeps the arithmetic singular.
func TestMonthToDateNetWorthDeltaHasOneDefinition(t *testing.T) {
	files := readInternal(t)
	var offenders []string
	for path, src := range files {
		if nwchangeOwners[path] {
			continue
		}
		if mtdDeltaHandRoll.MatchString(src) {
			offenders = append(offenders, path)
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("these files derive a month-to-date net-worth delta from raw cutoffs instead of "+
			"nwchange.MonthToDateChange, which is how four surfaces came to disagree (C341):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestNetWorthChangeSentenceHasOneAuthor keeps the wording singular.
//
// Four surfaces each Sprintf'd their own "▲ %d%% (+%s) this month" line, so the
// arrow, the parentheses, the sign and the window name could all drift apart —
// and three of them baked the English in, unreachable by the language setting.
// The sentence now lives in screens.nwChangeSub over i18n keys.
func TestNetWorthChangeSentenceHasOneAuthor(t *testing.T) {
	files := readInternal(t)
	var offenders []string
	for path, src := range files {
		if path == "screens/networth_change.go" || strings.HasPrefix(path, "i18n/") {
			continue
		}
		if nwSentenceHandRoll.MatchString(codeOnly(t, path, src)) {
			offenders = append(offenders, path)
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("these files build a net-worth change sentence themselves instead of calling "+
			"nwChangeSub, so the wording and the window label can drift (C341):\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestNwChangeSeamIsActuallyUsed stops the guards above from passing vacuously:
// if the surfaces stopped importing the seam, the regexes would find nothing to
// complain about and the ratchet would silently protect nothing.
func TestNwChangeSeamIsActuallyUsed(t *testing.T) {
	files := readInternal(t)
	want := []string{
		"screens/dashboard.go",
		"screens/dashboard_hero.go",
		"screens/accounts_tiles.go",
		"screens/networth.go",
		"screens/widget_builder.go",
		"engineenv/networthvars.go",
	}
	for _, path := range want {
		src, ok := files[path]
		if !ok {
			t.Errorf("%s not found — has it moved? the C341 guard needs updating", path)
			continue
		}
		if !strings.Contains(src, "nwchange.") {
			t.Errorf("%s no longer reads net-worth change through internal/nwchange (C341)", path)
		}
	}
}

// ─── C342/C343 window-stamp ratchets ─────────────────────────────────────────

// TestBadgeMapsAreDisjointAndReal keeps the dashboard's window story coherent.
//
// C343: a reader could not tell a "this month" figure from a "this period" one
// from a trailing-three-month one while all three sat on the same screen. Tiles
// now fall into exactly two camps — current-state tiles that wear a "Today" chip
// when the dashboard is paged away, and windowed tiles that always wear the
// selected period. A tile in both maps would claim to be both; a tile in neither
// is a figure with no window at all, which is the bug.
func TestBadgeMapsAreDisjointAndReal(t *testing.T) {
	src, ok := readInternal(t)["ui/widget.go"]
	if !ok {
		t.Fatal("ui/widget.go not found")
	}
	today := badgeMapIDs(t, src, "todayBadgeWidgets")
	windowed := badgeMapIDs(t, src, "periodWindowedWidgets")
	if len(today) == 0 || len(windowed) == 0 {
		t.Fatal("one of the badge maps came back empty — this guard would pass vacuously")
	}
	for id := range windowed {
		if today[id] {
			t.Errorf("tile %q is in BOTH badge maps: it cannot be current-state and "+
				"period-windowed at once (C343)", id)
		}
	}

	// Every windowed id must be a tile that actually exists, or the chip silently
	// never renders and the ticket reads as done while nothing was stamped.
	dash := readInternal(t)
	var declared strings.Builder
	for path, text := range dash {
		if strings.HasPrefix(path, "screens/dashboard") {
			declared.WriteString(text)
		}
	}
	for id := range windowed {
		if !strings.Contains(declared.String(), `ID: "`+id+`"`) &&
			!strings.Contains(declared.String(), `"`+id+`":`) {
			t.Errorf("periodWindowedWidgets names %q, which no dashboard tile declares — "+
				"its period chip would never render (C343)", id)
		}
	}
}

// badgeMapIDs pulls the quoted keys out of one `var name = map[string]bool{…}`.
func badgeMapIDs(t *testing.T, src, name string) map[string]bool {
	t.Helper()
	i := strings.Index(src, "var "+name+" = map[string]bool{")
	if i < 0 {
		t.Fatalf("%s has moved or been renamed; update this guard rather than deleting it", name)
	}
	rest := src[i:]
	end := strings.Index(rest, "\n}")
	if end < 0 {
		t.Fatalf("could not delimit the %s map literal", name)
	}
	out := map[string]bool{}
	for _, m := range regexp.MustCompile(`"([a-z0-9-]+)":`).FindAllStringSubmatch(rest[:end], -1) {
		out[m[1]] = true
	}
	return out
}

// TestHealthFactorsRenderTheirWindow keeps C342 closed.
//
// The dashboard's 60% savings rate (selected period) and /health's 31% (trailing
// three full months) were both right and neither said so. The model now carries
// the window; this fails if the view stops showing it, or goes back to naming
// only the savings factor.
func TestHealthFactorsRenderTheirWindow(t *testing.T) {
	src, ok := readInternal(t)["screens/health.go"]
	if !ok {
		t.Fatal("screens/health.go not found")
	}
	if !strings.Contains(src, "healthWindowLabel(f.Window)") {
		t.Error("the health factor tile no longer renders its measurement window (C342)")
	}
	if strings.Contains(src, `f.Key == "savings"`) {
		t.Error("the window caption is gated on the savings factor again — every factor " +
			"carries a Window now, and a value shown without one is what made 31% and 60% " +
			"read as a contradiction (C342)")
	}
}
