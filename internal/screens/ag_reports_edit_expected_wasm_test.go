// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func assistantUpdateFixture(t *testing.T) (*appstate.App, chatTool) {
	t.Helper()
	app, _ := assistantCreationFixture(t)
	if err := app.PutTransaction(assistantCreationRow("edit-original", "Market")); err != nil {
		t.Fatal(err)
	}
	return app, agExpectedUpdateTool(app)
}
func assistantUpdateRow(t *testing.T, app *appstate.App, id string) domain.Transaction {
	t.Helper()
	row, found, err := app.Store().GetTransaction(id)
	if err != nil || !found {
		t.Fatalf("missing row %s: %v", id, err)
	}
	return row
}
func assistantUpdateRejectUnchanged(t *testing.T, app *appstate.App, tool chatTool, raw string) {
	t.Helper()
	before, rev := app.Transactions(), app.Rev()
	preview, out := assistantCreationApprove(tool, raw)
	if !strings.Contains(preview, "Can't prepare") || !strings.Contains(out, "Can't make") || app.Rev() != rev || !reflect.DeepEqual(before, app.Transactions()) {
		t.Fatalf("invalid edit mutated: %s -> %s / %s", raw, preview, out)
	}
}

func TestAssistantExpectedUpdateRequiresPreviewAndBindsResolvedID(t *testing.T) {
	app, tool := assistantUpdateFixture(t)
	raw := json.RawMessage(`{"id":"edit-","description":"Reviewed replacement"}`)
	rev := app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "Review and approve") || app.Rev() != rev {
		t.Fatalf("unapproved write: %s", out)
	}
	if preview := tool.preview(raw); strings.Contains(preview, "Can't prepare") || !strings.Contains(preview, "edit-original") {
		t.Fatal(preview)
	}
	// Another matching prefix appearing after preview must not retarget the approval.
	if err := app.PutTransaction(assistantCreationRow("edit-added", "New")); err != nil {
		t.Fatal(err)
	}
	if out := tool.run(raw); !strings.Contains(out, "Updated exact transaction ID edit-original") {
		t.Fatal(out)
	}
	if assistantUpdateRow(t, app, "edit-original").Desc != "Reviewed replacement" || assistantUpdateRow(t, app, "edit-added").Desc != "Market purchase" {
		t.Fatal("prefix was retargeted")
	}
	rev = app.Rev()
	if out := tool.run(raw); !strings.Contains(out, "already processed") || app.Rev() != rev {
		t.Fatalf("approval reused: %s", out)
	}
}

func TestAssistantExpectedUpdateStrictValidationAndExactAmounts(t *testing.T) {
	app, tool := assistantUpdateFixture(t)
	for _, raw := range []string{
		`{"id":"edit-original","description":""}`, `{"id":"edit-original","date":"2026-02-30"}`,
		`{"id":"edit-original","date":"yesterday"}`, `{"id":"edit-original","amount":1.001}`,
		`{"id":"edit-original","amount":1e2}`, `{"id":"edit-original","amount":null}`,
		`{"id":"edit-original","amount":-1}`, `{"id":"edit-original","amount":0}`,
		`{"id":"edit-original","amount":"92233720368547758.08"}`, `{"id":"edit-original","payee":null}`,
		`{"id":"edit-original","unknown":true}`, `{"id":"edit-original","category":"Foo"}`,
		`{"id":"edit-original","category_id":"food","category":"Food"}`,
	} {
		assistantUpdateRejectUnchanged(t, app, tool, raw)
	}
	preview, out := assistantCreationApprove(tool, `{"id":"edit-original","amount":90071992547409.93}`)
	if !strings.Contains(preview, "-90071992547409.93 USD") || !strings.Contains(out, "Updated") || assistantUpdateRow(t, app, "edit-original").Amount.Amount != -9007199254740993 {
		t.Fatalf("decimal lost precision: %s / %s", preview, out)
	}
}

