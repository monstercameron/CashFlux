// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBV6AResponsiveBudgetRowsStayWithinTheirCard(t *testing.T) {
	resetSheet()
	registerBR3Row()
	registerBR2MOB()
	registerBV6A()
	css := Build()
	for _, want := range []string{
		"grid-template-columns:minmax(180px, 1fr) max-content minmax(160px, 1.2fr) 96px 72px;",
		"@media (min-width: 601px) and (max-width: 1200px)",
		"grid-template-columns:minmax(0, 1fr) minmax(0, max-content) 72px;",
		`grid-template-areas:"name figures actions" "bar bar status" "sub sub sub";`,
		".budget-crow-bar{grid-column:1 / 3;width:100%;",
		"@media (max-width: 600px)",
		`grid-template-areas:"name status actions" "figures figures figures" "bar bar bar" "sub sub sub";`,
		".budget-crow-actions button, .budget-crow-actions a{min-width:32px;min-height:32px;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV6A responsive row CSS is missing %q", want)
		}
	}
}

func TestBV6AFiguresAndCompactNotesCannotOverlap(t *testing.T) {
	resetSheet()
	registerNotesMarkerWrap()
	registerBR3Row()
	registerBR2MOB()
	registerBV6A()
	css := Build()
	for _, want := range []string{
		".budget-crow-head{min-width:0;overflow:hidden;",
		".budget-crow-figures{min-width:0;max-width:100%;justify-content:flex-end;flex-wrap:wrap;",
		".budget-crow-fig-separator{display:none;}",
		".budget-crow-notes .budget-crow-notes-text{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;",
		".budget-crow-notes-wrap .budget-crow-notes{flex:1 1 auto;min-width:0;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV6A figure/note CSS is missing %q", want)
		}
	}
	oldFixed := strings.LastIndex(css, ".budget-crow-figures{min-width:240px;white-space:nowrap;")
	newFlexible := strings.LastIndex(css, ".budget-crow-figures{min-width:0;max-width:100%;")
	if oldFixed >= 0 && newFlexible <= oldFixed {
		t.Fatal("flexible figure rule must follow the old fixed-width rule")
	}
}

func TestBV6ABarTrackAndRolloverTableRemainLegible(t *testing.T) {
	resetSheet()
	registerBR3Row()
	registerBV6A()
	css := Build()
	for _, want := range []string{
		".budget-crow-bar{min-width:0;height:6px;background:color-mix(in srgb, var(--text) 22%, var(--bg-card));",
		".budget-crow-bar .bar-pace-tick{height:12px;top:-3px;width:2px;",
		".budget-rollover-detail table{width:auto;max-width:100%;margin-left:0;",
		".budget-rollover-detail{max-width:560px;min-width:0;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV6A bar/table CSS is missing %q", want)
		}
	}
}

func TestBV6AInlineLimitEditorUsesOneAlignedBaseline(t *testing.T) {
	resetSheet()
	registerBV6A()
	css := Build()
	for _, want := range []string{
		".budget-limit-editor{display:grid;grid-template-columns:minmax(0, 1fr) auto minmax(180px, 240px) auto auto;align-items:center;",
		".budget-limit-editor-base{display:flex;align-items:center;gap:0.5rem;min-width:0;}",
		".budget-limit-editor button{min-width:32px;min-height:32px;}",
		"@media (max-width: 600px){.budget-limit-editor{grid-template-columns:minmax(0, 1fr) auto;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV6A limit editor CSS is missing %q", want)
		}
	}
}
