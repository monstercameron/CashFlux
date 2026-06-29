// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
)

func TestPlacementCRUD(t *testing.T) {
	s := newStore(t)
	pl := domain.Placement{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            "kpi-networth",
		Surface:       "dashboard",
		Spec: domain.WidgetSpec{
			SchemaVersion: domain.WidgetSpecVersion,
			ID:            "kpi-networth",
			Kind:          domain.KindNative,
			NativeID:      "kpi-networth",
			Settings:      widgetcfg.Config{"_accent": "#7c83ff"},
		},
		Layout: dashlayout.Item{ID: "kpi-networth", ColSpan: 1, RowSpan: 1},
	}
	if err := s.PutPlacement(pl); err != nil {
		t.Fatalf("PutPlacement: %v", err)
	}
	// A placement on another surface with the SAME widget id must not collide.
	pl2 := pl
	pl2.Surface = "page:budget"
	if err := s.PutPlacement(pl2); err != nil {
		t.Fatalf("PutPlacement (page): %v", err)
	}

	dash, err := s.PlacementsForSurface("dashboard")
	if err != nil {
		t.Fatalf("PlacementsForSurface: %v", err)
	}
	if len(dash) != 1 || dash[0].Surface != "dashboard" || dash[0].Spec.Settings["_accent"] != "#7c83ff" {
		t.Fatalf("dashboard placements wrong: %+v", dash)
	}
	all, _ := s.ListPlacements()
	if len(all) != 2 {
		t.Fatalf("expected 2 placements across surfaces, got %d", len(all))
	}

	if ok, err := s.DeletePlacement(pl); err != nil || !ok {
		t.Fatalf("DeletePlacement: ok=%v err=%v", ok, err)
	}
	if dash, _ := s.PlacementsForSurface("dashboard"); len(dash) != 0 {
		t.Fatalf("placement not deleted: %+v", dash)
	}
}

// TestPlacementContentLayoutRoundTrip proves a custom intra-tile content layout
// (the layout-engine config: blocks + per-block Style + tile Style) persists and
// reloads losslessly — so a saved compound widget acts purely as hydration data.
func TestPlacementContentLayoutRoundTrip(t *testing.T) {
	s := newStore(t)
	pl := domain.Placement{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            "spotlight",
		Surface:       "dashboard",
		Spec: domain.WidgetSpec{
			SchemaVersion: domain.WidgetSpecVersion, ID: "spotlight", Kind: domain.KindText,
			Style: domain.Style{Background: "var(--accent-dim)", Align: "center"},
			Content: domain.ContentLayout{Mode: domain.LayoutCustom, Blocks: []domain.Block{
				{Kind: domain.BlockIcon, Bind: "sparkles"},
				{Kind: domain.BlockFigure, Bind: "income|currency", ColSpan: 2, Style: domain.Style{Text: "var(--up)"}},
				{Kind: domain.BlockText, Text: "Net {{cashflow_net|signed}}"},
			}},
		},
		Layout: dashlayout.Item{ID: "spotlight", ColSpan: 2, RowSpan: 2},
	}
	if err := s.PutPlacement(pl); err != nil {
		t.Fatalf("PutPlacement: %v", err)
	}
	got, err := s.PlacementsForSurface("dashboard")
	if err != nil || len(got) != 1 {
		t.Fatalf("PlacementsForSurface: got %d err %v", len(got), err)
	}
	sp := got[0].Spec
	if sp.Content.Mode != domain.LayoutCustom || len(sp.Content.Blocks) != 3 {
		t.Fatalf("content layout lost: %+v", sp.Content)
	}
	if b := sp.Content.Blocks[1]; b.Bind != "income|currency" || b.ColSpan != 2 || b.Style.Text != "var(--up)" {
		t.Fatalf("block (bind/colspan/style) lost: %+v", b)
	}
	if sp.Style.Background != "var(--accent-dim)" || sp.Style.Align != "center" {
		t.Fatalf("tile style lost: %+v", sp.Style)
	}
}

func TestDatasetPlacementRoundTrip(t *testing.T) {
	s := newStore(t)
	want := domain.Placement{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            "recent",
		Surface:       "dashboard",
		Spec:          domain.WidgetSpec{SchemaVersion: domain.WidgetSpecVersion, ID: "recent", Kind: domain.KindNative, NativeID: "recent"},
		Layout:        dashlayout.Item{ID: "recent", ColSpan: 2, RowSpan: 2},
		Hidden:        true,
	}
	if err := s.PutPlacement(want); err != nil {
		t.Fatalf("PutPlacement: %v", err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Placements) != 1 {
		t.Fatalf("snapshot placements = %d, want 1", len(snap.Placements))
	}

	// Load the snapshot into a fresh store and read it back — export/import lossless.
	s2 := newStore(t)
	if err := s2.Load(snap); err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, _ := s2.PlacementsForSurface("dashboard")
	if len(got) != 1 || got[0].ID != "recent" || !got[0].Hidden || got[0].Layout.ColSpan != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
