// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reconcile"
	"github.com/monstercameron/CashFlux/internal/sweep"
)

func hardeningCard(id string, sign domain.LiabilitySign) domain.Account {
	return domain.Account{ID: id, Name: id, Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, LiabilitySign: sign}
}

func bookedBalance(t *testing.T, app *App, id string) int64 {
	t.Helper()
	ac, ok := app.findAccount(id)
	if !ok {
		t.Fatalf("account %q missing", id)
	}
	bal, err := ledger.Balance(ac, app.Transactions())
	if err != nil {
		t.Fatal(err)
	}
	return bal.Amount
}

func TestBalanceUpdateBooksDebtWithTheAccountsFixedSign(t *testing.T) {
	for _, sign := range []domain.LiabilitySign{domain.LiabilityPositiveOwed, domain.LiabilityNegativeOwed} {
		t.Run(string(sign), func(t *testing.T) {
			at := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
			app := newTestAppAt(t, at)
			if err := app.PutAccount(hardeningCard("card", sign)); err != nil {
				t.Fatal(err)
			}
			// An overpayment puts the card in credit, the state where inferring the sign from the
			// booked balance used to pick the opposite convention.
			credit := int64(-5000)
			if sign == domain.LiabilityNegativeOwed {
				credit = 5000
			}
			if err := app.PutTransaction(domain.Transaction{ID: "pay", AccountID: "card", Desc: "Overpayment",
				Amount: money.New(credit, "USD"), Date: at}); err != nil {
				t.Fatal(err)
			}
			preview, err := app.PreviewAccountReconciliation("card", -30000, at)
			if err != nil {
				t.Fatal(err)
			}
			if preview.CurrentStatedMinor != 5000 {
				t.Fatalf("current stated balance = %d, want +5000 (money held)", preview.CurrentStatedMinor)
			}
			if _, err := app.CommitAccountReconciliation(preview, "Balance update"); err != nil {
				t.Fatal(err)
			}
			want := int64(30000)
			if sign == domain.LiabilityNegativeOwed {
				want = -30000
			}
			if got := bookedBalance(t, app, "card"); got != want {
				t.Fatalf("booked balance after stating 'I owe 300.00' = %d, want %d", got, want)
			}
		})
	}
}

func TestSweepApprovalIsIdempotentAndChecksBeforeMovingMoney(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	newRule := func(t *testing.T) (*App, sweep.Proposal) {
		app := sweepFixture(t)
		if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		props := app.SweepProposals(now)
		if len(props) != 1 {
			t.Fatalf("proposals = %+v, want one", props)
		}
		return app, props[0]
	}

	t.Run("a repeated approval posts once", func(t *testing.T) {
		app, p := newRule(t)
		if err := app.ApproveSweepProposal(p, "Surplus sweep", now); err != nil {
			t.Fatalf("approval 1: %v", err)
		}
		for i := 1; i < 3; i++ {
			if err := app.ApproveSweepProposal(p, "Surplus sweep", now); !errors.Is(err, ErrSweepAlreadyPosted) {
				t.Fatalf("approval %d = %v, want ErrSweepAlreadyPosted", i+1, err)
			}
		}
		if n := len(app.Transactions()); n != 2 {
			t.Fatalf("%d transactions after three approvals, want the two legs of one transfer", n)
		}
		if got := bookedBalance(t, app, "chk"); got != 300000 {
			t.Fatalf("checking = %d, want 300000", got)
		}
	})

	t.Run("an amount above the current surplus is refused", func(t *testing.T) {
		app, p := newRule(t)
		p.AmountMinor += 1
		if err := app.ApproveSweepProposal(p, "Sweep", now); !errors.Is(err, ErrSweepProposalStale) {
			t.Fatalf("ApproveSweepProposal = %v, want ErrSweepProposalStale", err)
		}
		if n := len(app.Transactions()); n != 0 {
			t.Fatalf("%d transactions posted for a stale proposal", n)
		}
	})

	t.Run("a rule that changed since the proposal is refused", func(t *testing.T) {
		app, p := newRule(t)
		if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "bro", KeepMinor: 300000, Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err := app.ApproveSweepProposal(p, "Sweep", now); !errors.Is(err, ErrSweepProposalStale) {
			t.Fatalf("ApproveSweepProposal = %v, want ErrSweepProposalStale", err)
		}
		if n := len(app.Transactions()); n != 0 {
			t.Fatalf("%d transactions posted", n)
		}
	})

	t.Run("an archived destination is refused", func(t *testing.T) {
		app, p := newRule(t)
		sav, _ := app.findAccount("sav")
		sav.Archived = true
		if err := app.PutAccount(sav); err != nil {
			t.Fatal(err)
		}
		if err := app.ApproveSweepProposal(p, "Sweep", now); err == nil {
			t.Fatal("a sweep into an archived account was approved")
		}
		if n := len(app.Transactions()); n != 0 {
			t.Fatalf("%d transactions posted", n)
		}
	})

	t.Run("a viewer cannot approve", func(t *testing.T) {
		app, p := newRule(t)
		asViewer(app)
		if err := app.ApproveSweepProposal(p, "Sweep", now); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("ApproveSweepProposal = %v, want ErrReadOnly", err)
		}
	})
}

