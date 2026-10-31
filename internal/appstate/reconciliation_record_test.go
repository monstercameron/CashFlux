// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// asViewer makes every later mutation on app fail the read-only guard.
func asViewer(app *App) {
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleViewer })
}

func acctCovAsset(id, name string, openingMinor int64, asOf time.Time) domain.Account {
	return domain.Account{
		ID: id, Name: name, Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		OpeningBalance: money.New(openingMinor, "USD"), BalanceAsOf: asOf,
	}
}

// acctCovCheckpoint stores a provisional balance checkpoint on the account dated at.
func acctCovCheckpoint(t *testing.T, app *App, id, accountID string, deltaMinor int64, at time.Time) {
	t.Helper()
	tx, ok := provisional.New(id, accountID, "Checkpoint", 0, deltaMinor, "USD", at, at)
	if !ok {
		t.Fatalf("checkpoint %s: zero delta", id)
	}
	if err := app.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction %s: %v", id, err)
	}
}

func TestRecordReconciliationSavesHistoryAndRetiresCoveredCheckpoints(t *testing.T) {
	clock := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, clock)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, time.Time{})); err != nil {
		t.Fatal(err)
	}
	acctCovCheckpoint(t, app, "old-guess", "chk", 2000, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	acctCovCheckpoint(t, app, "later-guess", "chk", 500, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))

	ev := domain.Reconciliation{
		At: clock, StatementDate: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC),
		StatementBalance: money.New(102000, "USD"),
	}
	out, err := app.RecordReconciliation("chk", ev)
	if err != nil {
		t.Fatalf("RecordReconciliation: %v", err)
	}
	if out.Retired != 1 {
		t.Fatalf("outcome = %+v, want one checkpoint retired", out)
	}

	account, ok, err := app.Store().GetAccount("chk")
	if err != nil || !ok {
		t.Fatalf("GetAccount: ok=%v err=%v", ok, err)
	}
	if len(account.Reconciliations) != 1 || account.Reconciliations[0].StatementBalance.Amount != 102000 {
		t.Fatalf("history = %+v, want the one recorded reconciliation", account.Reconciliations)
	}
	if !account.BalanceAsOf.Equal(clock) {
		t.Fatalf("BalanceAsOf = %v, want the clock %v", account.BalanceAsOf, clock)
	}
	// Only the checkpoint after the statement date survives.
	left := provisional.Of("chk", app.Transactions())
	if len(left) != 1 || left[0].ID != "later-guess" {
		t.Fatalf("checkpoints left = %+v, want only later-guess", left)
	}
}

func TestRecordReconciliationUsesTheRecordingTimeWhenNoStatementDate(t *testing.T) {
	clock := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, clock)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, time.Time{})); err != nil {
		t.Fatal(err)
	}
	acctCovCheckpoint(t, app, "guess", "chk", 2000, time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC))
	out, err := app.RecordReconciliation("chk", domain.Reconciliation{At: clock, StatementBalance: money.New(102000, "USD")})
	if err != nil || out.Retired != 1 {
		t.Fatalf("RecordReconciliation = %+v, %v; want the checkpoint before At retired", out, err)
	}
}

func TestRecordReconciliationKeepsOnlyTheNewestThirtySixEvents(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC))
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 0, time.Time{})); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < reconcile.MaxHistory+4; i++ {
		ev := domain.Reconciliation{At: time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC), StatementBalance: money.New(int64(i), "USD")}
		if _, err := app.RecordReconciliation("chk", ev); err != nil {
			t.Fatalf("RecordReconciliation %d: %v", i, err)
		}
	}
	account, _, _ := app.Store().GetAccount("chk")
	if n := len(account.Reconciliations); n != reconcile.MaxHistory {
		t.Fatalf("history length = %d, want %d", n, reconcile.MaxHistory)
	}
	if first := account.Reconciliations[0].StatementBalance.Amount; first != 4 {
		t.Fatalf("oldest kept balance = %d, want 4 (the four oldest dropped)", first)
	}
}

