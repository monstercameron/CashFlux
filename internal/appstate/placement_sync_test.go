// SPDX-License-Identifier: MIT

package appstate

import (
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func testDashboardPlacement(id string) domain.Placement {
	return domain.Placement{
		SchemaVersion: domain.WidgetSpecVersion,
		ID:            id,
		Surface:       "dashboard",
		Spec: domain.WidgetSpec{
			SchemaVersion: domain.WidgetSpecVersion,
			ID:            id,
			Kind:          domain.KindNative,
			NativeID:      id,
		},
		Layout: dashlayout.Item{ID: id, ColSpan: 2, RowSpan: 1, Importance: 50},
	}
}

func TestPutPlacementsIfChangedSkipsWarmDashboardWrites(t *testing.T) {
	app, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	placements := []domain.Placement{testDashboardPlacement("hero"), testDashboardPlacement("recent")}

	before := app.Rev()
	changed, err := app.PutPlacementsIfChanged("dashboard", placements)
	if err != nil || !changed || app.Rev() != before+2 {
		t.Fatalf("first sync: changed=%v err=%v revision=%d, want %d", changed, err, app.Rev(), before+2)
	}
	warmed := app.Rev()
	for i := 0; i < 3; i++ {
		changed, err = app.PutPlacementsIfChanged("dashboard", placements)
		if err != nil || changed || app.Rev() != warmed {
			t.Fatalf("warm sync %d: changed=%v err=%v revision=%d, want %d", i, changed, err, app.Rev(), warmed)
		}
	}

	// A separate app with the same signature still needs its own stored rows.
	other, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Store().Close() })
	changed, err = other.PutPlacementsIfChanged("dashboard", placements)
	if err != nil || !changed {
		t.Fatalf("second app sync: changed=%v err=%v", changed, err)
	}
	if got := other.Placements("dashboard"); len(got) != len(placements) {
		t.Fatalf("second app stored %d placements, want %d", len(got), len(placements))
	}
}

func TestPutPlacementsIfChangedPersistsEachConfiguredChange(t *testing.T) {
	app, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	placements := []domain.Placement{testDashboardPlacement("recent")}
	if _, err := app.PutPlacementsIfChanged("dashboard", placements); err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		name   string
		change func()
	}{
		{"layout span and importance", func() { placements[0].Layout.ColSpan = 3; placements[0].Layout.Importance = 90 }},
		{"visibility", func() { placements[0].Hidden = true }},
		{"widget config", func() { placements[0].Spec.Settings = map[string]string{"count": "8"} }},
		{"style and title", func() { placements[0].Spec.Style.Accent = "#123456"; placements[0].Spec.Title = "My recent" }},
		{"Studio custom item", func() {
			custom := testDashboardPlacement("us:custom")
			custom.Spec.Kind, custom.Spec.NativeID = domain.KindList, ""
			custom.Spec.Pipeline = &domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "transactions"}}
			placements = append(placements, custom)
		}},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			step.change()
			before := app.Rev()
			changed, err := app.PutPlacementsIfChanged("dashboard", placements)
			if err != nil || !changed || app.Rev() != before+1 {
				t.Fatalf("sync: changed=%v err=%v revision=%d, want %d", changed, err, app.Rev(), before+1)
			}
			saved := app.Placements("dashboard")
			if len(saved) != len(placements) {
				t.Fatalf("stored %d placements, want %d", len(saved), len(placements))
			}
			byID := make(map[string]domain.Placement, len(saved))
			for _, p := range saved {
				byID[p.ID] = p
			}
			for _, p := range placements {
				if got, ok := byID[p.ID]; !ok || !reflect.DeepEqual(got, p) {
					t.Fatalf("stored placement %q = %+v, want %+v", p.ID, got, p)
				}
			}
			warmed := app.Rev()
			if changed, err := app.PutPlacementsIfChanged("dashboard", placements); err != nil || changed || app.Rev() != warmed {
				t.Fatalf("repeat sync: changed=%v err=%v revision=%d, want %d", changed, err, app.Rev(), warmed)
			}
		})
	}
}

func TestPutPlacementsIfChangedKeepsErrorsRetryable(t *testing.T) {
	app, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	p := testDashboardPlacement("us:custom")
	p.Spec.Kind, p.Spec.NativeID = domain.KindBuilder, ""
	p.Spec.Graph = json.RawMessage("{")
	before := app.Rev()
	if changed, err := app.PutPlacementsIfChanged("dashboard", []domain.Placement{p}); err == nil || changed || app.Rev() != before {
		t.Fatalf("invalid JSON: changed=%v err=%v revision=%d", changed, err, app.Rev())
	}
	p.Spec.Graph = json.RawMessage(`{"nodes":[]}`)
	if changed, err := app.PutPlacementsIfChanged("dashboard", []domain.Placement{p}); err != nil || !changed || app.Rev() != before+1 {
		t.Fatalf("corrected retry: changed=%v err=%v revision=%d", changed, err, app.Rev())
	}
	foreign := p
	foreign.Surface = "page:custom"
	before = app.Rev()
	if changed, err := app.PutPlacementsIfChanged("dashboard", []domain.Placement{foreign}); err == nil || changed || app.Rev() != before {
		t.Fatalf("surface mismatch: changed=%v err=%v revision=%d", changed, err, app.Rev())
	}
}

func TestPutPlacementsIfChangedReportsClosedStore(t *testing.T) {
	app, err := New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	before := app.Rev()
	changed, err := app.PutPlacementsIfChanged("dashboard", []domain.Placement{testDashboardPlacement("recent")})
	if err == nil || changed || app.Rev() != before {
		t.Fatalf("closed store: changed=%v err=%v revision=%d, want %d", changed, err, app.Rev(), before)
	}
}
