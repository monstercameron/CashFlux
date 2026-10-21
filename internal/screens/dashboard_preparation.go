// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/CashFlux/internal/widgetengine"
	"github.com/monstercameron/CashFlux/internal/widgetregistry"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/CashFlux/internal/widgetvis"
)

var dashboardPreparationNow = time.Now

var dashboardFrameContexts = dashboardview.NewFrameContexts(2)
var dashboardLayouts = &dashboardview.LayoutCache{}
var dashboardStoredPlacements = newViewCache[[]domain.Placement](2)

// A one-minute clock bucket matches the existing engine-variable freshness
// bound. Date/period/FX/data/config changes invalidate independently of time.
func prepareDashboardFrames(ctx widgetrender.RenderCtx, scope string, now time.Time) *dashboardview.PreparedFrames {
	identity := dashboardview.FrameIdentity{Instance: ctx.App.InstanceID(), Revision: ctx.App.Rev(), Preferences: uistate.CurrentDataRevision(),
		Scope: scope, Base: ctx.Base, Rates: fmt.Sprintf("%#v", ctx.Rates), Start: ctx.Start, End: ctx.End, ClockMinute: now.Unix() / 60, VariablesAt: engineVarsMemo.at}
	return dashboardFrameContexts.Prepare(identity, func() widgetengine.DataCtx { return frameDataCtxAt(ctx, now) })
}

func prepareDashboardLayout(app *appstate.App, ready bool, items []dashlayout.Item, configs uistate.WidgetConfigs, hidden widgetvis.Set) *dashboardview.PreparedLayout {
	defaults := make(map[string]domain.WidgetSpec, len(items)+1)
	if spec, ok := widgetregistry.DefaultSpec("hero"); ok {
		defaults["hero"] = spec
	}
	if ready {
		for _, item := range items {
			if spec, ok := widgetregistry.DefaultSpec(item.ID); ok {
				defaults[item.ID] = spec
			}
		}
	}
	stored := dashboardStoredPlacements.GetOrCompute(revKey(app), func() []domain.Placement { return app.Placements("dashboard") })
	layout, _ := dashboardLayouts.Prepare(dashboardview.LayoutInput{Instance: app.InstanceID(), Ready: ready, Items: items, Configs: map[string]widgetcfg.Config(configs), Hidden: hidden, Stored: stored, Defaults: defaults})
	return layout
}
