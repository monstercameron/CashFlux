//go:build js && wasm

package browserstore_test

import (
	"io"
	"strings"

	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	. "github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// This runs the production sample model and storage adapter in the Node WASM
// lane. It is a storage integration proof, not a rendered browser/e2e test.
func TestSampleHouseholdSessionLeavesZeroPlaintext(t *testing.T) {
	SetupSecureForTest(t)
	a, err := appstate.New(io.Discard, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Store().Close() })
	priorDefault := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { uistate.FlushPendingSettingsPersist(); appstate.Default = priorDefault })
	accounts := a.Accounts()
	if len(accounts) == 0 {
		t.Fatal("sample household has no accounts")
	}
	checkpointDone := make(chan error, 1)
	uistate.SaveCheckpointThen("Before Blue Bottle ENC3 edit", func(id string, err error) {
		if err == nil && !uistate.HasCheckpoint(id) {
			t.Error("checkpoint callback ran before the index committed")
		}
		checkpointDone <- err
	})
	select {
	case err := <-checkpointDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("checkpoint did not commit")
	}
	txn := domain.Transaction{
		ID: "enc3-scripted-transaction", AccountID: accounts[0].ID,
		Date:  time.Date(2026, time.October, 18, 12, 0, 0, 0, time.UTC),
		Payee: "Blue Bottle ENC3", Desc: "1,850 private sample receipt",
		Amount: money.New(-185000, accounts[0].Currency),
	}
	if err := a.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	txn.Amount.Amount = -185001
	txn.Desc = "1,850 edited sample receipt"
	txn.Cleared = true
	if err := a.PutTransaction(txn); err != nil {
		t.Fatal(err)
	}
	// Exercise the production review, search and notification persistence paths,
	// then exercise every audited side-copy family with their serialized state.
	uistate.PersistTxFilter(uistate.TxFilter{Text: "Blue Bottle ENC3 1,850"})
	uistate.DismissReviewItem(txn.ID)
	uistate.PersistNotifyFeed([]uistate.FeedItem{{ID: "enc3-proof", Title: "Blue Bottle ENC3 reviewed: 1,850"}})
	uistate.KVSet("cashflux:notifhistory", `[{"message":"Blue Bottle ENC3 reviewed"}]`)
	uistate.SettingKVSet("cashflux:learn-tally", `{"Blue Bottle ENC3":"Dining"}`)
	state := map[string]string{
		"cashflux:tx-filter":          uistate.KVGet("cashflux:tx-filter"),
		"cashflux:review-resolutions": uistate.SettingKVGet("cashflux:review-resolutions"),
		"cashflux:notify:feed":        uistate.KVGet("cashflux:notify:feed"),
		"cashflux:notifhistory":       uistate.KVGet("cashflux:notifhistory"),
		"cashflux:learn-tally":        uistate.SettingKVGet("cashflux:learn-tally"),
	}
	exported, err := a.ExportJSONRedacted()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exported), "Blue Bottle ENC3") {
		t.Fatal("sentinel was never serialized")
	}
	for _, key := range []string{
		"cashflux:dataset", "cashflux:checkpoint-data:proof", "cashflux:checkpoints",
		"cashflux:sync-queue", "cashflux:sync-conflict:proof", "cashflux:quarantine:proof",
		"cashflux:ws-data:proof", "cashflux:import-wip", "cashflux:reconcile:draft:proof",
		"cashflux:artifact:proof", "cashflux:member-pins", "cashflux:cred:openai",
	} {
		PutSecureForTest(t, key, string(exported))
	}
	for key, value := range state {
		PutSecureForTest(t, key, value)
	}
	Set("cashflux:auth:access-token", "enc3-access-secret")
	Set("cashflux:auth:refresh-token", "enc3-refresh-secret")
	Barrier()
	AssertNoPlaintextForTest(t, append([]string{"Blue Bottle", "1,850", "enc3-access-secret", "enc3-refresh-secret"}, accounts[0].Name))
	// A fresh KeyService instance must decrypt the same data without App Lock.
	if err := ReopenSecureForTest(); err != nil {
		t.Fatal(err)
	}
	if got, _, err := GetSecure("cashflux:dataset"); err != nil || got != string(exported) {
		t.Fatal("reload lost sample state", err)
	}
	reloaded, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reloaded.Store().Close() })
	if err := reloaded.ImportJSON([]byte(GetString("cashflux:dataset"))); err != nil {
		t.Fatal(err)
	}
	appstate.Default = reloaded
	if uistate.CurrentTxFilter().Text != "Blue Bottle ENC3 1,850" {
		t.Fatal("reload lost search")
	}
	if len(uistate.LoadReviewResolutions()) == 0 {
		t.Fatal("reload lost review resolution")
	}
	if !strings.Contains(uistate.KVGet("cashflux:notify:feed"), "Blue Bottle ENC3") {
		t.Fatal("reload lost notifications")
	}
	if err := EnableSecureForTest("enc3-test-passcode"); err != nil {
		t.Fatal(err)
	}
	LockSecure()
	AssertNoPlaintextForTest(t, []string{"Blue Bottle", "1,850", accounts[0].Name})
	if err := UnlockSecure("enc3-test-passcode"); err != nil {
		t.Fatal(err)
	}
	if GetString("cashflux:tx-filter") != state["cashflux:tx-filter"] {
		t.Fatal("locked reload lost saved search")
	}
	AssertNoPlaintextForTest(t, []string{"Blue Bottle", "1,850", accounts[0].Name})
}
