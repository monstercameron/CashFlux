// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestUXT5RowPopoverAndReducedMotionRules(t *testing.T) {
	resetSheet()
	registerUXT5()
	css := Build()
	for _, want := range []string{
		".txn-row-info{width:24px;height:24px;flex:0 0 24px;",
		".txn-row-info-text{display:block;flex:1 0 100%;max-width:100%;margin-top:.15rem;font-size:12px;line-height:1.3;font-weight:400;color:var(--text-dim);}",
		".txn-table td.row-desc-cell .row-desc-inner{flex-direction:column;align-items:stretch;gap:0;}",
		".txn-table td.row-desc-cell .txn-desc-tags{align-self:flex-start;max-width:100%;width:100%;height:19px;flex:0 0 19px;}",
		".txn-transfer-category{display:block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}",
		".txn-upcoming-popover{position:absolute;top:calc(100% + 6px);left:0;",
		".txn-row-flash{animation:txn-row-flash 150ms ease-out;}",
		"@media (prefers-reduced-motion: reduce){.txn-row-flash{animation:none;}}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("UXT5 CSS missing %q", want)
		}
	}
}