func TestRecordReconciliationRefusesUnknownAccountAndViewer(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 0, time.Time{})); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RecordReconciliation("ghost", domain.Reconciliation{At: time.Now()}); err == nil {
		t.Fatal("recorded a reconciliation for a missing account")
	}
	asViewer(app)
	if _, err := app.RecordReconciliation("chk", domain.Reconciliation{At: time.Now()}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer RecordReconciliation = %v, want ErrReadOnly", err)
	}
	account, _, _ := app.Store().GetAccount("chk")
	if len(account.Reconciliations) != 0 {
		t.Fatalf("a refused reconciliation was saved: %+v", account.Reconciliations)
	}
}

func TestPreviewWithCategoryFilesTheCheckpointUnderThatCategory(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, time.Time{})); err != nil {
		t.Fatal(err)
	}
	if err := app.PutCategory(domain.Category{ID: "misc", Name: "Miscellaneous", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory: %v", err)
	}
	asOf := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	preview, err := app.PreviewAccountReconciliation("chk", 98000, asOf)
	if err != nil {
		t.Fatal(err)
	}
	filed := preview.WithCategory("misc")
	if filed.CategoryID != "misc" || preview.CategoryID != "" {
		t.Fatalf("WithCategory changed the receiver or lost the id: original %q, copy %q", preview.CategoryID, filed.CategoryID)
	}
	checkpoint, err := app.CommitAccountReconciliation(filed, "Statement balance")
	if err != nil {
		t.Fatalf("CommitAccountReconciliation: %v", err)
	}
	if checkpoint.CategoryID != "misc" || checkpoint.Amount.Amount != -2000 {
		t.Fatalf("checkpoint = %+v, want category misc and amount -2000", checkpoint)
	}
}

