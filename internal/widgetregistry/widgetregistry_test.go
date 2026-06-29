// SPDX-License-Identifier: MIT

package widgetregistry

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestRegistrySeededFromDefaults(t *testing.T) {
	// Every default bento item must have a descriptor with a matching default size.
	for _, it := range dashlayout.DefaultItems() {
		d, ok := Get(it.ID)
		if !ok {
			t.Fatalf("id %q missing from registry", it.ID)
		}
		if d.DefaultCol != it.ColSpan || d.DefaultRow != it.RowSpan {
			t.Errorf("id %q size = %dx%d, want %dx%d", it.ID, d.DefaultCol, d.DefaultRow, it.ColSpan, it.RowSpan)
		}
	}
	// The full-width welcome/hero is registered too (not a packed grid item).
	if _, ok := Get("hero"); !ok {
		t.Fatal("hero descriptor missing from registry")
	}
	// Catalog covers every default item plus the hero.
	if got, want := len(Catalog()), len(dashlayout.DefaultItems())+1; got != want {
		t.Fatalf("Catalog() = %d descriptors, want %d (DefaultItems + hero)", got, want)
	}
}

func TestDefaultSpecValidates(t *testing.T) {
	for _, d := range Catalog() {
		spec, ok := DefaultSpec(d.ID)
		if !ok {
			t.Fatalf("DefaultSpec(%q) not found", d.ID)
		}
		switch d.Class {
		case ClassDataDriven:
			// A DataDriven widget is fully described by its spec and carries no NativeID:
			// a KPI has a scalar binding; a List/Chart has a pipeline.
			if spec.NativeID != "" {
				t.Errorf("DefaultSpec(%q) DataDriven but has nativeId %q", d.ID, spec.NativeID)
			}
			switch spec.Kind {
			case domain.KindKPI:
				if spec.Scalar == nil {
					t.Errorf("DefaultSpec(%q) kpi without scalar binding", d.ID)
				}
			case domain.KindList, domain.KindTable, domain.KindChart:
				if spec.Pipeline == nil {
					t.Errorf("DefaultSpec(%q) %s without pipeline", d.ID, spec.Kind)
				}
			case domain.KindText, domain.KindImage, domain.KindSpacer:
				// A compound widget is described by a custom ContentLayout (blocks).
				if spec.Content.Mode != domain.LayoutCustom || len(spec.Content.Blocks) == 0 {
					t.Errorf("DefaultSpec(%q) compound without custom content blocks", d.ID)
				}
			default:
				t.Errorf("DefaultSpec(%q) DataDriven with unexpected kind %q", d.ID, spec.Kind)
			}
		default:
			if spec.Kind != domain.KindNative || spec.NativeID != d.ID {
				t.Errorf("DefaultSpec(%q) = kind %q nativeId %q, want native/%s", d.ID, spec.Kind, spec.NativeID, d.ID)
			}
		}
		if err := spec.Validate(); err != nil {
			t.Errorf("DefaultSpec(%q) invalid: %v", d.ID, err)
		}
	}
	if _, ok := DefaultSpec("does-not-exist"); ok {
		t.Fatal("DefaultSpec should report missing ids")
	}
}

// TestDataDrivenKPISpec verifies the declarative KPI widgets carry a complete,
// independent scalar binding (formula + format + templated sub) — the "fully
// composable from a spec" contract the engine hydrates.
func TestDataDrivenKPISpec(t *testing.T) {
	for _, id := range []string{"kpi-assets", "kpi-liabilities"} {
		d, ok := Get(id)
		if !ok || d.Class != ClassDataDriven {
			t.Fatalf("%s: class = %v, want data-driven", id, d.Class)
		}
		spec, _ := DefaultSpec(id)
		if spec.Kind != domain.KindKPI || spec.Scalar == nil {
			t.Fatalf("%s: spec not a KPI scalar binding: %+v", id, spec)
		}
		if spec.Scalar.Expr == "" || spec.Scalar.Format == "" {
			t.Errorf("%s: incomplete binding %+v", id, *spec.Scalar)
		}
		// The returned spec must not alias the registry table (independent placements).
		spec.Scalar.Expr = "mutated"
		if again, _ := DefaultSpec(id); again.Scalar.Expr == "mutated" {
			t.Errorf("%s: DefaultSpec aliases the shared binding table", id)
		}
	}
}

func TestDefaultPlacement(t *testing.T) {
	pl, ok := DefaultPlacement("kpi-networth", "dashboard")
	if !ok {
		t.Fatal("DefaultPlacement(kpi-networth) failed")
	}
	if err := pl.Validate(); err != nil {
		t.Fatalf("placement invalid: %v", err)
	}
	// kpi-networth is a declarative KPI: a scalar binding, no NativeID.
	if pl.Surface != "dashboard" || pl.Spec.Kind != domain.KindKPI || pl.Spec.Scalar == nil {
		t.Fatalf("unexpected placement: %+v", pl)
	}
	if _, ok := DefaultPlacement("nope", "dashboard"); ok {
		t.Fatal("unknown id should not produce a placement")
	}
}
