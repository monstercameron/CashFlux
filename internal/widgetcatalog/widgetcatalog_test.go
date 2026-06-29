// SPDX-License-Identifier: MIT

package widgetcatalog

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/customfields"
)

func TestMetricsIncludeBuiltinsAndCustomFields(t *testing.T) {
	defs := []customfields.Def{
		{ID: "1", EntityType: "transaction", Key: "tip", Label: "Tip", Type: customfields.TypeNumber},
		{ID: "2", EntityType: "account", Key: "color", Label: "Color", Type: customfields.TypeText}, // non-numeric → excluded
	}
	ms := Metrics(defs, nil)
	byName := map[string]Metric{}
	for _, m := range ms {
		byName[m.Name] = m
	}
	// A curated core molecule is present with its friendly label + doc + group.
	nw, ok := byName["net_worth"]
	if !ok || nw.Label != "Net worth" || nw.Doc == "" || nw.Group != GroupCore {
		t.Fatalf("net_worth metric = %+v", nw)
	}
	// New atoms surface automatically.
	if _, ok := byName["asset_accounts"]; !ok {
		t.Error("asset_accounts metric missing")
	}
	// The numeric custom field appears, labelled from its def; the text one does not.
	tip, ok := byName["cf_txn_tip"]
	if !ok || tip.Group != GroupCustom || tip.Label != "Tip (transaction)" {
		t.Fatalf("custom-field metric = %+v (ok=%v)", tip, ok)
	}
	if _, ok := byName["cf_acct_color"]; ok {
		t.Error("non-numeric custom field should not be a metric")
	}
}

func TestOptionSetsNonEmpty(t *testing.T) {
	for name, opts := range map[string][]Option{
		"Formats": Formats(), "Kinds": Kinds(), "Collections": Collections(),
		"SeriesMetrics": SeriesMetrics(), "Transforms": Transforms(),
		"BlockKinds": BlockKinds(), "TemplateVerbs": TemplateVerbs(),
	} {
		if len(opts) == 0 {
			t.Errorf("%s returned no options", name)
		}
		for _, o := range opts {
			if o.Value == "" || o.Label == "" {
				t.Errorf("%s has an empty option: %+v", name, o)
			}
		}
	}
}