func TestPreviewAndCommitAccountReconciliationRefusals(t *testing.T) {
	asOf := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	newFixture := func(t *testing.T) *App {
		t.Helper()
		app := newApp(t, false)
		if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, time.Time{})); err != nil {
			t.Fatal(err)
		}
		archived := acctCovAsset("old", "Old", 0, time.Time{})
		archived.Archived = true
		if err := app.PutAccount(archived); err != nil {
			t.Fatal(err)
		}
		return app
	}

	t.Run("preview refuses a blank, unknown or archived account", func(t *testing.T) {
		app := newFixture(t)
		for _, id := range []string{"", "ghost", "old"} {
			if _, err := app.PreviewAccountReconciliation(id, 1000, asOf); err == nil {
				t.Errorf("preview for account %q was accepted", id)
			}
		}
	})

	t.Run("a preview with no asOf date uses the clock", func(t *testing.T) {
		clock := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
		app := newTestAppAt(t, clock)
		if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, time.Time{})); err != nil {
			t.Fatal(err)
		}
		preview, err := app.PreviewAccountReconciliation("chk", 90000, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		if preview.AsOf.Year() != 2026 || preview.AsOf.Month() != time.October || preview.AsOf.Day() != 31 {
			t.Fatalf("AsOf = %v, want the clock's day", preview.AsOf)
		}
		if preview.CheckpointAction != "add" || preview.CheckpointAmountMinor != -10000 {
			t.Fatalf("preview = %+v, want an add of -10000", preview)
		}
	})

	t.Run("commit refuses a hand-built preview", func(t *testing.T) {
		app := newFixture(t)
		if _, err := app.CommitAccountReconciliation(AccountReconciliationPreview{}, ""); err == nil {
			t.Fatal("an empty preview was committed")
		}
		preview, err := app.PreviewAccountReconciliation("chk", 90000, asOf)
		if err != nil {
			t.Fatal(err)
		}
		preview.CheckpointAmountMinor = 1 // tampered after the preview was built
		if _, err := app.CommitAccountReconciliation(preview, ""); err == nil {
			t.Fatal("a tampered preview was committed")
		}
		if n := len(app.Transactions()); n != 0 {
			t.Fatalf("%d rows written by refused commits", n)
		}
	})

	t.Run("commit refuses when the account changed after the preview", func(t *testing.T) {
		app := newFixture(t)
		preview, err := app.PreviewAccountReconciliation("chk", 90000, asOf)
		if err != nil {
			t.Fatal(err)
		}
		renamed := acctCovAsset("chk", "Renamed", 100000, time.Time{})
		if err := app.PutAccount(renamed); err != nil {
			t.Fatal(err)
		}
		if _, err := app.CommitAccountReconciliation(preview, ""); err == nil {
			t.Fatal("a preview of an edited account was committed")
		}
	})

	t.Run("commit refuses a viewer", func(t *testing.T) {
		app := newFixture(t)
		preview, err := app.PreviewAccountReconciliation("chk", 90000, asOf)
		if err != nil {
			t.Fatal(err)
		}
		asViewer(app)
		if _, err := app.CommitAccountReconciliation(preview, ""); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("viewer commit = %v, want ErrReadOnly", err)
		}
	})

	t.Run("a second preview replaces the open checkpoint", func(t *testing.T) {
		app := newFixture(t)
		first, _ := app.PreviewAccountReconciliation("chk", 90000, asOf)
		if _, err := app.CommitAccountReconciliation(first, "first"); err != nil {
			t.Fatal(err)
		}
		second, err := app.PreviewAccountReconciliation("chk", 95000, asOf.AddDate(0, 0, 1))
		if err != nil {
			t.Fatal(err)
		}
		if second.CheckpointAction != "replace" || second.CheckpointCount != 1 || second.CheckpointAmountMinor != -5000 {
			t.Fatalf("second preview = %+v, want replace of one checkpoint with -5000", second)
		}
		if _, err := app.CommitAccountReconciliation(second, "second"); err != nil {
			t.Fatal(err)
		}
		cps := provisional.Of("chk", app.Transactions())
		if len(cps) != 1 || cps[0].Amount.Amount != -5000 {
			t.Fatalf("checkpoints = %+v, want the single replacement of -5000", cps)
		}
	})
}

