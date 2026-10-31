// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aispend"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestPutSettingsValidatesAndGuardsTheDefaultAccount(t *testing.T) {
	app := newApp(t, false)
	archived := refsAccount("old")
	archived.Archived = true
	for _, ac := range []domain.Account{refsAccount("chk"), archived} {
		if err := app.PutAccount(ac); err != nil {
			t.Fatal(err)
		}
	}
	with := func(id string) error {
		s := app.Settings()
		s.DefaultAccountID = id
		return app.PutSettings(s)
	}
	if err := with("missing"); err == nil {
		t.Error("a missing account was saved as the default")
	}
	if err := with("old"); err == nil {
		t.Error("an archived account was saved as the default")
	}
	if err := with("chk"); err != nil {
		t.Fatalf("a valid default was refused: %v", err)
	}

	// A default already stored stays writable when the account is archived later and other fields change.
	chk := storedAccount(app, "chk")
	chk.Archived = true
	if err := app.PutAccount(chk); err != nil {
		t.Fatal(err)
	}
	s := app.Settings()
	s.FlexBudget = 5000
	if err := app.PutSettings(s); err != nil {
		t.Fatalf("an unrelated settings write was refused: %v", err)
	}

	asViewer(app)
	if err := with(""); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer clearing the default = %v, want ErrReadOnly", err)
	}
	s = app.Settings()
	s.FlexBudget = 7000
	if err := app.PutSettings(s); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer changing the household flex budget = %v, want ErrReadOnly", err)
	}
	s = app.Settings()
	s.Music = &store.MusicState{Enabled: true, Volume: 0.4}
	if err := app.PutSettings(s); err != nil {
		t.Errorf("viewer saving their own settings = %v, want success", err)
	}
}

func TestPutSettingsOnAnAppWithoutAStore(t *testing.T) {
	var app *App
	if err := app.PutSettings(store.Settings{}); !errors.Is(err, ErrDurabilityUnavailable) {
		t.Fatalf("PutSettings on a nil App = %v, want ErrDurabilityUnavailable", err)
	}
}

func TestPutSettingsViewerCannotChangeTheHouseholdAISpendCapOrRecord(t *testing.T) {
	app := newApp(t, false)
	role := domain.RoleOwner
	app.SetActiveRoleFunc(func() domain.MemberRole { return role })
	now := time.Now().UTC()
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	s := app.Settings()
	s.AISpendCapUSD = 20
	s.AISpend = []aispend.Bucket{{Month: month, Feature: "assistant", Calls: 3, Tokens: 900}}
	if err := app.PutSettings(s); err != nil {
		t.Fatalf("owner setting the cap: %v", err)
	}

	role = domain.RoleViewer
	zeroCap := app.Settings()
	zeroCap.AISpendCapUSD = 0
	if err := app.PutSettings(zeroCap); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer zeroing the cap = %v, want ErrReadOnly", err)
	}
	wiped := app.Settings()
	wiped.AISpend = nil
	if err := app.PutSettings(wiped); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer wiping the spend record = %v, want ErrReadOnly", err)
	}
	if got := app.Settings(); got.AISpendCapUSD != 20 || len(got.AISpend) != 1 {
		t.Fatalf("a refused write changed the household AI settings: cap %v, record %+v", got.AISpendCapUSD, got.AISpend)
	}

	music := app.Settings()
	music.Music = &store.MusicState{Enabled: true, Volume: 0.4}
	if err := app.PutSettings(music); err != nil {
		t.Fatalf("viewer saving music state = %v, want success", err)
	}

	// Metering a viewer's own AI call still lands in the household record.
	app.RecordAISpend("assistant", "gpt-test", ai.Usage{TotalTokens: 100})
	if got := app.Settings(); len(got.AISpend) != 1 || got.AISpend[0].Calls != 4 || got.AISpend[0].Tokens != 1000 {
		t.Fatalf("a viewer's AI call was not metered: %+v", got.AISpend)
	}
}