func TestSweepProposalsShareOneSourceBalance(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	for _, r := range []domain.SweepRule{
		{ID: "a", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true},
		{ID: "b", SourceAccountID: "chk", DestAccountID: "bro", KeepMinor: 300000, Enabled: true},
	} {
		if _, err := app.PutSweepRule(r); err != nil {
			t.Fatal(err)
		}
	}
	props := app.SweepProposals(now)
	if len(props) != 1 || props[0].RuleID != "a" || props[0].AmountMinor != 200000 {
		t.Fatalf("proposals = %+v, want the whole 2,000.00 surplus proposed once, by rule a", props)
	}

	// A second rule with a lower floor takes only what the first left.
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "b", SourceAccountID: "chk", DestAccountID: "bro", KeepMinor: 100000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	props = app.SweepProposals(now)
	var total int64
	for _, p := range props {
		total += p.AmountMinor
	}
	if len(props) != 2 || total > 500000-100000 || props[1].AmountMinor != 200000 {
		t.Fatalf("proposals = %+v, want 2,000.00 then the remaining 2,000.00 above the lower floor", props)
	}
}

func TestSweepProposalsRespectEarmarksArchivedAccountsAndLiabilities(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", KeepMinor: 300000, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().PutEarmark(domain.Earmark{ID: "em", DestinationID: "chk", DestinationKind: domain.EarmarkKindAccount,
		Amount: money.New(50000, "USD"), Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	props := app.SweepProposals(now)
	if len(props) != 1 || props[0].AmountMinor != 150000 || props[0].EarmarkedMinor != 50000 {
		t.Fatalf("proposals = %+v, want an earmark to reduce the sweep to 1,500.00", props)
	}

	sav, _ := app.findAccount("sav")
	sav.Archived = true
	if err := app.PutAccount(sav); err != nil {
		t.Fatal(err)
	}
	if props := app.SweepProposals(now); len(props) != 0 {
		t.Fatalf("proposals = %+v, want none into an archived destination", props)
	}

	if err := app.PutAccount(hardeningCard("card", domain.LiabilityPositiveOwed)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.PutSweepRule(domain.SweepRule{SourceAccountID: "card", DestAccountID: "bro"}); err == nil {
		t.Fatal("a liability was accepted as a sweep source")
	}
}

func TestSweepRuleWritesNeedEditRights(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := sweepFixture(t)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r1", SourceAccountID: "chk", DestAccountID: "sav", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	asViewer(app)
	if _, err := app.PutSweepRule(domain.SweepRule{ID: "r2", SourceAccountID: "chk", DestAccountID: "sav"}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer PutSweepRule = %v, want ErrReadOnly", err)
	}
	if err := app.DeleteSweepRule("r1"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer DeleteSweepRule = %v, want ErrReadOnly", err)
	}
	if err := app.MarkSweepProposed("r1", now); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer MarkSweepProposed = %v, want ErrReadOnly", err)
	}
	rules, _ := app.Store().ListSweepRules()
	if len(rules) != 1 || rules[0].LastProposed != "" {
		t.Fatalf("rules = %+v, want the one original rule untouched", rules)
	}
}

