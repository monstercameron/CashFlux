// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestIMV7ImportVisualSystem(t *testing.T) {
	resetSheet()
	registerImportWizard()
	registerTierSystem()
	registerIMV7()
	css := Build()
	for _, want := range []string{
		".doc-picker, .doc-picker *, .doc-form, .doc-form *, .card-step-active, .card-step-active *{font-family:Inter, sans-serif;font-size:var(--fs-body);font-weight:400;font-style:normal;letter-spacing:normal;text-transform:none;}",
		".doc-form button, .card-step-active button{font-weight:500;}",
		".doc-type-grid{display:grid;grid-template-columns:repeat(3, minmax(0, 1fr));align-items:stretch;gap:0.75rem;}",
		".doc-type-tile{box-sizing:border-box;min-width:0;min-height:10rem;height:100%;align-items:flex-start;}",
		".doc-type-tile .dt-title, .doc-branch-title{font-size:var(--fs-figure);font-weight:600;letter-spacing:normal;}",
		".doc-picker .tier-chip, .doc-form .tier-chip{font-size:var(--fs-meta);font-weight:500;letter-spacing:normal;text-transform:none;}",
		".doc-form .btn-plus, [data-testid='suggest-categories-ai']{color:var(--tier-smartplus);background:transparent;border:1px solid var(--tier-smartplus);box-shadow:none;font-size:var(--fs-body);font-weight:500;}",
		".doc-form .btn-plus::before, [data-testid='suggest-categories-ai']::before{content:\"Smart+\";font-size:var(--fs-meta);font-weight:600;margin-right:0.25rem;}",
		".doc-form .btn-plus .key-gate-mark, [data-testid='suggest-categories-ai'] .key-gate-mark{font-size:var(--fs-meta);font-weight:500;color:var(--tier-smartplus);}",
		"[data-theme=\"light\"] .doc-branch-title.smart, [data-theme=\"light\"] .doc-picker .tier-chip.smart, [data-theme=\"light\"] .doc-type-tile.smart .tier-icon{color:#286b43;}",
		"[data-theme=\"light\"] .doc-branch-title.plus, [data-theme=\"light\"] .doc-picker .tier-chip.plus, [data-theme=\"light\"] .doc-type-tile.smartplus .tier-icon{color:#4d55b8;}",
		"[data-theme=\"light\"] .doc-type-tile.smart:hover{border-color:#286b43;}",
		"[data-theme=\"light\"] .doc-type-tile.smartplus:hover{border-color:#4d55b8;}",
		"[data-theme=\"dark\"] .doc-branch-title.smart, [data-theme=\"dark\"] .doc-picker .tier-chip.smart{color:#71d99a;}",
		"[data-theme=\"dark\"] .doc-type-tile.smart .tier-icon{color:#71d99a;}",
		"[data-theme=\"dark\"] .doc-type-tile.smart:hover{border-color:#71d99a;}",
		"[data-theme=\"dark\"] .doc-branch-title.plus, [data-theme=\"dark\"] .doc-picker .tier-chip.plus{color:#aaaefe;}",
		"[data-theme=\"dark\"] .doc-type-tile.smartplus .tier-icon{color:#aaaefe;}",
		"[data-theme=\"dark\"] .doc-type-tile.smartplus:hover{border-color:#aaaefe;}",
		"[data-theme=\"light\"] .doc-form .btn-plus .key-gate-mark, [data-theme=\"light\"] [data-testid='suggest-categories-ai'] .key-gate-mark{color:#4d55b8;}",
		"[data-theme=\"dark\"] .doc-form .btn-plus .key-gate-mark, [data-theme=\"dark\"] [data-testid='suggest-categories-ai'] .key-gate-mark{color:#aaaefe;}",
		".draft-ledger .row{min-height:40px;font-size:var(--fs-body);font-weight:400;}",
		".draft-ledger .row .amount{font-size:var(--fs-body);font-weight:500;font-variant-numeric:tabular-nums;}",
		"@media (max-width: 640px){.doc-type-grid{grid-template-columns:repeat(2, minmax(0, 1fr));}}",
		"@media (max-width: 420px){.doc-type-grid{grid-template-columns:1fr;}}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("IMV7 import styles missing %q", want)
		}
	}
	if strings.Contains(css, ".doc-picker .tier-chip{font-size:var(--type-11)") {
		t.Fatal("the Import picker still uses the 11 px uppercase tier chip")
	}
}

func TestIMV7SmartPlusActionsStaySecondaryAndMarked(t *testing.T) {
	resetSheet()
	registerIMV7()
	css := Build()
	button := strings.Index(css, ".doc-form .btn-plus, [data-testid='suggest-categories-ai']{")
	primary := strings.Index(css, ".doc-form .btn-primary{")
	if button < 0 {
		t.Fatal("Smart+ action style is missing")
	}
	if strings.Contains(css[button:button+strings.Index(css[button:], "}")], "background:var(--tier-smartplus)") {
		t.Fatal("Smart+ action should not use the filled primary treatment")
	}
	if primary >= 0 && button > primary {
		t.Fatal("Smart+ secondary action was registered after and could override the primary")
	}
	if !strings.Contains(css, ".doc-form .btn-plus::before, [data-testid='suggest-categories-ai']::before{content:\"Smart+\"") {
		t.Fatal("Smart+ action does not carry a visible tier mark")
	}
}
