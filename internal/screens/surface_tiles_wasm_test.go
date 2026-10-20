// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

const (
	surfaceTilesOrderA = "surface-tiles-test-order-a"
	surfaceTilesOrderB = "surface-tiles-test-order-b"
	surfaceTilesStateA = "surface-tiles-test-state-a"
	surfaceTilesStateB = "surface-tiles-test-state-b"
)

func TestSurfaceTilesClassAttrsOrderContextAndUnavailableTile(t *testing.T) {
	widgetrender.Register(surfaceTilesOrderA, func(ctx widgetrender.RenderCtx) ui.Node {
		return P(Attr("data-testid", "surface-a"), ctx.Spec.ID, ":", ctx.Base)
	})
	widgetrender.Register(surfaceTilesOrderB, func(ctx widgetrender.RenderCtx) ui.Node {
		return P(Attr("data-testid", "surface-b"), ctx.Spec.ID, ":", ctx.Base)
	})

	specs := []domain.WidgetSpec{
		{ID: "a-placement", Kind: domain.KindNative, NativeID: surfaceTilesOrderA},
		{ID: "unknown-placement", Kind: domain.KindNative, NativeID: "surface-tiles-no-renderer"},
		{ID: "b-placement", Kind: domain.KindNative, NativeID: surfaceTilesOrderB},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(surfaceTiles(surfaceTilesProps{
		Class:   "bento-goals",
		Specs:   specs,
		Context: widgetrender.RenderCtx{Base: "USD"},
		Attrs:   []any{Attr("data-testid", "goals-surface"), Attr("data-method", "monthly")},
	}))

	root := findByTestID(f, "div", "goals-surface")
	if root == nil {
		t.Fatal("bento root did not preserve caller attributes")
	}
	if got := root.Attr("class"); got != "bento bento-goals" {
		t.Fatalf("bento class = %q, want %q", got, "bento bento-goals")
	}
	if got := root.Attr("data-method"); got != "monthly" {
		t.Fatalf("bento data-method = %q, want monthly", got)
	}
	if got := strings.Join(strings.Fields(f.Text()), " "); got != "a-placement : USD b-placement : USD" {
		t.Fatalf("tile text/order/context = %q, want available tiles in spec order", got)
	}
	if findByTestID(f, "p", "surface-a") == nil || findByTestID(f, "p", "surface-b") == nil || findByTestID(f, "p", "unknown-placement") != nil {
		t.Fatal("known tiles should render and unavailable tile should be skipped")
	}
}

type surfaceTilesStateProps struct{ ID string }

func surfaceTilesStateProbe(props surfaceTilesStateProps) ui.Node {
	count := ui.UseState(0)
	return Button(
		ID(props.ID),
		Attr("data-testid", props.ID),
		OnClick(func() { count.Set(count.Get() + 1) }),
		props.ID, ":", count.Get(),
	)
}

func TestSurfaceTilesKeepsComponentStateWithStableSpecKeys(t *testing.T) {
	register := func(nativeID string) {
		widgetrender.Register(nativeID, func(ctx widgetrender.RenderCtx) ui.Node {
			return ui.CreateElement(surfaceTilesStateProbe, surfaceTilesStateProps{ID: ctx.Spec.ID})
		})
	}
	register(surfaceTilesStateA)
	register(surfaceTilesStateB)

	spec := func(id, nativeID string) domain.WidgetSpec {
		return domain.WidgetSpec{ID: id, Kind: domain.KindNative, NativeID: nativeID}
	}
	first := surfaceTilesProps{
		Class: "bento-test",
		Specs: []domain.WidgetSpec{
			spec("tile-a", surfaceTilesStateA),
			spec("tile-b", surfaceTilesStateB),
		},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(surfaceTiles(first))
	f.ClickByID("tile-a")

	first.Specs = []domain.WidgetSpec{
		spec("tile-b", surfaceTilesStateB),
		spec("tile-a", surfaceTilesStateA),
	}
	f.Rerender(surfaceTiles(first))

	if got := strings.Join(strings.Fields(f.Text()), " "); got != "tile-b : 0 tile-a : 1" {
		t.Fatalf("tile state after reorder = %q, want state to follow stable spec IDs", got)
	}
}