func TestTransferPreviewMatchesWhatPosts(t *testing.T) {
	at := time.Date(2026, 6, 1, 15, 30, 0, 0, time.UTC)
	app := newTestAppAt(t, at)
	if err := app.PutAccount(hardeningCard("card", domain.LiabilityPositiveOwed)); err != nil {
		t.Fatal(err)
	}
	if err := app.PutAccount(refsAccount("chk")); err != nil {
		t.Fatal(err)
	}
	// A cash advance from the card with a fee: the debt grows by both.
	p := TransferParams{FromAccountID: "card", ToAccountID: "chk", AmountMinor: 20000, FeeMinor: 500}
	pv, err := app.PreviewTransferPair(p)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := app.PreviewExpectedTransfer(p)
	if err != nil {
		t.Fatal(err)
	}
	if pv != expected.Balances {
		t.Fatalf("preview %+v differs from the expected-state preview %+v", pv, expected.Balances)
	}
	if _, _, err := app.CreateTransferPair(p); err != nil {
		t.Fatal(err)
	}
	if got := bookedBalance(t, app, "card"); got != pv.FromAfter.Amount || got != 20500 {
		t.Fatalf("card balance = %d, preview said %d, want 20500", got, pv.FromAfter.Amount)
	}
	if got := bookedBalance(t, app, "chk"); got != pv.ToAfter.Amount {
		t.Fatalf("checking balance = %d, preview said %d", got, pv.ToAfter.Amount)
	}
	for _, tx := range app.Transactions() {
		if !tx.Date.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("transfer row dated %v, want the app clock's day", tx.Date)
		}
	}
}

func TestTransfersRefuseArchivedAccounts(t *testing.T) {
	app := newApp(t, false)
	for _, id := range []string{"a", "b"} {
		if err := app.PutAccount(refsAccount(id)); err != nil {
			t.Fatal(err)
		}
	}
	b := storedAccount(app, "b")
	b.Archived = true
	if err := app.PutAccount(b); err != nil {
		t.Fatal(err)
	}
	p := TransferParams{FromAccountID: "a", ToAccountID: "b", AmountMinor: 100}
	if _, err := app.PreviewTransferPair(p); err == nil {
		t.Error("PreviewTransferPair accepted an archived account")
	}
	if _, _, err := app.CreateTransferPair(p); err == nil {
		t.Error("CreateTransferPair accepted an archived account")
	}
	if n := len(app.Transactions()); n != 0 {
		t.Errorf("%d transactions posted", n)
	}
}

