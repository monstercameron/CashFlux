// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTrendFrameKeepsRangeWhenEndpointMovementOverflows(t *testing.T) {
	f := render.New(t)
	defer func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	frame := domain.NewFrame(
		domain.Field{Name: "value", Type: domain.FieldMoney, Values: []any{int64(math.MinInt64), int64(math.MaxInt64)}},
		domain.Field{Name: "t", Type: domain.FieldNumber, Values: []any{int64(1), int64(2)}},
	)
	f.Render(trendFrame(frame, widgetrender.RenderCtx{Net: money.New(math.MaxInt64, "USD")}))
	f.Stabilize()

	delta := byTestID(f, "span", "dashboard-trend-delta")
	if delta == nil || !strings.Contains(strings.ToLower(delta.Text()), "unavailable") || strings.Contains(delta.Text(), "No change") {
		t.Fatalf("overflowing trend delta did not remain unavailable: %s", f.Text())
	}
	rangeNode := byTestID(f, "span", "dashboard-trend-range")
	if rangeNode == nil || !strings.Contains(rangeNode.Text(), " - ") {
		t.Fatalf("valid range was lost with overflowing delta: %s", f.Text())
	}
}