func TestAssistantExpectedUpdateRejectsStaleRowsAndReferences(t *testing.T) {
	for _, kind := range []string{"transaction", "account", "category", "member"} {
		t.Run(kind, func(t *testing.T) {
			app, tool := assistantUpdateFixture(t)
			member := domain.Member{ID: "member", Name: "Person"}
			if err := app.PutMember(member); err != nil {
				t.Fatal(err)
			}
			row := assistantUpdateRow(t, app, "edit-original")
			row.CategoryID = "food"
			row.SetOwners([]string{member.ID})
			if err := app.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			raw := json.RawMessage(`{"id":"edit-original","payee":"Reviewed shop"}`)
			if preview := tool.preview(raw); strings.Contains(preview, "Can't prepare") {
				t.Fatal(preview)
			}
			switch kind {
			case "transaction":
				row.Desc = "Concurrent correction"
				if err := app.PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			case "account":
				a := app.Accounts()[0]
				a.Name = "Changed"
				if err := app.PutAccount(a); err != nil {
					t.Fatal(err)
				}
			case "category":
				c := app.Categories()[0]
				c.Name = "Changed"
				if err := app.PutCategory(c); err != nil {
					t.Fatal(err)
				}
			case "member":
				member.Name = "Changed"
				if err := app.PutMember(member); err != nil {
					t.Fatal(err)
				}
			}
			before, rev := app.Transactions(), app.Rev()
			if out := tool.run(raw); !strings.Contains(out, "Couldn't save") || app.Rev() != rev || !reflect.DeepEqual(before, app.Transactions()) {
				t.Fatalf("stale %s wrote: %s", kind, out)
			}
		})
	}
}

