// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/viewcache"
)

type smartResultKey struct {
	Context  string
	Now      time.Time
	Page     smart.Page
	Settings string
	Language string
}

// Nine routed pages can all ask for a SMART result during one sweep. Retain a
// few recent settings/pages so returning to Dashboard still reuses its result.
var smartResultsCache = viewcache.New[smartResultKey, []smart.Insight](16)

// Test seams count actual engine invocations without changing detector behavior.
var smartPreparedRun = smartengine.Run
var smartPreparedRunPage = smartengine.RunPage

func preparedSmartInsights(app *appstate.App, weekStart time.Weekday, settings smart.Settings, page smart.Page) ([]smart.Insight, error) {
	return preparedSmartInsightsAt(app, weekStart, settings, page, string(uistate.ActiveLanguage()), time.Now())
}

func preparedSmartInsightsAt(app *appstate.App, weekStart time.Weekday, settings smart.Settings, page smart.Page, language string, now time.Time) ([]smart.Insight, error) {
	// A closed/unavailable store must not be disguised by a warm result cache.
	if err := app.Store().CheckReady(); err != nil {
		return nil, err
	}
	in := buildSmartInputAt(app, weekStart, now)
	run := func() []smart.Insight {
		if page == "" {
			return smartPreparedRun(in, settings)
		}
		return smartPreparedRunPage(in, settings, page)
	}
	encodedSettings, err := json.Marshal(settings)
	if err != nil {
		// Keep the view available if a future Settings field cannot be encoded;
		// the result is fresh and intentionally never entered into the cache.
		slog.Warn("smart result cache key failed", "err", err)
		return run(), nil
	}
	key := smartResultKey{
		Context:  smartContextKey(app, weekStart, in.Now),
		Now:      in.Now,
		Page:     page,
		Settings: string(encodedSettings),
		Language: language,
	}
	result := smartResultsCache.GetOrCompute(key, func() []smart.Insight { return cloneSmartInsights(run()) })
	return cloneSmartInsights(result), nil
}
