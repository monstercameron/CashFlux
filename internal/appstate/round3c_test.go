// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestPutSettingsViewerWritesPerPersonStateButNotHouseholdSettings(t *testing.T) {
	app := newApp(t, false)
	role := domain.RoleOwner
	app.SetActiveRoleFunc(func() domain.MemberRole { return role })
	base := app.Settings()
	base.BaseCurrency = "USD"
	base.FXRates = map[string]float64{"EUR": 0.9}
	if err := app.PutSettings(base); err != nil {
		t.Fatalf("owner PutSettings: %v", err)
	}

	role = domain.RoleViewer
	person := app.Settings()
	person.Music = &store.MusicState{Enabled: true, Volume: 0.5, Index: 2}
	person.OpenAIKey = "sk-test"
	person.OpenAIModel = "m"
	if err := app.PutSettings(person); err != nil {
		t.Fatalf("viewer writing per-person state = %v, want it allowed", err)
	}
	if got := app.Settings(); got.Music == nil || got.Music.Index != 2 || got.OpenAIKey != "sk-test" {
		t.Fatalf("per-person state not saved: %+v", got)
	}

	rates := app.Settings()
	rates.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(rates); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer changing FXRates = %v, want ErrReadOnly", err)
	}
	baseCur := app.Settings()
	baseCur.BaseCurrency = "EUR"
	if err := app.PutSettings(baseCur); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer changing BaseCurrency = %v, want ErrReadOnly", err)
	}
	flex := app.Settings()
	flex.FlexBudget = 100
	if err := app.PutSettings(flex); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer changing FlexBudget = %v, want ErrReadOnly", err)
	}
	if got := app.Settings(); got.BaseCurrency != "USD" || got.FXRates["EUR"] != 0.9 || got.FlexBudget != 0 {
		t.Fatalf("a refused write changed household settings: %+v", got)
	}

	role = domain.RoleAdmin
	if err := app.PutSettings(rates); err != nil {
		t.Fatalf("admin changing FXRates = %v, want allowed", err)
	}
}

func TestPutAccountRefusesAClassOrSignChangeOnceTransactionsExist(t *testing.T) {
	app := newApp(t, false)
	card := domain.Account{ID: "card", Name: "Card", Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(9000, "USD"), LiabilitySign: domain.LiabilityPositiveOwed}
	if err := app.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	// With no transactions yet the class and sign may still change.
	flipped := card
	flipped.LiabilitySign = domain.LiabilityNegativeOwed
	if err := app.PutAccount(flipped); err != nil {
		t.Fatalf("flipping the sign of an empty account = %v, want allowed", err)
	}
	if err := app.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "t1", AccountID: "card", Desc: "x", Amount: money.New(-100, "USD"), Date: time.Now()}); err != nil {
		t.Fatal(err)
	}

	asset := card
	asset.Class, asset.Type, asset.LiabilitySign = domain.ClassAsset, domain.TypeChecking, ""
	if err := app.PutAccount(asset); !errors.Is(err, ErrAccountSignLocked) {
		t.Fatalf("liability to asset with transactions = %v, want ErrAccountSignLocked", err)
	}
	if err := app.PutAccount(flipped); !errors.Is(err, ErrAccountSignLocked) {
		t.Fatalf("sign flip with transactions = %v, want ErrAccountSignLocked", err)
	}
	if got := storedAccount(app, "card"); got.Class != domain.ClassLiability || got.LiabilitySign != domain.LiabilityPositiveOwed {
		t.Fatalf("a refused write changed the stored account: %+v", got)
	}

	renamed := card
	renamed.Name = "Renamed"
	if err := app.PutAccount(renamed); err != nil {
		t.Fatalf("unchanged class and sign = %v, want allowed", err)
	}
	unset := card
	unset.LiabilitySign = ""
	if err := app.PutAccount(unset); err != nil {
		t.Fatalf("an unset sign keeps the stored one, got %v", err)
	}
}

func widgetPageUsing(artifactID string) domain.CustomPage {
	return domain.CustomPage{ID: "pg", Slug: "pg", Name: "Page", CreatedAt: time.Now(),
		Widgets: []domain.PageWidget{{ID: "w1", Type: "image", Binding: domain.WidgetBinding{ArtifactID: artifactID}}}}
}

func TestRemoveAccountDocumentKeepsAFileACustomPageWidgetUses(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	ac := docTestAccount(t, app)
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "pic.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.4")}); err != nil {
		t.Fatal(err)
	}
	artID := app.Artifacts()[0].ID
	if err := app.PutCustomPage(widgetPageUsing(artID)); err != nil {
		t.Fatal(err)
	}
	if err := app.RemoveAccountDocument(ac.ID, artID); err != nil {
		t.Fatal(err)
	}
	if len(app.Artifacts()) != 1 {
		t.Fatal("the file was deleted although a custom-page widget is bound to it")
	}
}

func TestDeleteAccountKeepsAFileACustomPageWidgetOrGoalImageUses(t *testing.T) {
	app := newApp(t, false)
	for _, id := range []string{"a", "b"} {
		if err := app.PutAccount(refsAccount(id)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"page.pdf", "goal.pdf", "orphan.pdf"} {
		if err := app.AttachAccountDocument("a", AccountDocumentInput{Name: name, MIME: "application/pdf", Data: []byte("x" + name)}); err != nil {
			t.Fatal(err)
		}
	}
	refs := storedAccount(app, "a").DocRefs
	if len(refs) != 3 {
		t.Fatalf("%d document refs, want 3", len(refs))
	}
	pageArt, goalArt, orphanArt := refs[0].ArtifactID, refs[1].ArtifactID, refs[2].ArtifactID
	if err := app.PutCustomPage(widgetPageUsing(pageArt)); err != nil {
		t.Fatal(err)
	}
	if err := app.PutGoal(domain.Goal{ID: "g", Name: "Goal", Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(0, "USD"), GoalImageArtifactID: goalArt}); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteAccount("a"); err != nil {
		t.Fatal(err)
	}
	for name, art := range map[string]string{"widget": pageArt, "goal image": goalArt} {
		if _, ok, _ := app.Store().GetArtifact(art); !ok {
			t.Fatalf("the %s artifact was deleted with the account", name)
		}
	}
	if _, ok, _ := app.Store().GetArtifact(orphanArt); ok {
		t.Fatal("an unreferenced artifact survived the account delete")
	}
}
