// SPDX-License-Identifier: MIT

package appstate

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/provisional"
)

func TestExpectedAccountReconciliationCreatesCheckpointInStatedConvention(t *testing.T) {
	clock := time.Date(2026, 10, 31, 9, 30, 0, 0, time.UTC)
	a := newTestAppAt(t, clock)
	account := domain.Account{ID: "balance", Name: "Card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(50000, "USD")}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	asOf := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	preview, err := a.PreviewAccountReconciliation(account.ID, -65000, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CurrentStatedMinor != -50000 || preview.AdjustmentStatedMinor != -15000 || !preview.NeedsAdjustment {
		t.Fatalf("preview = %+v", preview)
	}
	checkpoint, err := a.CommitAccountReconciliation(preview, "Statement balance")
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Amount.Amount != 15000 || !checkpoint.ExcludeFromReports || !checkpoint.Date.Equal(asOf) {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
	saved, ok, err := a.Store().GetAccount(account.ID)
	if err != nil || !ok || saved.OpeningBalance.Amount != account.OpeningBalance.Amount || !saved.BalanceAsOf.Equal(clock) {
		t.Fatalf("saved account = %+v, ok=%v, err=%v", saved, ok, err)
	}
}

func TestExpectedAccountReconciliationZeroDeltaDoesNotRewriteTimestamp(t *testing.T) {
	a := newApp(t, false)
	account := domain.Account{ID: "zero", Name: "Cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: money.New(1234, "USD")}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	before, _, _ := a.Store().GetAccount(account.ID)
	preview, err := a.PreviewAccountReconciliation(account.ID, 1234, time.Now().AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CommitAccountReconciliation(preview, ""); err != nil {
		t.Fatal(err)
	}
	after, _, _ := a.Store().GetAccount(account.ID)
	if len(a.Transactions()) != 0 || !after.BalanceAsOf.Equal(before.BalanceAsOf) {
		t.Fatalf("no-op changed data: rows=%d asOf=%v (before %v)", len(a.Transactions()), after.BalanceAsOf, before.BalanceAsOf)
	}
}

func TestExpectedAccountReconciliationReplacesAndRemovesCheckpointFromRawLedger(t *testing.T) {
	a := newApp(t, false)
	account := domain.Account{ID: "recheck", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(10000, "USD")}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	for _, row := range []domain.Transaction{
		{ID: "old-one", AccountID: account.ID, Amount: money.New(2000, "USD"), Date: time.Now(), Desc: "old checkpoint", BalanceCheckpointAt: time.Now(), ExcludeFromReports: true, Tags: []string{"reconciliation-checkpoint"}},
		{ID: "old-two", AccountID: account.ID, Amount: money.New(500, "USD"), Date: time.Now(), Desc: "older checkpoint", BalanceCheckpointAt: time.Now(), ExcludeFromReports: true, Tags: []string{"reconciliation-checkpoint"}},
	} {
		if err := a.Store().PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	asOf := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	preview, err := a.PreviewAccountReconciliation(account.ID, 15000, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CurrentStatedMinor != 12500 || preview.AdjustmentStatedMinor != 2500 ||
		preview.CheckpointAmountMinor != 5000 || preview.CheckpointCount != 2 || preview.CheckpointAction != "replace" {
		t.Fatalf("replacement preview = %+v", preview)
	}
	if _, err := a.CommitAccountReconciliation(preview, "new checkpoint"); err != nil {
		t.Fatal(err)
	}
	if got := balanceOf(t, a, account); got != 15000 {
		t.Fatalf("balance after replacement = %d, want 15000", got)
	}
	checkpoints := provisional.Of(account.ID, a.Transactions())
	if len(checkpoints) != 1 || checkpoints[0].Amount.Amount != 5000 {
		t.Fatalf("checkpoints after replacement = %+v", checkpoints)
	}
	remove, err := a.PreviewAccountReconciliation(account.ID, 10000, asOf.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if remove.CheckpointAction != "remove" || remove.CheckpointAmountMinor != 0 {
		t.Fatalf("remove preview = %+v", remove)
	}
	if checkpoint, err := a.CommitAccountReconciliation(remove, ""); err != nil || checkpoint.ID != "" {
		t.Fatalf("remove commit = %+v, %v", checkpoint, err)
	}
	if got := balanceOf(t, a, account); got != 10000 || len(provisional.Of(account.ID, a.Transactions())) != 0 {
		t.Fatalf("balance/checkpoints after removal = %d/%+v", got, provisional.Of(account.ID, a.Transactions()))
	}
}

func TestExpectedAccountReconciliationNegativeOwedStorageConvention(t *testing.T) {
	a := newApp(t, false)
	account := domain.Account{ID: "negative-card", Name: "Imported card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(-50000, "USD")}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	preview, err := a.PreviewAccountReconciliation(account.ID, -65000, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if preview.CurrentStatedMinor != -50000 || preview.AdjustmentStatedMinor != -15000 || preview.CheckpointAmountMinor != -15000 {
		t.Fatalf("negative-owed preview = %+v", preview)
	}
	if _, err := a.CommitAccountReconciliation(preview, ""); err != nil {
		t.Fatal(err)
	}
	if got := balanceOf(t, a, account); got != -65000 {
		t.Fatalf("negative-owed balance = %d, want -65000", got)
	}
}

func TestExpectedAccountReconciliationRejectsStaleLedgerAndOverflow(t *testing.T) {
	a := newApp(t, false)
	account := domain.Account{ID: "stale", Name: "Cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: money.New(100, "USD")}
	if err := a.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	preview, err := a.PreviewAccountReconciliation(account.ID, 200, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.PutTransaction(domain.Transaction{ID: "new-row", AccountID: account.ID, Amount: money.New(10, "USD"), Date: time.Now(), Desc: "New row"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CommitAccountReconciliation(preview, ""); err == nil {
		t.Fatal("stale ledger was accepted")
	}
	if len(a.Transactions()) != 1 {
		t.Fatalf("stale commit changed ledger: %d rows", len(a.Transactions()))
	}

	maxAccount := domain.Account{ID: "overflow", Name: "Large", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")}
	if err := a.PutAccount(maxAccount); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PreviewAccountReconciliation(maxAccount.ID, math.MinInt64, time.Now()); err == nil {
		t.Fatal("overflowing displayed adjustment was accepted")
	}
}

func TestExpectedTransferRequiresUnchangedFXSettingsAndMatchesPreview(t *testing.T) {
	a := newApp(t, false)
	from, to := seedFXPair(t, a)
	p, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "expected-fx", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if p.Balances.ToAfter.Amount != 15000 {
		t.Fatalf("FX preview destination = %d, want 15000", p.Balances.ToAfter.Amount)
	}
	outID, inID, err := a.CreateTransferPairExpected(p)
	if err != nil {
		t.Fatal(err)
	}
	if outID == "" || inID == "" || len(a.Transactions()) != 2 {
		t.Fatalf("transfer = %q/%q, rows=%d", outID, inID, len(a.Transactions()))
	}
	if got := balanceOf(t, a, to); got != p.Balances.ToAfter.Amount {
		t.Fatalf("destination booked = %d, preview = %d", got, p.Balances.ToAfter.Amount)
	}

	stale, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "expected-fx-stale", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 1000})
	if err != nil {
		t.Fatal(err)
	}
	settings := a.Settings()
	settings.FXRates = map[string]float64{"EUR": 3}
	if err := a.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	before := len(a.Transactions())
	if _, _, err := a.CreateTransferPairExpected(stale); err == nil {
		t.Fatal("transfer with stale FX settings was accepted")
	}
	if len(a.Transactions()) != before {
		t.Fatalf("stale FX commit wrote rows: %d -> %d", before, len(a.Transactions()))
	}
}

func TestExpectedTransferPreviewIsImmutableAtApprovalBoundary(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 0)
	preview, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "immutable", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 1000})
	if err != nil {
		t.Fatal(err)
	}
	preview.Params.AmountMinor = 2000
	if _, _, err := a.CreateTransferPairExpected(preview); err == nil {
		t.Fatal("mutated preview was accepted")
	}
	if len(a.Transactions()) != 0 {
		t.Fatalf("mutated preview wrote %d rows", len(a.Transactions()))
	}
}

func TestExpectedTransferRetryWithOperationIDIsIdempotent(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 0)
	preview, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "assistant_retry_1", Source: domain.TxnSourceAssistant,
		FromAccountID: from.ID, ToAccountID: to.ID, AmountMinor: 1000})
	if err != nil {
		t.Fatal(err)
	}
	outID, inID, err := a.CreateTransferPairExpected(preview)
	if err != nil {
		t.Fatal(err)
	}
	revision := a.Rev()
	retryOut, retryIn, err := a.CreateTransferPairExpected(preview)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retryOut != outID || retryIn != inID || len(a.Transactions()) != 2 || a.Rev() != revision {
		t.Fatalf("retry = %q/%q rows=%d rev=%d (before %d)", retryOut, retryIn, len(a.Transactions()), a.Rev(), revision)
	}
}

func TestExpectedTransferLiabilitySourcePreviewMatchesPostedFeeLeg(t *testing.T) {
	a := newApp(t, false)
	from := domain.Account{ID: "loan", Name: "Loan card", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD", OpeningBalance: money.New(50000, "USD")}
	to := domain.Account{ID: "cash", Name: "Cash", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeCash, Currency: "USD", OpeningBalance: money.New(10000, "USD")}
	if err := a.PutAccount(from); err != nil {
		t.Fatal(err)
	}
	if err := a.PutAccount(to); err != nil {
		t.Fatal(err)
	}
	preview, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "liability-source-fee", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 10000, FeeMinor: 125})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.CreateTransferPairExpected(preview); err != nil {
		t.Fatal(err)
	}
	if got := balanceOf(t, a, from); got != preview.Balances.FromAfter.Amount {
		t.Fatalf("source booked = %d, preview = %d", got, preview.Balances.FromAfter.Amount)
	}
	if got := balanceOf(t, a, to); got != preview.Balances.ToAfter.Amount {
		t.Fatalf("destination booked = %d, preview = %d", got, preview.Balances.ToAfter.Amount)
	}
}

func TestExpectedAccountReconciliationStampsTheClockAndKeepsTheStatementDateOnTheCheckpoint(t *testing.T) {
	clock := time.Date(2026, 10, 31, 9, 30, 0, 0, time.UTC)
	a := newTestAppAt(t, clock)
	if err := a.PutAccount(acctCovAsset("chk", "Checking", 10000, time.Time{})); err != nil {
		t.Fatal(err)
	}
	statement := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	preview, err := a.PreviewAccountReconciliation("chk", 12000, statement)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := a.CommitAccountReconciliation(preview, "Statement")
	if err != nil {
		t.Fatal(err)
	}
	if !checkpoint.Date.Equal(statement) {
		t.Fatalf("checkpoint dated %v, want the statement date %v", checkpoint.Date, statement)
	}
	if got := storedAccount(a, "chk").BalanceAsOf; !got.Equal(clock) {
		t.Fatalf("BalanceAsOf = %v, want the clock %v", got, clock)
	}
}

func TestExpectedAccountReconciliationRefusesAnAccountThatDoesNotValidate(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAccount(acctCovAsset("chk", "Checking", 10000, time.Time{})); err != nil {
		t.Fatal(err)
	}
	broken := storedAccount(a, "chk")
	broken.Name = ""
	if err := a.Store().PutAccount(broken); err != nil {
		t.Skipf("store refuses an unnamed account: %v", err)
	}
	preview, err := a.PreviewAccountReconciliation("chk", 12000, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CommitAccountReconciliation(preview, "Statement"); err == nil {
		t.Fatal("a balance update was committed for an account that fails validation")
	}
	if n := len(a.Transactions()); n != 0 {
		t.Fatalf("a refused balance update wrote %d rows", n)
	}
}

func TestExpectedTransferIgnoresUnrelatedSettingsWrites(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 0)
	preview, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "unrelated-settings", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 1000})
	if err != nil {
		t.Fatal(err)
	}
	settings := a.Settings()
	settings.FXUpdatedAt = map[string]time.Time{"EUR": time.Now()}
	settings.FlexBudget = 12345
	if err := a.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.CreateTransferPairExpected(preview); err != nil {
		t.Fatalf("an unrelated settings write failed the transfer: %v", err)
	}
	if n := len(a.Transactions()); n != 2 {
		t.Fatalf("transfer wrote %d rows, want 2", n)
	}
}