func TestAssistantExpectedUpdateHonorsLiabilityFlowAndFullFields(t *testing.T) {
	for _, sign := range []domain.LiabilitySign{domain.LiabilityPositiveOwed, domain.LiabilityNegativeOwed} {
		t.Run(string(sign), func(t *testing.T) {
			app, tool := assistantUpdateFixture(t)
			opening, booked := int64(10000), int64(100)
			if sign == domain.LiabilityNegativeOwed {
				opening, booked = -10000, -100
			}
			account := domain.Account{ID: "card", Name: "Card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(opening, "USD"), LiabilitySign: sign}
			if err := app.PutAccount(account); err != nil {
				t.Fatal(err)
			}
			row := assistantCreationRow("card-row", "Card shop")
			row.AccountID = account.ID
			row.Amount = money.New(booked, "USD")
			if err := app.PutTransaction(row); err != nil {
				t.Fatal(err)
			}
			preview, out := assistantCreationApprove(tool, `{"id":"card-row","amount":"12.34","payee":"Updated shop","description":"Reviewed description","category_id":"food","date":"2026-10-19","cleared":true,"tags":["a","b"]}`)
			want := int64(1234)
			if sign == domain.LiabilityNegativeOwed {
				want = -1234
			}
			for _, text := range []string{"public flow -12.34 USD", "negative public flow is a charge", "Reviewed description", "Updated shop", "2026-10-19", "food", "cleared"} {
				if !strings.Contains(preview, text) {
					t.Fatalf("incomplete preview lacks %q: %s", text, preview)
				}
			}
			got := assistantUpdateRow(t, app, row.ID)
			if !strings.Contains(out, "Updated") || got.Amount.Amount != want || !got.Cleared || got.ClearedAt.IsZero() || got.CategoryID != "food" || !reflect.DeepEqual(got.Tags, []string{"a", "b"}) {
				t.Fatalf("wrong effective update: %+v / %s", got, out)
			}
			preview, out = assistantCreationApprove(tool, `{"id":"card-row","direction":"income"}`)
			if !strings.Contains(preview, "public flow 12.34 USD") || !strings.Contains(out, "Updated") || assistantUpdateRow(t, app, row.ID).Amount.Amount != -want {
				t.Fatalf("payment reversed: %s / %s", preview, out)
			}
		})
	}
}

func TestAssistantExpectedUpdateCanonicalTransferPairFeeAndStalePartner(t *testing.T) {
	for _, stale := range []string{"none", "partner", "fee", "extra group row"} {
		t.Run(stale, func(t *testing.T) {
			app, tool := assistantUpdateFixture(t)
			other := app.Accounts()[0]
			other.ID = "other"
			other.Name = "Other"
			if err := app.PutAccount(other); err != nil {
				t.Fatal(err)
			}
			outID, inID, err := app.CreateTransferPair(appstate.TransferParams{OperationID: "edit-transfer", FromAccountID: "bank", ToAccountID: "other", AmountMinor: 1000, FeeMinor: 25, Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatal(err)
			}
			feeID := ""
			for _, row := range app.Transactions() {
				if strings.HasSuffix(row.ID, "-fee") {
					feeID = row.ID
				}
			}
			raw := json.RawMessage(`{"id":"` + outID + `","amount":"20.00","date":"2026-10-21"}`)
			preview := tool.preview(raw)
			for _, id := range []string{outID, inID, feeID} {
				if id == "" || !strings.Contains(preview, id) {
					t.Fatalf("full transfer preview omits %s: %s", id, preview)
				}
			}
			switch stale {
			case "partner", "fee":
				id := inID
				if stale == "fee" {
					id = feeID
				}
				row := assistantUpdateRow(t, app, id)
				row.Desc = "Concurrent edit"
				if err := app.PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			case "extra group row":
				row := assistantCreationRow("extra-fee", "Extra")
				row.TransferGroupID = assistantUpdateRow(t, app, outID).TransferGroupID
				if err := app.PutTransaction(row); err != nil {
					t.Fatal(err)
				}
			}
			before, rev := app.Transactions(), app.Rev()
			out := tool.run(raw)
			if stale != "none" {
				if !strings.Contains(out, "Couldn't save") || app.Rev() != rev || !reflect.DeepEqual(before, app.Transactions()) {
					t.Fatalf("stale pair wrote: %s", out)
				}
				return
			}
			if !strings.Contains(out, "Updated exact transaction ID "+outID) || assistantUpdateRow(t, app, outID).Amount.Amount != -2000 || assistantUpdateRow(t, app, inID).Amount.Amount != 2000 || assistantUpdateRow(t, app, feeID).Amount.Amount != -25 || assistantUpdateRow(t, app, inID).Date.Day() != 21 {
				t.Fatalf("paired transfer changed incorrectly: %s", out)
			}
		})
	}
}

func TestAssistantExpectedUpdateUnknownFXAndChangedRatesRefuse(t *testing.T) {
	app, tool := assistantUpdateFixture(t)
	eur := app.Accounts()[0]
	eur.ID = "eur"
	eur.Name = "Euro"
	eur.Currency = "EUR"
	eur.OpeningBalance = money.Zero("EUR")
	if err := app.PutAccount(eur); err != nil {
		t.Fatal(err)
	}
	outID, inID, err := app.CreateTransferPair(appstate.TransferParams{FromAccountID: "eur", ToAccountID: "bank", AmountMinor: 100, ReceivedMinor: 200, Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"id":"` + outID + `","amount":"2.00"}`
	assistantUpdateRejectUnchanged(t, app, tool, raw)
	s := app.Settings()
	s.BaseCurrency = "USD"
	s.FXRates = map[string]float64{"EUR": 2}
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	preview := tool.preview(json.RawMessage(raw))
	if strings.Contains(preview, "Can't prepare") || !strings.Contains(preview, "4.00 USD") {
		t.Fatal(preview)
	}
	s.FXRates = map[string]float64{"EUR": 3}
	if err := app.PutSettings(s); err != nil {
		t.Fatal(err)
	}
	before, rev := app.Transactions(), app.Rev()
	if out := tool.run(json.RawMessage(raw)); !strings.Contains(out, "Couldn't save") || app.Rev() != rev || !reflect.DeepEqual(before, app.Transactions()) {
		t.Fatalf("stale rate repriced pair: %s", out)
	}
	tool = agExpectedUpdateTool(app)
	preview, out := assistantCreationApprove(tool, raw)
	if !strings.Contains(preview, "6.00 USD") || !strings.Contains(out, "Updated") || assistantUpdateRow(t, app, inID).Amount.Amount != 600 {
		t.Fatalf("fresh canonical FX failed: %s / %s", preview, out)
	}
}

func TestAssistantExpectedUpdateSplitConsistencyAndExactReferenceAmbiguity(t *testing.T) {
	app, tool := assistantUpdateFixture(t)
	row := assistantUpdateRow(t, app, "edit-original")
	row.Splits = []domain.CategorySplit{{CategoryID: "food", Amount: row.Amount}}
	if err := app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	assistantUpdateRejectUnchanged(t, app, tool, `{"id":"edit-original","amount":"2.00"}`)
	if _, out := assistantCreationApprove(tool, `{"id":"edit-original","description":"Split metadata"}`); !strings.Contains(out, "Updated") {
		t.Fatal(out)
	}
	if err := app.PutCategory(domain.Category{ID: "parent", Name: "Parent", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "food2", Name: "Food", Kind: domain.KindExpense, ParentID: "parent"}); err != nil {
		t.Fatal(err)
	}
	assistantUpdateRejectUnchanged(t, app, tool, `{"id":"edit-original","category":"Food"}`)
	if _, out := assistantCreationApprove(tool, `{"id":"edit-original","category_id":"food2"}`); !strings.Contains(out, "Updated") {
		t.Fatal(out)
	}
}
