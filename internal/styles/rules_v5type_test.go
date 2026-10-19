// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTransactionsTypeScale(t *testing.T) {
	resetSheet()
	registerTxnAudit()
	registerTxnCalendar()
	registerTxnEditSurface()
	registerTxnTemplatesSurface()
	registerTxnToolbar()
	registerTxnUpcoming()
	registerTxnScope()
	registerAcctxn()
	registerR10W()
	registerV5Type()

	fontSizes := map[string]string{
		"var(--fs-meta)":   "12px",
		"var(--fs-body)":   "14px",
		"var(--fs-figure)": "17px",
		"var(--fs-title)":  "24px",
	}
	allowedSizes := map[string]bool{"12px": true, "14px": true, "17px": true, "24px": true}
	allowedWeights := map[string]bool{"400": true, "500": true, "600": true}
	matched := 0
	for _, r := range sheet {
		for _, d := range r.decls {
			switch d.prop {
			case "font-size":
				matched++
				actual := d.value
				if resolved, ok := fontSizes[actual]; ok {
					actual = resolved
				}
				if !allowedSizes[actual] {
					t.Errorf("Transactions selector %q declares font-size %q; use the 12/14/17/24px scale", r.selector, d.value)
				}
			case "font-weight":
				matched++
				if !allowedWeights[d.value] {
					t.Errorf("Transactions selector %q declares font-weight %q; use 400, 500, or 600", r.selector, d.value)
				}
			}
		}
	}
	if matched == 0 {
		t.Fatal("no Transactions typography declarations were collected")
	}
}

func TestV5TypePrimaryButtonIsFlatAccent(t *testing.T) {
	resetSheet()
	registerV5Type()
	css := Build()
	if !strings.Contains(css, ".btn-primary{background:var(--accent);border-color:var(--accent);color:var(--primary-button-ink);box-shadow:none;}") {
		t.Fatalf("primary button should use a flat accent fill and contrast-aware ink; got %s", css)
	}
}