func TestAccountHasTransactionsCountsTransferLegs(t *testing.T) {
	app := newApp(t, false)
	for _, ac := range []domain.Account{
		acctCovAsset("a", "A", 100000, time.Time{}),
		acctCovAsset("b", "B", 0, time.Time{}),
		acctCovAsset("c", "C", 0, time.Time{}),
	} {
		if err := app.PutAccount(ac); err != nil {
			t.Fatal(err)
		}
	}
	if app.AccountHasTransactions("a") {
		t.Fatal("a fresh account reports transactions")
	}
	if _, _, err := app.CreateTransferPair(TransferParams{FromAccountID: "a", ToAccountID: "b", AmountMinor: 1000}); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{"a": true, "b": true, "c": false, "ghost": false} {
		if got := app.AccountHasTransactions(id); got != want {
			t.Errorf("AccountHasTransactions(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestDeleteAccountRefusals(t *testing.T) {
	app := newApp(t, false)
	for _, ac := range []domain.Account{
		acctCovAsset("a", "A", 100000, time.Time{}),
		acctCovAsset("b", "B", 0, time.Time{}),
		acctCovAsset("c", "C", 0, time.Time{}),
	} {
		if err := app.PutAccount(ac); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := app.CreateTransferPair(TransferParams{FromAccountID: "a", ToAccountID: "b", AmountMinor: 1000}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if err := app.DeleteAccount(id); !errors.Is(err, ErrAccountHasTransactions) {
			t.Errorf("DeleteAccount(%q) = %v, want ErrAccountHasTransactions", id, err)
		}
	}
	if err := app.DeleteAccount("ghost"); err != nil {
		t.Fatalf("deleting an unknown account = %v, want a no-op", err)
	}
	if n := len(app.Accounts()); n != 3 {
		t.Fatalf("%d accounts after refused deletes, want 3", n)
	}
	asViewer(app)
	if err := app.DeleteAccount("c"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer DeleteAccount = %v, want ErrReadOnly", err)
	}
	if n := len(app.Accounts()); n != 3 {
		t.Fatalf("a viewer removed an account: %d left", n)
	}
}

func TestAnchoredBalanceReadsPreferTheNearestSnapshot(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.UTC) }
	app := newApp(t, false)
	// Opening 1,000.00 confirmed on the 1st becomes the first anchor.
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, day(1))); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutBalanceSnapshot(domain.BalanceSnapshot{ID: "snap-10", AccountID: "chk", BalanceMinor: 150000, Currency: "USD", AsOf: day(10)}); err != nil {
		t.Fatal(err)
	}
	for _, tx := range []domain.Transaction{
		{ID: "t5", AccountID: "chk", Date: day(5), Desc: "Deposit", Amount: money.New(2000, "USD")},
		{ID: "t12", AccountID: "chk", Date: day(12), Desc: "Coffee", Amount: money.New(-500, "USD")},
	} {
		if err := app.PutTransaction(tx); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		day  time.Time
		want int64
	}{
		{"before any anchor falls back to the ledger fold", time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), 100000},
		{"first anchor plus later rows", day(7), 102000},
		{"a newer anchor replaces earlier rows", day(11), 150000},
		{"rows after the newest anchor are applied", day(13), 149500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := app.AccountBalanceAsOf("chk", tt.day)
			if !ok || got != tt.want {
				t.Fatalf("AccountBalanceAsOf = %d, %v; want %d", got, ok, tt.want)
			}
		})
	}

	series := app.AccountBalanceSeries("chk", day(9), day(13))
	want := []int64{102000, 150000, 150000, 149500, 149500}
	if len(series) != len(want) {
		t.Fatalf("series has %d points, want %d: %+v", len(series), len(want), series)
	}
	for i, p := range series {
		if p.BalanceMinor != want[i] || !p.Date.Equal(day(9+i)) {
			t.Errorf("point %d = %+v, want %d on day %d", i, p, want[i], 9+i)
		}
	}

	if got := app.AccountBalanceSeries("chk", day(13), day(9)); got != nil {
		t.Errorf("inverted range = %+v, want nil", got)
	}
	if got := app.AccountBalanceSeries("ghost", day(9), day(13)); got != nil {
		t.Errorf("unknown account series = %+v, want nil", got)
	}
	if got, ok := app.AccountBalanceAsOf("ghost", day(9)); ok || got != 0 {
		t.Errorf("unknown account balance = %d, %v; want 0, false", got, ok)
	}
	if _, ok := app.findAccount("chk"); !ok {
		t.Error("findAccount missed an existing account")
	}
	if _, ok := app.findAccount("ghost"); ok {
		t.Error("findAccount found a missing account")
	}
}

func TestPostReconcileAdjustmentDatesRowOnTheClockAndRefusesBadInput(t *testing.T) {
	clock := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, clock)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 0, time.Time{})); err != nil {
		t.Fatal(err)
	}
	posted, err := app.PostReconcileAdjustment("chk", -1250, reconcile.AsStated, "Statement difference")
	if err != nil {
		t.Fatal(err)
	}
	if posted.Date.Year() != 2026 || posted.Date.Month() != time.October || posted.Date.Day() != 31 || posted.Amount.Amount != -1250 {
		t.Fatalf("posted = %+v, want -1250 dated on the clock's day", posted)
	}
	// A transaction must describe itself, so a blank description is refused and nothing is written.
	if _, err := app.PostReconcileAdjustment("chk", 100, reconcile.AsStated, ""); err == nil {
		t.Fatal("an adjustment with no description was posted")
	}
	asViewer(app)
	if _, err := app.PostReconcileAdjustment("chk", 100, reconcile.AsStated, "x"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer adjustment = %v, want ErrReadOnly", err)
	}
	if n := len(app.Transactions()); n != 1 {
		t.Fatalf("%d transactions, want only the first adjustment", n)
	}
}

