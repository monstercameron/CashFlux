// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/store"
)

var settingsBenchmarkResult store.Settings

// BenchmarkSettingsSnapshot compares repeated legacy SQL/JSON reads with the
// checked snapshot used by controllers. Each operation owns nine returned values;
// the prepared path includes readiness checks and deep copies on every read.
func BenchmarkSettingsSnapshot(b *testing.B) {
	a, err := New(io.Discard, false)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = a.Store().Close() })
	settings := store.Settings{
		BaseCurrency:       "USD",
		FXRates:            map[string]float64{},
		FXUpdatedAt:        map[string]time.Time{},
		FreshnessOverrides: map[string]int{"checking": 7, "savings": 30, "credit": 14},
		PayoffBaseline:     &store.PayoffBaseline{TotalOwed: 1200000, Currency: "USD"},
		Music:              &store.MusicState{Volume: 0.4, Index: 2},
	}
	for i := range 8 {
		code := fmt.Sprintf("FX%d", i)
		settings.FXRates[code] = 1.2
		settings.FXUpdatedAt[code] = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	}
	for i := range 12 {
		settings.AISpend = append(settings.AISpend, aispend.Bucket{Feature: "assistant", Calls: i + 1, Tokens: 800})
	}
	if err := a.Store().PutSettings(settings); err != nil {
		b.Fatal(err)
	}
	for _, candidate := range []struct {
		name string
		read func() (store.Settings, error)
	}{{"SQLJSON", a.Store().GetSettings}, {"CheckedSnapshot", a.SettingsChecked}} {
		b.Run(candidate.name, func(b *testing.B) {
			if _, err := candidate.read(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for range 9 {
					value, err := candidate.read()
					if err != nil {
						b.Fatal(err)
					}
					settingsBenchmarkResult = value
				}
			}
		})
	}
}