func TestExpectedTransferReservesTheOperationReceiptWithTheRows(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 0)
	preview, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "receipt-op", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.CreateTransferPairExpected(preview); err != nil {
		t.Fatal(err)
	}
	receipts, err := a.Store().ListAllTransferReceipts()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range receipts {
		found = found || r.ID == "receipt-op"
	}
	if !found {
		t.Fatalf("receipts = %+v, want one for receipt-op", receipts)
	}
	// The ordinary path treats the same operation and payload as a retry, not a second transfer.
	if _, _, err := a.CreateTransferPair(preview.Params); err != nil {
		t.Fatalf("ordinary retry of the same operation: %v", err)
	}
	if n := len(a.Transactions()); n != 2 {
		t.Fatalf("%d rows after the retry, want 2", n)
	}
	other := preview.Params
	other.AmountMinor = 2000
	if _, _, err := a.CreateTransferPair(other); err == nil {
		t.Fatal("the operation ID was reused for a different transfer")
	}
}

func TestExpectedTransferTreatsABlankOperationIDAsMissing(t *testing.T) {
	a := newApp(t, false)
	from, to := seedAssetPair(t, a, 100000, 0)
	preview, err := a.PreviewExpectedTransfer(TransferParams{OperationID: "   ", FromAccountID: from.ID,
		ToAccountID: to.ID, AmountMinor: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(preview.Params.OperationID) == "" {
		t.Fatal("a blank operation ID was kept")
	}
}
