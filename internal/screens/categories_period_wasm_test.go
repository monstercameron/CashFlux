//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCategoriesKeepsUsageAndWithholdsUnknownFXAmounts(t *testing.T) {
	app := pickerTestApp(t)
	if err := app.PutAccount(domain.Account{
		ID: "categories-period-account", Name: "Checking", Currency: "EUR", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := app.PutTransaction(domain.Transaction{
		ID: "categories-period-foreign", AccountID: "categories-period-account", Desc: "Foreign purchase", Date: time.Now(),
		CategoryID: "c-zoo", Amount: money.New(-1234, "EUR"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Categories))
	f.Stabilize()
	row := f.ByID("cat-row-c-zoo")
	if row == nil || !strings.Contains(row.Text(), "1 transaction") {
		t.Fatalf("independent transaction usage was lost: %s", f.Text())
	}
	if byTestID(f, "p", "categories-calculation-unavailable") == nil {
		t.Fatalf("missing FX was rendered as a healthy zero: %s", f.Text())
	}
}
