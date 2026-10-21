// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
)

func TestPlanningPreparedHostSurfacesInvalidInputsAndClosedSources(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	defer func() { _ = app.Store().Close() }()

	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	prepared := preparePlanning(app, now, "bad-trim", "", "bad-amount", "", "", "", 2)
	if prepared.TrimForecast.Available || prepared.TrimForecast.Err == nil {
		t.Fatalf("invalid trim should be an explicit unavailable fact: %+v", prepared.TrimForecast)
	}
	if prepared.Affordability.Available || prepared.Affordability.Err == nil {
		t.Fatalf("invalid purchase amount should be an explicit unavailable fact: %+v", prepared.Affordability)
	}

	if err := app.Store().Close(); err != nil {
		t.Fatalf("close source store: %v", err)
	}
	prepared = preparePlanning(app, now, "", "", "", "", "", "", 2)
	if prepared.Forecast.Available || prepared.Runway.Available || prepared.SafeToSpend.Available {
		t.Fatalf("closed source must not produce financial results: forecast=%+v runway=%+v safe=%+v", prepared.Forecast, prepared.Runway, prepared.SafeToSpend)
	}
	if prepared.Forecast.Err == nil || prepared.SafeToSpend.Err == nil {
		t.Fatalf("closed source errors should remain visible: forecast=%+v safe=%+v", prepared.Forecast, prepared.SafeToSpend)
	}
}