func TestAttachAccountDocumentRefusesScriptCapableAndOversizeFiles(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	ac := docTestAccount(t, app)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)
	refused := []struct {
		name string
		in   AccountDocumentInput
	}{
		{"html with a charset", AccountDocumentInput{Name: "a.pdf", MIME: "text/html; charset=utf-8", Data: []byte("x")}},
		{"svg with parameters", AccountDocumentInput{Name: "a.png", MIME: "image/svg+xml;charset=utf-8", Data: []byte("x")}},
		{"upper-case svg", AccountDocumentInput{Name: "a.svg", MIME: "IMAGE/SVG+XML", Data: []byte("x")}},
		{"text javascript", AccountDocumentInput{Name: "a.txt", MIME: "text/javascript", Data: []byte("x")}},
		{"application javascript", AccountDocumentInput{Name: "a.txt", MIME: "application/javascript", Data: []byte("x")}},
		{"xslt", AccountDocumentInput{Name: "a.txt", MIME: "application/xslt+xml", Data: []byte("x")}},
		{"text xsl", AccountDocumentInput{Name: "a.txt", MIME: "text/xsl", Data: []byte("x")}},
		{"xhtml", AccountDocumentInput{Name: "a.txt", MIME: "application/xhtml+xml", Data: []byte("x")}},
		{"blank type but a .svg name", AccountDocumentInput{Name: "logo.SVG", Data: []byte("x")}},
		{"spoofed pdf type but a .html name", AccountDocumentInput{Name: "page.html", MIME: "application/pdf", Data: []byte("x")}},
		{"trailing dot hides the extension", AccountDocumentInput{Name: "page.htm.", MIME: "application/pdf", Data: []byte("x")}},
		{"png type with svg bytes", AccountDocumentInput{Name: "a.png", MIME: "image/png", Data: svg}},
		{"pdf type with html bytes", AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf", Data: []byte("\n  <!DOCTYPE html><html></html>")}},
		{"unknown image subtype", AccountDocumentInput{Name: "a.png", MIME: "image/x-evil", Data: []byte("x")}},
		{"malformed type", AccountDocumentInput{Name: "a.pdf", MIME: "not a/type;;=", Data: []byte("x")}},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			if err := app.AttachAccountDocument(ac.ID, tt.in); !errors.Is(err, ErrUnsupportedDocType) {
				t.Fatalf("AttachAccountDocument = %v, want ErrUnsupportedDocType", err)
			}
		})
	}
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf"}); !errors.Is(err, ErrDocumentEmpty) {
		t.Errorf("empty file = %v, want ErrDocumentEmpty", err)
	}
	big := bytes.Repeat([]byte{'a'}, MaxAccountDocumentBytes+1)
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf", Data: big}); !errors.Is(err, ErrDocumentTooLarge) {
		t.Errorf("oversize file = %v, want ErrDocumentTooLarge", err)
	}
	if n := len(app.Artifacts()); n != 0 {
		t.Fatalf("%d artifacts stored by refused attachments", n)
	}

	accepted := []struct {
		name, mime, wantMIME, wantKind string
	}{
		{"scan.png", "IMAGE/PNG", "image/png", "image"},
		{"scan.jpg", "image/jpeg; q=1", "image/jpeg", "image"},
		{"policy.pdf", "Application/PDF", "application/pdf", "document"},
		{"letter.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "document"},
		{"notes.bin", "", "", "document"},
	}
	for _, tt := range accepted {
		if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: tt.name, MIME: tt.mime, Data: acceptedDocBytes(tt.name)}); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
	}
	got := map[string]domain.Artifact{}
	for _, art := range app.Artifacts() {
		got[art.Name] = art
	}
	for _, tt := range accepted {
		art := got[tt.name]
		if art.MIME != tt.wantMIME || art.Kind != tt.wantKind {
			t.Errorf("%s stored as kind %q type %q, want %q %q", tt.name, art.Kind, art.MIME, tt.wantKind, tt.wantMIME)
		}
	}
}

// acceptedDocBytes returns file bytes that suit the file name: raster magic for images, a PDF header otherwise.
func acceptedDocBytes(name string) []byte {
	switch {
	case strings.HasSuffix(name, ".png"):
		return pngTestBytes
	case strings.HasSuffix(name, ".jpg"):
		return jpegTestBytes
	}
	return []byte("%PDF-1.4 data")
}

func TestClearDefaultAccountIDReportsAnUnreadableStore(t *testing.T) {
	app := seedDefaultsFixture(t)
	if err := app.SetDefaultAccountID("chk"); err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.ClearDefaultAccountID(); err == nil {
		t.Fatal("ClearDefaultAccountID succeeded against a closed store")
	}
	if err := app.SetDefaultAccountID("chk"); err == nil {
		t.Fatal("SetDefaultAccountID succeeded against a closed store")
	}
}

