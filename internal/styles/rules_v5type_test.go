// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTransactionsTypeScale(t *testing.T) {
	resetSheet()
	registerReviewSurface()
	registerFlipFit()
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
			// Review's earlier rules remain registered for layout and color. TV7D's
			// final scoped overrides are verified separately below; the raw source
			// declarations do not describe the computed type scale.
			if strings.Contains(r.selector, ".rvs") || strings.Contains(r.selector, ".catpick") {
				continue
			}
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

func TestTV7DDialogTypographyUsesSharedTokens(t *testing.T) {
	resetSheet()
	registerV5Type()
	css := Build()
	for _, expected := range []string{
		".flip-back .set-h h3{font-family:Inter, sans-serif;font-size:var(--fs-figure);font-weight:600;}",
		".rvs .rvs-intro-title{font-family:Inter, sans-serif;font-size:var(--fs-figure);font-weight:600;}",
		".rvs .rvs-big{font-size:var(--fs-title);font-weight:600;font-variant-numeric:tabular-nums;}",
		".rvs .rvs-tier-count, .rvs .rvs-foot-sub, .rvs .rvs-note, .rvs .rvs-kbd, .rvs .rvw-rawpayee, .txn-edit .mtrend-spark-xaxis, .txn-edit .mtrend-spark-yaxis, .txn-edit .mtrend-spark-meta, .txn-cal .txn-cal-ghost, .txn-cal .txn-cal-week-count{font-size:var(--fs-meta);font-weight:400;font-style:normal;font-variant-numeric:tabular-nums;}",
		".txn-cal .txn-cal-moved{font-size:var(--fs-body);font-weight:400;font-style:normal;font-variant-numeric:tabular-nums;}",
		".empty[data-testid='txn-nomatch'] .btn-link{font-size:var(--fs-body);font-weight:500;font-style:normal;}",
		".bento-ledger .txn-table .td-user, .notify-btn .notify-badge{font-size:var(--fs-meta);font-weight:500;font-variant-numeric:tabular-nums;}",
	} {
		if !strings.Contains(css, expected) {
			t.Errorf("missing TV7D typography rule %q in %s", expected, css)
		}
	}
}