func TestCreateAccountExpectedRefusals(t *testing.T) {
	base := func(id, name string) domain.Account {
		return domain.Account{ID: id, Name: name, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
			Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.Zero("USD")}
	}
	newFixture := func(t *testing.T) (*App, []domain.Account) {
		t.Helper()
		app := newApp(t, false)
		if err := app.Store().PutSettings(storeSettingsUSD()); err != nil {
			t.Fatal(err)
		}
		if err := app.PutAccount(acctCovAsset("existing", "Household", 0, time.Time{})); err != nil {
			t.Fatal(err)
		}
		archived := acctCovAsset("gone", "Closed", 0, time.Time{})
		archived.Archived = true
		if err := app.PutAccount(archived); err != nil {
			t.Fatal(err)
		}
		live, err := app.Store().ListAccounts()
		if err != nil {
			t.Fatal(err)
		}
		return app, live
	}

	t.Run("blank id", func(t *testing.T) {
		app, live := newFixture(t)
		if _, err := app.CreateAccountExpected(base("", "New"), live, "USD"); err == nil {
			t.Fatal("an account with no id was created")
		}
	})
	t.Run("invalid account", func(t *testing.T) {
		app, live := newFixture(t)
		if _, err := app.CreateAccountExpected(base("new", ""), live, "USD"); err == nil {
			t.Fatal("an account with no name was created")
		}
	})
	t.Run("name already used by an active account, ignoring case and spaces", func(t *testing.T) {
		app, live := newFixture(t)
		if _, err := app.CreateAccountExpected(base("new", "  household "), live, "USD"); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatal("a duplicate account name was created")
		}
	})
	t.Run("name used only by an archived account is allowed", func(t *testing.T) {
		app, live := newFixture(t)
		if _, err := app.CreateAccountExpected(base("new", "Closed"), live, "USD"); err != nil {
			t.Fatalf("reusing an archived account's name: %v", err)
		}
	})
	t.Run("viewer", func(t *testing.T) {
		app, live := newFixture(t)
		asViewer(app)
		if _, err := app.CreateAccountExpected(base("new", "New"), live, "USD"); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("viewer create = %v, want ErrReadOnly", err)
		}
	})
	t.Run("accounts changed since the preview", func(t *testing.T) {
		app, live := newFixture(t)
		if err := app.PutAccount(acctCovAsset("late", "Late", 0, time.Time{})); err != nil {
			t.Fatal(err)
		}
		if _, err := app.CreateAccountExpected(base("new", "New"), live, "USD"); err == nil || !strings.Contains(err.Error(), "accounts changed since preview") {
			t.Fatal("created an account against a stale account list")
		}
	})
	t.Run("opening balance without a date is anchored on the clock", func(t *testing.T) {
		clock := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
		app := newTestAppAt(t, clock)
		if err := app.Store().PutSettings(storeSettingsUSD()); err != nil {
			t.Fatal(err)
		}
		live, _ := app.Store().ListAccounts()
		draft := base("funded", "Funded")
		draft.OpeningBalance = money.New(25000, "USD")
		if _, err := app.CreateAccountExpected(draft, live, "USD"); err != nil {
			t.Fatalf("CreateAccountExpected: %v", err)
		}
		history := app.BalanceHistory("funded")
		if len(history) != 1 || history[0].BalanceMinor != 25000 || !history[0].AsOf.Equal(clock) {
			t.Fatalf("balance history = %+v, want one 25000 anchor on the clock", history)
		}
	})
}