func TestMemberDefaultAccountNeedsEditRightsNotMemberManagement(t *testing.T) {
	app := seedDefaultsFixture(t)
	app.SetActiveRoleFunc(func() domain.MemberRole { return domain.RoleAdmin })
	if _, err := app.SetMemberDefaultAccount("m1", "chk"); err != nil {
		t.Fatalf("editor SetMemberDefaultAccount = %v, want it allowed", err)
	}
	if got := memberDefault(t, app, "m1"); got != "chk" {
		t.Fatalf("default = %q, want chk", got)
	}
	if err := app.PutMember(domain.Member{ID: "m2", Name: "Other"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("editor PutMember = %v, want it still refused", err)
	}
	asViewer(app)
	if _, err := app.SetMemberDefaultAccount("m1", "chk"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer SetMemberDefaultAccount = %v, want ErrReadOnly", err)
	}
	if _, err := app.ClearMemberDefaultAccount("m1"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer ClearMemberDefaultAccount = %v, want ErrReadOnly", err)
	}
	if got := memberDefault(t, app, "m1"); got != "chk" {
		t.Fatalf("default after refused clear = %q, want chk", got)
	}
}

func TestPostReconcileAdjustmentGuards(t *testing.T) {
	at := time.Date(2026, 3, 20, 15, 45, 0, 0, time.UTC)
	app := newTestAppAt(t, at)
	if err := app.PutAccount(refsAccount("acc")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.PostReconcileAdjustment("acc", 0, reconcile.AsStated, "Adjustment"); err == nil {
		t.Error("a zero difference posted a row")
	}
	first, err := app.PostReconcileAdjustmentOnce("acc", "key-1", 2500, reconcile.AsStated, "Adjustment")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Date.Equal(time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("row dated %v, want the start of the day", first.Date)
	}
	again, err := app.PostReconcileAdjustmentOnce("acc", "key-1", 2500, reconcile.AsStated, "Adjustment")
	if err != nil || again.ID != first.ID {
		t.Fatalf("retry = %+v, %v; want the first row back", again, err)
	}
	if n := len(app.Transactions()); n != 1 {
		t.Fatalf("%d rows after a retried action, want 1", n)
	}
	if _, err := app.PostReconcileAdjustmentOnce("acc", "key-2", 100, reconcile.AsStated, "Adjustment"); err != nil {
		t.Fatal(err)
	}
	if n := len(app.Transactions()); n != 2 {
		t.Fatalf("%d rows after a second action, want 2", n)
	}

	acc := storedAccount(app, "acc")
	acc.Archived = true
	if err := app.PutAccount(acc); err != nil {
		t.Fatal(err)
	}
	if _, err := app.PostReconcileAdjustment("acc", 100, reconcile.AsStated, "Adjustment"); err == nil {
		t.Error("an archived account was adjusted")
	}
	asViewer(app)
	if _, err := app.PostReconcileAdjustment("acc", 100, reconcile.AsStated, "Adjustment"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("viewer adjustment = %v, want ErrReadOnly", err)
	}
}

func TestMarkBalancesUpdatedCountsADuplicateIDOnce(t *testing.T) {
	now := time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)
	app := seedDefaultsFixture(t)
	written, err := app.MarkBalancesUpdated([]string{"chk", "chk", "old"}, now)
	if err != nil || written != 2 {
		t.Fatalf("MarkBalancesUpdated = %d, %v; want 2 distinct accounts", written, err)
	}
}

func TestAccountMethodsOnAMissingAppReturnAnError(t *testing.T) {
	var app *App
	now := time.Now()
	calls := map[string]func() error{
		"PutAccount":       func() error { return app.PutAccount(refsAccount("a")) },
		"DeleteAccount":    func() error { return app.DeleteAccount("a") },
		"SetDefaultMember": func() error { return app.SetDefaultMember("m") },
		"Preview":          func() error { _, err := app.PreviewAccountReconciliation("a", 0, now); return err },
		"Commit": func() error {
			_, err := app.CommitAccountReconciliation(AccountReconciliationPreview{}, "")
			return err
		},
		"Record":       func() error { _, err := app.RecordReconciliation("a", domain.Reconciliation{}); return err },
		"Adjust":       func() error { _, err := app.PostReconcileAdjustment("a", 1, reconcile.AsStated, "x"); return err },
		"MarkBalances": func() error { _, err := app.MarkBalancesUpdated([]string{"a"}, now); return err },
		"Transfer": func() error {
			_, _, err := app.CreateTransferPair(TransferParams{FromAccountID: "a", ToAccountID: "b", AmountMinor: 1})
			return err
		},
		"TransferPreview": func() error {
			_, err := app.PreviewTransferPair(TransferParams{FromAccountID: "a", ToAccountID: "b", AmountMinor: 1})
			return err
		},
		"ExpectedPreview": func() error {
			_, err := app.PreviewExpectedTransfer(TransferParams{FromAccountID: "a", ToAccountID: "b", AmountMinor: 1})
			return err
		},
		"PutSweepRule":  func() error { _, err := app.PutSweepRule(domain.SweepRule{}); return err },
		"Approve":       func() error { return app.ApproveSweepProposal(sweep.Proposal{}, "x", now) },
		"MemberDefault": func() error { _, err := app.SetMemberDefaultAccount("m", "a"); return err },
		"Attach": func() error {
			return app.AttachAccountDocument("a", AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf", Data: []byte("x")})
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, ErrDurabilityUnavailable) {
				t.Fatalf("err = %v, want ErrDurabilityUnavailable", err)
			}
		})
	}
}
