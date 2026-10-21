// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestArtifactsRowsUsePreparedReferenceFacts(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	artifact := domain.Artifact{ID: "receipt", Name: "Receipt", Kind: "image", Bytes: []byte{1}, Size: 1}
	if err := app.PutArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	account := domain.Account{ID: "cash", Name: "Cash", Currency: "USD", Class: domain.ClassAsset,
		Type: domain.TypeChecking, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "txn", AccountID: account.ID, Desc: "Purchase",
		Date: time.Now(), Amount: money.New(-100, "USD"), Attachments: []domain.AttachmentRef{{ArtifactID: artifact.ID}, {ArtifactID: artifact.ID}}}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCustomPage(domain.CustomPage{ID: "page", Name: "Receipts", Slug: "receipts",
		Widgets: []domain.PageWidget{{Binding: domain.WidgetBinding{ArtifactID: artifact.ID}}}}); err != nil {
		t.Fatal(err)
	}
	artifactsSummaryCache.Clear()
	t.Cleanup(artifactsSummaryCache.Clear)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Artifacts))
	f.Stabilize()
	if byTestID(f, "div", "artifact-refs") == nil || byTestID(f, "div", "artifact-page-refs") == nil {
		t.Fatalf("artifact row omitted prepared reference facts: %s", f.Text())
	}
	if artifactsSummaryCache.Len() != 1 {
		t.Fatalf("prepared summary cache entries = %d, want one", artifactsSummaryCache.Len())
	}
	first, err := artifactVaultSnapshot(app)
	if err != nil {
		t.Fatal(err)
	}
	if first.Summary.TransactionReferences[artifact.ID] != 2 {
		t.Fatalf("prepared transaction references = %d, want 2", first.Summary.TransactionReferences[artifact.ID])
	}
	warm, err := artifactVaultSnapshot(app)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Artifacts) == 0 || len(warm.Artifacts) == 0 || &first.Artifacts[0] != &warm.Artifacts[0] {
		t.Fatal("warm render replaced prepared artifact slice; expected cache reuse without a new snapshot")
	}
	if err := app.PutTransaction(domain.Transaction{ID: "txn-2", AccountID: account.ID, Desc: "Second purchase",
		Date: time.Now(), Amount: money.New(-200, "USD"), Attachments: []domain.AttachmentRef{{ArtifactID: artifact.ID}}}); err != nil {
		t.Fatal(err)
	}
	updated, err := artifactVaultSnapshot(app)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Summary.TransactionReferences[artifact.ID] != 3 || &first.Artifacts[0] == &updated.Artifacts[0] {
		t.Fatal("revision did not refresh reference facts and artifact snapshot")
	}
	// The test mutates through App outside the screen's own upload handlers, so
	// broadcast that data change only after verifying the prepared cache model.
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(byTestID(f, "div", "artifact-refs").Text(), "3") {
		t.Fatalf("render kept stale reference count after revision: %s", f.Text())
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := artifactVaultSnapshot(app); err == nil {
		t.Fatal("closed store passed the fallible source guard")
	}
	uistate.BumpDataRevision()
	f.Stabilize()
	if !strings.Contains(strings.ToLower(f.Text()), "unavailable") {
		t.Fatalf("closed source did not render unavailable state: %s", f.Text())
	}
}
