// SPDX-License-Identifier: MIT

package domain

import (
	"encoding/json"
	"testing"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
)

func TestWidgetSpecValidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    WidgetSpec
		wantErr bool
	}{
		{
			name:    "unknown kind",
			spec:    WidgetSpec{ID: "a", Kind: "bogus"},
			wantErr: true,
		},
		{
			name:    "kpi requires scalar",
			spec:    WidgetSpec{ID: "a", Kind: KindKPI},
			wantErr: true,
		},
		{
			name: "valid kpi",
			spec: WidgetSpec{ID: "a", Kind: KindKPI, Scalar: &ScalarBind{Expr: "net_worth", Format: "money"}},
		},
		{
			name:    "table requires pipeline",
			spec:    WidgetSpec{ID: "a", Kind: KindTable},
			wantErr: true,
		},
		{
			name: "valid table",
			spec: WidgetSpec{ID: "a", Kind: KindTable, Pipeline: &Pipeline{Source: Source{Kind: SourceCollection, Collection: "transactions"}}},
		},
		{
			name: "more than one binding",
			spec: WidgetSpec{ID: "a", Kind: KindKPI,
				Scalar:   &ScalarBind{Expr: "x"},
				Pipeline: &Pipeline{Source: Source{Kind: SourceCollection, Collection: "transactions"}}},
			wantErr: true,
		},
		{
			name:    "native requires nativeId",
			spec:    WidgetSpec{ID: "a", Kind: KindNative},
			wantErr: true,
		},
		{
			name: "valid native",
			spec: WidgetSpec{ID: "a", Kind: KindNative, NativeID: "smart-digest"},
		},
		{
			name:    "builder requires graph",
			spec:    WidgetSpec{ID: "a", Kind: KindBuilder},
			wantErr: true,
		},
		{
			name: "valid builder",
			spec: WidgetSpec{ID: "a", Kind: KindBuilder, Graph: json.RawMessage(`{"nodes":[]}`)},
		},
		{
			name:    "text takes no binding",
			spec:    WidgetSpec{ID: "a", Kind: KindText, NativeID: "x"},
			wantErr: true,
		},
		{
			name: "valid text",
			spec: WidgetSpec{ID: "a", Kind: KindText},
		},
		{
			name: "valid spacer",
			spec: WidgetSpec{ID: "a", Kind: KindSpacer},
		},
		{
			name:    "standard layout with blocks is invalid",
			spec:    WidgetSpec{ID: "a", Kind: KindText, Content: ContentLayout{Mode: LayoutStandard, Blocks: []Block{{Kind: BlockText}}}},
			wantErr: true,
		},
		{
			name:    "custom layout needs blocks",
			spec:    WidgetSpec{ID: "a", Kind: KindText, Content: ContentLayout{Mode: LayoutCustom}},
			wantErr: true,
		},
		{
			name: "valid custom layout",
			spec: WidgetSpec{ID: "a", Kind: KindText, Content: ContentLayout{Mode: LayoutCustom, Blocks: []Block{{Kind: BlockText, Text: "hi"}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPlacementValidate(t *testing.T) {
	good := Placement{
		ID: "p1", Surface: "dashboard",
		Spec:   WidgetSpec{ID: "w1", Kind: KindKPI, Scalar: &ScalarBind{Expr: "net_worth"}},
		Layout: dashlayout.Item{ID: "w1", ColSpan: 1, RowSpan: 1},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid placement rejected: %v", err)
	}
	noSurface := good
	noSurface.Surface = ""
	if err := noSurface.Validate(); err == nil {
		t.Fatal("empty surface should be rejected")
	}
	badSpec := good
	badSpec.Spec = WidgetSpec{ID: "w1", Kind: KindKPI} // missing scalar
	if err := badSpec.Validate(); err == nil {
		t.Fatal("invalid embedded spec should be rejected")
	}
}

func TestStyleCSS(t *testing.T) {
	if got := (Style{}).CSS(); len(got) != 0 {
		t.Fatalf("empty style should emit no CSS, got %v", got)
	}
	s := Style{Background: "var(--bg-card)", Accent: "#7c83ff", Shadow: "0 1px 3px rgba(0,0,0,.35)", Align: "left", FontWeight: "600"}
	css := s.CSS()
	if css["background-color"] != "var(--bg-card)" {
		t.Errorf("background-color = %q", css["background-color"])
	}
	if css["text-align"] != "left" || css["font-weight"] != "600" {
		t.Errorf("align/weight = %q/%q", css["text-align"], css["font-weight"])
	}
	if css["box-shadow"] != "inset 0 3px 0 0 #7c83ff, 0 1px 3px rgba(0,0,0,.35)" {
		t.Errorf("box-shadow = %q", css["box-shadow"])
	}
	if (Style{Shadow: "none"}).CSS()["box-shadow"] != "none" {
		t.Error("explicit none shadow should emit box-shadow:none")
	}
}

func TestWidgetSpecJSONRoundTrip(t *testing.T) {
	in := WidgetSpec{
		SchemaVersion: WidgetSpecVersion,
		ID:            "kpi-networth",
		Kind:          KindKPI,
		Title:         "Net worth",
		Scalar:        &ScalarBind{Expr: "net_worth", Format: "money"},
		Style:         Style{Accent: "var(--accent)"},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out WidgetSpec
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID != in.ID || out.Kind != in.Kind || out.Scalar == nil || out.Scalar.Expr != "net_worth" {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("round-tripped spec invalid: %v", err)
	}
}