func TestAccountDocumentRefusalsAndRemoval(t *testing.T) {
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

	t.Run("script-capable types are refused before anything is stored", func(t *testing.T) {
		app := newTestAppAt(t, now)
		ac := docTestAccount(t, app)
		for _, mime := range []string{"image/svg+xml", "text/html", "application/xhtml+xml", "text/xml", " Application/XML "} {
			err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "x", MIME: mime, Data: []byte("<x/>")})
			if !errors.Is(err, ErrUnsupportedDocType) {
				t.Errorf("attach %q = %v, want ErrUnsupportedDocType", mime, err)
			}
		}
		if n := len(app.Artifacts()); n != 0 {
			t.Fatalf("%d artifacts stored for refused files", n)
		}
	})

	t.Run("a viewer cannot attach or remove", func(t *testing.T) {
		app := newTestAppAt(t, now)
		ac := docTestAccount(t, app)
		if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf", Data: []byte("x")}); err != nil {
			t.Fatal(err)
		}
		artifactID := app.Artifacts()[0].ID
		asViewer(app)
		if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "b.pdf", MIME: "application/pdf", Data: []byte("x")}); !errors.Is(err, ErrReadOnly) {
			t.Errorf("viewer attach = %v, want ErrReadOnly", err)
		}
		if err := app.RemoveAccountDocument(ac.ID, artifactID); !errors.Is(err, ErrReadOnly) {
			t.Errorf("viewer remove = %v, want ErrReadOnly", err)
		}
		if n := len(app.Artifacts()); n != 1 {
			t.Errorf("%d artifacts, want 1", n)
		}
	})

	t.Run("an image is filed as an image and removal deletes the unreferenced file", func(t *testing.T) {
		app := newTestAppAt(t, now)
		ac := docTestAccount(t, app)
		if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "photo.png", MIME: "image/png", Data: append(append([]byte(nil), pngTestBytes...), 1, 2, 3), ExpiryISO: "2026-04-01"}); err != nil {
			t.Fatal(err)
		}
		arts := app.Artifacts()
		if len(arts) != 1 || arts[0].Kind != "image" || arts[0].Size != len(pngTestBytes)+3 {
			t.Fatalf("artifacts = %+v, want one image", arts)
		}
		if len(app.Tasks()) != 1 {
			t.Fatalf("%d tasks, want the renewal reminder", len(app.Tasks()))
		}
		if err := app.RemoveAccountDocument(ac.ID, arts[0].ID); err != nil {
			t.Fatalf("RemoveAccountDocument: %v", err)
		}
		saved, _, _ := app.Store().GetAccount(ac.ID)
		if len(saved.DocRefs) != 0 {
			t.Fatalf("doc refs = %+v, want none", saved.DocRefs)
		}
		if len(app.Artifacts()) != 0 {
			t.Fatal("removing the last reference left the stored file behind")
		}
	})

	t.Run("removal errors name their cause", func(t *testing.T) {
		app := newTestAppAt(t, now)
		ac := docTestAccount(t, app)
		if err := app.RemoveAccountDocument("ghost", "x"); !errors.Is(err, ErrAccountNotFound) {
			t.Errorf("unknown account = %v, want ErrAccountNotFound", err)
		}
		if err := app.RemoveAccountDocument(ac.ID, "no-such-artifact"); !errors.Is(err, ErrAccountDocumentNotFound) {
			t.Errorf("unknown document = %v, want ErrAccountDocumentNotFound", err)
		}
	})

	t.Run("reconcileDocExpiry never panics on an empty ledger", func(t *testing.T) {
		app := newTestAppAt(t, now)
		app.reconcileDocExpiry(now)
		if n := len(app.Tasks()); n != 0 {
			t.Fatalf("%d tasks from an empty reconcile", n)
		}
	})
}

