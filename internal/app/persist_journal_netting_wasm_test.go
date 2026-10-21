// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func TestJRNLHydrationImportsRefundNettingForParsedAndReplayPaths(t *testing.T) {
	previousApp := appstate.Default
	t.Cleanup(func() {
		if previousApp != nil {
			previousApp.SyncTxnLinkNetting()
		} else {
			reports.SetRefundLinks(nil)
		}
	})
	ds := jrnlDataset()
	if len(ds.Transactions) < 2 {
		t.Fatal("fixture needs two transactions")
	}
	purchaseID, refundID := ds.Transactions[0].ID, ds.Transactions[1].ID
	ds.Transactions[0].Date = time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	ds.Transactions[0].Amount = money.New(-12000, "USD")
	ds.Transactions[0].CategoryID = "clothing"
	ds.Transactions[1].Date = time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC)
	ds.Transactions[1].Amount = money.New(4000, "USD")
	ds.Transactions[1].CategoryID = "clothing"
	ds.TxnLinks = []domain.TxnLink{{
		ID: "hydration-refund", Kind: domain.TxnLinkRefundPair,
		TxnIDs: []string{purchaseID, refundID}, Amount: money.New(4000, "USD"),
	}}

	f := solp22JournalDataset(t, ds, jrnlFixtureWorkspace)
	// The fixture's normal journal snapshot exercises replay, which must discard
	// the checkpoint's parsed value and import the replayed portable dataset.
	for _, tc := range []struct {
		name     string
		snapshot durableDatasetSnapshot
	}{
		{
			name: "parsed checkpoint",
			snapshot: durableDatasetSnapshot{
				SecureSnapshot: browserstore.SecureSnapshot{
					Values: map[string]string{
						datasetStoreKey: f.checkpoint,
						datasetGenKey:   "",
					},
					Errors: map[string]error{},
				},
				workspace: jrnlFixtureWorkspace,
			},
		},
		{name: "journal replay", snapshot: jrnlRead(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, parsed, _, err := recoverDatasetSnapshotParsed(tc.snapshot)
			if err != nil {
				t.Fatalf("recover dataset snapshot: %v", err)
			}
			wantParsed := tc.name == "parsed checkpoint"
			if (parsed != nil) != wantParsed {
				t.Fatalf("recovery parsed checkpoint = %v, want %v", parsed != nil, wantParsed)
			}
			reports.SetRefundLinks(nil)
			appstate.Default = f.app
			if err := hydrateDurableDataset(f.app, tc.snapshot); err != nil {
				t.Fatalf("hydrate durable dataset: %v", err)
			}
			assertRefundNettingOverlay(t, f.app)
		})
	}
}

func assertRefundNettingOverlay(t *testing.T, app *appstate.App) {
	t.Helper()
	rates := currency.Rates{Base: "USD"}
	march, err := reports.IncomeVsExpense(
		app.Transactions(),
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC), rates,
	)
	if err != nil || march.Expense != 8000 || march.Income != 0 {
		t.Fatalf("March flow = %+v, err=%v; want $80 expense and no income", march, err)
	}
	april, err := reports.IncomeVsExpense(
		app.Transactions(),
		time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC), rates,
	)
	if err != nil || april.Expense != 0 || april.Income != 0 {
		t.Fatalf("April flow = %+v, err=%v; want no income or expense", april, err)
	}
}