func TestMarkBalancesUpdatedRefusesViewerAndReportsPartialWrites(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)

	t.Run("viewer", func(t *testing.T) {
		app := seedDefaultsFixture(t)
		asViewer(app)
		written, err := app.MarkBalancesUpdated([]string{"chk"}, now)
		if !errors.Is(err, ErrReadOnly) || written != 0 {
			t.Fatalf("MarkBalancesUpdated = %d, %v; want 0 and ErrReadOnly", written, err)
		}
	})

	t.Run("an account that no longer validates is reported and the rest are still stamped", func(t *testing.T) {
		app := seedDefaultsFixture(t)
		// Stored directly, bypassing PutAccount's validation, as an older build could have left it.
		broken := acctCovAsset("broken", "", 0, time.Time{})
		if err := app.Store().PutAccount(broken); err != nil {
			t.Skipf("store refuses an unnamed account, so the partial-write path cannot be reached: %v", err)
		}
		written, err := app.MarkBalancesUpdated([]string{"broken", "chk"}, now)
		if err == nil || written != 1 {
			t.Fatalf("MarkBalancesUpdated = %d, %v; want 1 written and an error for the broken account", written, err)
		}
		chk, _, _ := app.Store().GetAccount("chk")
		if !chk.BalanceAsOf.Equal(now) {
			t.Fatalf("chk BalanceAsOf = %v, want %v", chk.BalanceAsOf, now)
		}
	})
}

func TestDefaultAccountMutationsRefuseViewer(t *testing.T) {
	app := seedDefaultsFixture(t)
	if err := app.SetDefaultAccountID("chk"); err != nil {
		t.Fatal(err)
	}
	asViewer(app)
	if err := app.SetDefaultAccountID("chk"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer SetDefaultAccountID = %v, want ErrReadOnly", err)
	}
	if err := app.ClearDefaultAccountID(); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer ClearDefaultAccountID = %v, want ErrReadOnly", err)
	}
	if got := app.DefaultAccountID(); got != "chk" {
		t.Fatalf("default account = %q, want it unchanged at chk", got)
	}
}

func TestReconcileDocExpiryLogsAndContinuesWhenTheReminderCannotBeSaved(t *testing.T) {
	now := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, now)
	ac := docTestAccount(t, app)
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "p.pdf", MIME: "application/pdf", Data: []byte("x"), ExpiryISO: "2026-04-01"}); err != nil {
		t.Fatal(err)
	}
	// Drop the reminder, then make saving a new one fail: the reconcile must log and return, not panic.
	for _, task := range app.Tasks() {
		if err := app.DeleteTask(task.ID); err != nil {
			t.Fatal(err)
		}
	}
	asViewer(app)
	app.reconcileDocExpiry(now)
	if n := len(app.Tasks()); n != 0 {
		t.Fatalf("%d tasks, want none because the viewer cannot create the reminder", n)
	}
}

func TestRestoringARetiredCheckpointKeepsTheReconciliationRecord(t *testing.T) {
	clock := time.Date(2026, 10, 31, 8, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, clock)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, time.Time{})); err != nil {
		t.Fatal(err)
	}
	acctCovCheckpoint(t, app, "guess", "chk", 2000, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	retiredRow, ok := app.TransactionByID("guess")
	if !ok {
		t.Fatal("checkpoint missing")
	}
	ev := domain.Reconciliation{At: clock, StatementDate: time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC), StatementBalance: money.New(102000, "USD")}
	if out, err := app.RecordReconciliation("chk", ev); err != nil || out.Retired != 1 {
		t.Fatalf("RecordReconciliation = %+v, %v", out, err)
	}
	if err := app.RestoreTransactions([]domain.Transaction{retiredRow}); err != nil {
		t.Fatalf("RestoreTransactions: %v", err)
	}
	account, _, _ := app.Store().GetAccount("chk")
	if len(account.Reconciliations) != 1 || !account.BalanceAsOf.Equal(clock) {
		t.Fatalf("after restoring the retired checkpoint: history=%d asOf=%v, want the record and stamp kept", len(account.Reconciliations), account.BalanceAsOf)
	}
	if _, ok := app.TransactionByID("guess"); !ok {
		t.Fatal("the retired checkpoint was not restored")
	}
}
