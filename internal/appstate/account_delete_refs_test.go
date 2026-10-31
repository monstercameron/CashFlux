// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

func storedAccount(app *App, id string) domain.Account {
	ac, _ := app.findAccount(id)
	return ac
}

func refsAccount(id string) domain.Account {
	return domain.Account{ID: id, Name: id, Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
}

// seedAccountReferences stores one record of every kind that points at account "gone", plus a
// sibling record pointing at "stay" that must survive.
func seedAccountReferences(t *testing.T, app *App) {
	t.Helper()
	for _, id := range []string{"gone", "stay"} {
		if err := app.PutAccount(refsAccount(id)); err != nil {
			t.Fatal(err)
		}
	}
	st := app.Store()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(app.PutMember(domain.Member{ID: "m1", Name: "Priya", Prefs: domain.MemberPrefs{DefaultAccountID: "gone"}}))
	must(app.SetDefaultAccountID("gone"))
	must(st.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "link-gone", DebtAccountID: "gone", Currency: "USD"}))
	must(st.PutDebtPaymentLink(domain.DebtPaymentLink{ID: "link-stay", DebtAccountID: "stay", Currency: "USD"}))
	must(st.PutDebtPayeeRule(domain.DebtPayeeRule{ID: "pr-gone", Payee: "x", DebtAccountID: "stay", SourceAccountID: "gone"}))
	must(st.PutDebtPayeeRule(domain.DebtPayeeRule{ID: "pr-stay", Payee: "y", DebtAccountID: "stay"}))
	must(st.PutSweepRule(domain.SweepRule{ID: "sw-gone", SourceAccountID: "stay", DestAccountID: "gone", Cadence: domain.SweepMonthly}))
	must(st.PutSweepRule(domain.SweepRule{ID: "sw-stay", SourceAccountID: "stay", DestAccountID: "stay2", Cadence: domain.SweepMonthly}))
	must(st.PutGoal(domain.Goal{ID: "goal", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		TargetAmount: money.New(1000, "USD"), AccountID: "gone", AccountIDs: []string{"stay", "gone"},
		Allocations: []domain.GoalAllocation{{AccountID: "gone", Amount: money.New(10, "USD")}, {AccountID: "stay", Amount: money.New(20, "USD")}}}))
	must(st.PutAccountGroup(domain.AccountGroup{ID: "grp", Name: "Liquid", AccountIDs: []string{"gone", "stay"}}))
	must(st.PutHolding(domain.Holding{ID: "h-gone", AccountID: "gone"}))
	must(st.PutHolding(domain.Holding{ID: "h-stay", AccountID: "stay"}))
	must(st.PutRealizedSale(domain.RealizedSale{ID: "rs-gone", AccountID: "gone"}))
	must(st.PutRecurring(domain.Recurring{ID: "rec-gone", Label: "Rent", AccountID: "gone"}))
	must(st.PutRecurring(domain.Recurring{ID: "rec-stay", Label: "Pay", AccountID: "stay"}))
	must(st.PutBalanceSnapshot(domain.BalanceSnapshot{ID: "bs-gone", AccountID: "gone", AsOf: time.Now()}))
	must(st.PutEarmark(domain.Earmark{ID: "em-gone", DestinationID: "gone", DestinationKind: domain.EarmarkKindAccount}))
	must(st.PutDocument(domain.Document{ID: "doc", AccountID: "gone"}))
	must(st.PutTask(domain.Task{ID: "task-gone", Title: "Renew", RelatedType: domain.RelatedAccount, RelatedID: "gone",
		Custom: map[string]any{docExpiryTaskKey: "k1"}}))
	must(st.PutTask(domain.Task{ID: "task-other", Title: "Other", RelatedType: domain.RelatedAccount, RelatedID: "gone"}))
	must(app.AttachAccountDocument("gone", AccountDocumentInput{Name: "policy.pdf", MIME: "application/pdf", Data: []byte("pdf")}))
}

func TestDeleteAccountRemovesEveryReferenceInOneCommit(t *testing.T) {
	app := newApp(t, false)
	if err := app.PutAccount(refsAccount("stay2")); err != nil {
		t.Fatal(err)
	}
	seedAccountReferences(t, app)
	artifactID := storedAccount(app, "gone").DocRefs[0].ArtifactID
	st := app.Store()
	revBefore := st.Rev()

	if err := app.DeleteAccount("gone"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if st.Rev() == revBefore {
		t.Fatal("DeleteAccount wrote nothing")
	}

	if _, ok, _ := st.GetAccount("gone"); ok {
		t.Error("account still stored")
	}
	if got := memberDefault(t, app, "m1"); got != "" {
		t.Errorf("member default account = %q, want cleared", got)
	}
	if got := app.DefaultAccountID(); got != "" {
		t.Errorf("household default account = %q, want cleared", got)
	}
	links, _ := st.ListDebtPaymentLinks()
	if len(links) != 1 || links[0].ID != "link-stay" {
		t.Errorf("debt-payment links = %+v, want only link-stay", links)
	}
	payee, _ := st.ListDebtPayeeRules()
	if len(payee) != 1 || payee[0].ID != "pr-stay" {
		t.Errorf("debt payee rules = %+v, want only pr-stay", payee)
	}
	rules, _ := st.ListSweepRules()
	if len(rules) != 1 || rules[0].ID != "sw-stay" {
		t.Errorf("sweep rules = %+v, want only sw-stay", rules)
	}
	goal, _, _ := st.GetGoal("goal")
	if goal.AccountID != "" || !slices.Equal(goal.AccountIDs, []string{"stay"}) ||
		len(goal.Allocations) != 1 || goal.Allocations[0].AccountID != "stay" {
		t.Errorf("goal = %+v, want no reference to the deleted account and the stay links kept", goal)
	}
	grp, _, _ := st.GetAccountGroup("grp")
	if !slices.Equal(grp.AccountIDs, []string{"stay"}) {
		t.Errorf("group members = %v, want [stay]", grp.AccountIDs)
	}
	holdings, _ := st.ListHoldings()
	if len(holdings) != 1 || holdings[0].ID != "h-stay" {
		t.Errorf("holdings = %+v, want only h-stay", holdings)
	}
	if sales, _ := st.ListRealizedSales(); len(sales) != 0 {
		t.Errorf("realized sales = %+v, want none", sales)
	}
	recurring, _ := st.ListRecurring()
	if len(recurring) != 1 || recurring[0].ID != "rec-stay" {
		t.Errorf("recurring = %+v, want only rec-stay", recurring)
	}
	if snaps, _ := st.ListBalanceSnapshots("gone"); len(snaps) != 0 {
		t.Errorf("balance snapshots = %+v, want none", snaps)
	}
	if earmarks, _ := st.ListEarmarks(); len(earmarks) != 0 {
		t.Errorf("earmarks = %+v, want none", earmarks)
	}
	if doc, _, _ := st.GetDocument("doc"); doc.AccountID != "" {
		t.Errorf("document account = %q, want cleared", doc.AccountID)
	}
	if _, ok, _ := st.GetArtifact(artifactID); ok {
		t.Error("attached file still stored")
	}
	tasks, _ := st.ListTasks()
	if len(tasks) != 1 || tasks[0].ID != "task-other" {
		t.Errorf("tasks = %+v, want only the unrelated task", tasks)
	}
}

func TestDeleteAccountKeepsASharedAttachmentAndRefusesAnAccountWithTransactions(t *testing.T) {
	app := newApp(t, false)
	for _, id := range []string{"a", "b"} {
		if err := app.PutAccount(refsAccount(id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.AttachAccountDocument("a", AccountDocumentInput{Name: "x.pdf", MIME: "application/pdf", Data: []byte("x")}); err != nil {
		t.Fatal(err)
	}
	shared := storedAccount(app, "a").DocRefs[0]
	b := storedAccount(app, "b")
	b.DocRefs = []domain.AccountDocRef{shared}
	if err := app.PutAccount(b); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteAccount("a"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := app.Store().GetArtifact(shared.ArtifactID); !ok {
		t.Fatal("an attachment still referenced by another account was removed")
	}

	if err := app.PutTransaction(domain.Transaction{ID: "t1", AccountID: "b", Desc: "x", Amount: money.New(100, "USD"), Date: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteAccount("b"); !errors.Is(err, ErrAccountHasTransactions) {
		t.Fatalf("DeleteAccount with a transaction = %v, want ErrAccountHasTransactions", err)
	}
	if _, ok, _ := app.Store().GetAccount("b"); !ok {
		t.Fatal("a refused delete removed the account")
	}
	if err := app.DeleteAccount("never-existed"); err != nil {
		t.Fatalf("deleting an unknown account = %v, want a no-op", err)
	}
}

func TestPutAccountWritesTheBalanceSnapshotWithTheAccount(t *testing.T) {
	at := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	app := newTestAppAt(t, at)
	ac := refsAccount("acc")
	ac.OpeningBalance = money.New(5000, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	snaps := app.BalanceHistory("acc")
	if len(snaps) != 1 || snaps[0].BalanceMinor != 5000 || !snaps[0].AsOf.Equal(at) || snaps[0].Currency != "USD" {
		t.Fatalf("snapshots after create = %+v, want one 5000 USD point at the app clock", snaps)
	}
	if got := storedAccount(app, "acc"); got.OpeningBalance.Amount != 5000 {
		t.Fatalf("stored opening balance = %d, want 5000", got.OpeningBalance.Amount)
	}
	ac.Name = "Renamed"
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	if n := len(app.BalanceHistory("acc")); n != 1 {
		t.Fatalf("%d snapshots after an edit that kept the balance, want 1", n)
	}
	ac.OpeningBalance = money.New(7000, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	if n := len(app.BalanceHistory("acc")); n != 2 {
		t.Fatalf("%d snapshots after a balance change, want 2", n)
	}
}

func TestPutAccountRejectedWriteLeavesNoPhantomSnapshot(t *testing.T) {
	app := newApp(t, false)
	card := domain.Account{ID: "card", Name: "Card", Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(9000, "USD"), LiabilitySign: "sideways"}
	if err := app.PutAccount(card); err == nil {
		t.Fatal("an account with an unknown liability sign was saved")
	}
	if snaps := app.BalanceHistory("card"); len(snaps) != 0 {
		t.Fatalf("snapshots = %+v, want none for an account that failed to save", snaps)
	}
	if _, ok, _ := app.Store().GetAccount("card"); ok {
		t.Fatal("the rejected account was stored")
	}
}

func TestPutAccountKeepsTheLiabilitySignWhenTheOpeningBalanceChanges(t *testing.T) {
	app := newApp(t, false)
	card := domain.Account{ID: "card", Name: "Card", Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, OpeningBalance: money.New(9000, "USD")}
	if err := app.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	if got := storedAccount(app, "card").LiabilitySign; got != domain.LiabilityPositiveOwed {
		t.Fatalf("sign after create = %q, want positive_owed", got)
	}
	card.LiabilitySign = ""
	card.OpeningBalance = money.New(-4000, "USD")
	if err := app.PutAccount(card); err != nil {
		t.Fatal(err)
	}
	if got := storedAccount(app, "card").LiabilitySign; got != domain.LiabilityPositiveOwed {
		t.Fatalf("sign after a corrected opening balance = %q, want it kept as positive_owed", got)
	}
}

func TestPutAccountRefusesACurrencyChangeOnceTransactionsExist(t *testing.T) {
	app := newApp(t, false)
	ac := refsAccount("acc")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	ac.Currency = "EUR"
	if err := app.PutAccount(ac); err != nil {
		t.Fatalf("changing the currency of an empty account = %v, want it allowed", err)
	}
	ac.Currency = "USD"
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "t1", AccountID: "acc", Desc: "x", Amount: money.New(100, "USD"), Date: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ac.Currency = "EUR"
	if err := app.PutAccount(ac); !errors.Is(err, ErrAccountCurrencyLocked) {
		t.Fatalf("PutAccount currency change = %v, want ErrAccountCurrencyLocked", err)
	}
	if got := storedAccount(app, "acc").Currency; got != "USD" {
		t.Fatalf("stored currency = %q, want USD", got)
	}
}

func TestSetDefaultMemberResolvesTheTargetBeforeWriting(t *testing.T) {
	app := newApp(t, false)
	for _, m := range []domain.Member{{ID: "m1", Name: "A", IsDefault: true}, {ID: "m2", Name: "B"}} {
		if err := app.Store().PutMember(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.SetDefaultMember("ghost"); err == nil {
		t.Fatal("SetDefaultMember with an unknown id succeeded")
	}
	if got := app.DefaultMemberID(); got != "m1" {
		t.Fatalf("default after a failed change = %q, want m1 kept", got)
	}
	if err := app.SetDefaultMember("m2"); err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, m := range app.Members() {
		if m.IsDefault {
			defaults++
			if m.ID != "m2" {
				t.Fatalf("default member = %q, want m2", m.ID)
			}
		}
	}
	if defaults != 1 {
		t.Fatalf("%d default members, want exactly one", defaults)
	}
	asViewer(app)
	if err := app.SetDefaultMember("m1"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("viewer SetDefaultMember = %v, want ErrReadOnly", err)
	}
	if got := app.DefaultMemberID(); got != "m2" {
		t.Fatalf("default after a refused change = %q, want m2", got)
	}
}

func TestBulkMutateRestoresSuppressionAfterAPanic(t *testing.T) {
	app := newApp(t, false)
	fired := 0
	app.OnTxnMutated(func() { fired++ })
	func() {
		defer func() { _ = recover() }()
		app.BulkMutate(func() { panic("boom") })
	}()
	if app.triggersSuspended || app.suppressTxnObservers {
		t.Fatal("a panic inside BulkMutate left triggers or observers suppressed")
	}
	if fired != 0 {
		t.Fatalf("observers fired %d times for a panicked bulk edit, want 0", fired)
	}
	app.BulkMutate(func() {})
	if fired != 1 {
		t.Fatalf("observers fired %d times after a normal bulk edit, want 1", fired)
	}
}

func TestRecordReconciliationChangesNothingWhenTheAccountCannotBeSaved(t *testing.T) {
	at := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	app := newApp(t, false)
	if err := app.PutAccount(acctCovAsset("chk", "Checking", 100000, at)); err != nil {
		t.Fatal(err)
	}
	acctCovCheckpoint(t, app, "cp1", "chk", 5000, at)
	// An account an older build left unnamed cannot be saved through validation.
	broken := storedAccount(app, "chk")
	broken.Name = ""
	if err := app.Store().PutAccount(broken); err != nil {
		t.Skipf("store refuses an unnamed account: %v", err)
	}
	if _, err := app.RecordReconciliation("chk", domain.Reconciliation{At: at.AddDate(0, 0, 1), StatementBalance: money.New(105000, "USD")}); err == nil {
		t.Fatal("RecordReconciliation succeeded for an account that does not validate")
	}
	if _, ok, _ := app.Store().GetTransaction("cp1"); !ok {
		t.Fatal("the checkpoint was retired although the reconciliation was not recorded")
	}
	if got := storedAccount(app, "chk"); len(got.Reconciliations) != 0 {
		t.Fatalf("reconciliations = %+v, want none", got.Reconciliations)
	}
}

func TestDeleteAccountClearsRuleWorkflowTaskAndBillReferences(t *testing.T) {
	app := newApp(t, false)
	for _, id := range []string{"gone", "stay"} {
		if err := app.PutAccount(refsAccount(id)); err != nil {
			t.Fatal(err)
		}
	}
	st := app.Store()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	is := func(op rules.ConditionOp, id string) rules.RuleCondition {
		return rules.RuleCondition{Field: rules.ConditionFieldAccount, Op: op, Value: id}
	}
	must(st.PutRule(rules.Rule{ID: "bill-only", Match: "water", SetBillAccountID: "gone"}))
	must(st.PutRule(rules.Rule{ID: "bill-and-cat", Match: "power", SetBillAccountID: "gone", SetCategoryID: "utilities"}))
	must(st.PutRule(rules.Rule{ID: "dead", SetCategoryID: "x", Conditions: []rules.RuleCondition{is(rules.ConditionOpIs, "gone")}}))
	must(st.PutRule(rules.Rule{ID: "not-gone", SetCategoryID: "x", Conditions: []rules.RuleCondition{
		is(rules.ConditionOpIsNot, "gone"), {Field: rules.ConditionFieldPayee, Op: rules.ConditionOpContains, Value: "cafe"}}}))
	must(st.PutRule(rules.Rule{ID: "other", SetCategoryID: "x", Conditions: []rules.RuleCondition{is(rules.ConditionOpIs, "stay")}}))

	must(st.PutWorkflow(workflow.Workflow{ID: "wf-mixed", Name: "Mixed", Enabled: true, Actions: []workflow.Action{
		{Kind: workflow.ActionTransfer, TransferFromAccountID: "stay", TransferToAccountID: "gone", TransferAmount: 100},
		{Kind: workflow.ActionTransfer, TransferFromAccountID: "stay", TransferToAccountID: "stay", TransferAmount: 50},
	}}))
	must(st.PutWorkflow(workflow.Workflow{ID: "wf-only", Name: "Only", Enabled: true, Actions: []workflow.Action{
		{Kind: workflow.ActionTransfer, TransferFromAccountID: "gone", TransferToAccountID: "stay", TransferAmount: 100},
	}}))

	must(st.PutTask(domain.Task{ID: "linked", Title: "Check statement", RelatedType: domain.RelatedAccount, RelatedID: "gone"}))
	must(st.PutTask(domain.Task{ID: "kept", Title: "Check other", RelatedType: domain.RelatedAccount, RelatedID: "stay"}))
	must(app.PutTransaction(domain.Transaction{ID: "pay", AccountID: "stay", Desc: "Water bill", Date: time.Now(),
		Amount: money.New(-500, "USD"), BillAccountID: "gone"}))

	if err := app.DeleteAccount("gone"); err != nil {
		t.Fatal(err)
	}

	got, _ := st.ListRules()
	byID := map[string]rules.Rule{}
	for _, r := range got {
		byID[r.ID] = r
	}
	if _, ok := byID["bill-only"]; ok {
		t.Error("a rule whose only action was the deleted bill account was kept")
	}
	if r, ok := byID["bill-and-cat"]; !ok || r.SetBillAccountID != "" || r.SetCategoryID != "utilities" {
		t.Errorf("bill-and-cat = %+v, want the bill link cleared and the category kept", r)
	}
	if _, ok := byID["dead"]; ok {
		t.Error("a rule that can never match again was kept")
	}
	if r := byID["not-gone"]; len(r.Conditions) != 1 || r.Conditions[0].Field != rules.ConditionFieldPayee {
		t.Errorf("not-gone conditions = %+v, want only the payee condition", r.Conditions)
	}
	if r := byID["other"]; len(r.Conditions) != 1 || r.Conditions[0].Value != "stay" {
		t.Errorf("other = %+v, want it untouched", r)
	}

	mixed, _, _ := st.GetWorkflow("wf-mixed")
	if len(mixed.Actions) != 1 || mixed.Actions[0].TransferToAccountID != "stay" || !mixed.Enabled {
		t.Errorf("wf-mixed = %+v, want only the stay transfer and still enabled", mixed)
	}
	only, _, _ := st.GetWorkflow("wf-only")
	if len(only.Actions) != 0 || only.Enabled {
		t.Errorf("wf-only = %+v, want no actions and switched off", only)
	}

	linked, _, _ := st.GetTask("linked")
	if linked.RelatedType != domain.RelatedNone || linked.RelatedID != "" {
		t.Errorf("linked task = %+v, want the account link cleared", linked)
	}
	kept, _, _ := st.GetTask("kept")
	if kept.RelatedID != "stay" {
		t.Errorf("kept task = %+v, want it untouched", kept)
	}
	pay, _ := app.TransactionByID("pay")
	if pay.BillAccountID != "" {
		t.Errorf("payment BillAccountID = %q, want it cleared", pay.BillAccountID)
	}
}

func TestWithoutTriggersRestoresThePauseAfterAPanicAndIgnoresANilFunc(t *testing.T) {
	app := newApp(t, false)
	func() {
		defer func() { _ = recover() }()
		app.WithoutTriggers(func() { panic("boom") })
	}()
	if app.triggersSuspended {
		t.Fatal("a panic inside WithoutTriggers left workflow triggers paused")
	}
	app.WithoutTriggers(nil)
	app.BulkMutate(nil)
	if app.triggersSuspended || app.suppressTxnObservers {
		t.Fatal("a nil func changed the suppression state")
	}
}

func TestPutAccountRefusesACurrencyChangeWhileRecordsReferenceIt(t *testing.T) {
	seed := map[string]func(*testing.T, *App){
		"holding": func(t *testing.T, app *App) {
			if err := app.Store().PutHolding(domain.Holding{ID: "h", AccountID: "acc"}); err != nil {
				t.Fatal(err)
			}
		},
		"recurring entry": func(t *testing.T, app *App) {
			if err := app.Store().PutRecurring(domain.Recurring{ID: "rec", Label: "Rent", AccountID: "acc"}); err != nil {
				t.Fatal(err)
			}
		},
		"goal allocation": func(t *testing.T, app *App) {
			if err := app.Store().PutGoal(domain.Goal{ID: "g", Name: "Trip", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
				TargetAmount: money.New(1000, "USD"),
				Allocations:  []domain.GoalAllocation{{AccountID: "acc", Amount: money.New(10, "USD")}}}); err != nil {
				t.Fatal(err)
			}
		},
		"earmark": func(t *testing.T, app *App) {
			if err := app.Store().PutEarmark(domain.Earmark{ID: "em", DestinationID: "acc", DestinationKind: domain.EarmarkKindAccount,
				Amount: money.New(500, "USD"), Currency: "USD"}); err != nil {
				t.Fatal(err)
			}
		},
		"realized sale": func(t *testing.T, app *App) {
			if err := app.Store().PutRealizedSale(domain.RealizedSale{ID: "rs", AccountID: "acc", Name: "Fund"}); err != nil {
				t.Fatal(err)
			}
		},
		"debt payment link": func(t *testing.T, app *App) {
			if err := app.Store().PutDebtPaymentLink(domain.DebtPaymentLink{ID: "link", DebtAccountID: "acc", Currency: "USD"}); err != nil {
				t.Fatal(err)
			}
		},
		"sweep rule source": func(t *testing.T, app *App) {
			if err := app.Store().PutSweepRule(domain.SweepRule{ID: "sw", SourceAccountID: "acc", DestAccountID: "elsewhere", Cadence: domain.SweepMonthly}); err != nil {
				t.Fatal(err)
			}
		},
		"sweep rule destination": func(t *testing.T, app *App) {
			if err := app.Store().PutSweepRule(domain.SweepRule{ID: "sw", SourceAccountID: "elsewhere", DestAccountID: "acc", Cadence: domain.SweepMonthly}); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, add := range seed {
		t.Run(name, func(t *testing.T) {
			app := newApp(t, false)
			ac := refsAccount("acc")
			if err := app.PutAccount(ac); err != nil {
				t.Fatal(err)
			}
			add(t, app)
			st := app.Store()
			// The records and the account's own history as they stand before the refused change.
			counts := func() [5]int {
				earmarks, _ := st.ListEarmarks()
				sales, _ := st.ListRealizedSales()
				links, _ := st.ListDebtPaymentLinks()
				sweeps, _ := st.ListSweepRules()
				snaps, _ := st.ListBalanceSnapshots("acc")
				return [5]int{len(earmarks), len(sales), len(links), len(sweeps), len(snaps)}
			}
			before := counts()
			ac.Currency = "EUR"
			if err := app.PutAccount(ac); !errors.Is(err, ErrAccountCurrencyLocked) {
				t.Fatalf("PutAccount currency change = %v, want ErrAccountCurrencyLocked", err)
			}
			if got := storedAccount(app, "acc").Currency; got != "USD" {
				t.Fatalf("stored currency = %q, want USD", got)
			}
			if after := counts(); after != before {
				t.Fatalf("record counts (earmarks, sales, links, sweeps, snapshots) = %v after the refusal, want %v", after, before)
			}
			snaps, _ := st.ListBalanceSnapshots("acc")
			for _, sn := range snaps {
				if sn.Currency == "EUR" {
					t.Fatalf("snapshot %s was re-stamped to %q by a refused change", sn.ID, sn.Currency)
				}
			}
		})
	}

	t.Run("records on another account do not lock it", func(t *testing.T) {
		app := newApp(t, false)
		for _, id := range []string{"acc", "other"} {
			if err := app.PutAccount(refsAccount(id)); err != nil {
				t.Fatal(err)
			}
		}
		if err := app.Store().PutHolding(domain.Holding{ID: "h", AccountID: "other"}); err != nil {
			t.Fatal(err)
		}
		ac := refsAccount("acc")
		ac.Currency = "EUR"
		if err := app.PutAccount(ac); err != nil {
			t.Fatalf("PutAccount currency change = %v, want it allowed", err)
		}
	})
}

func TestPutAccountMovesALoneOpeningSnapshotWithACurrencyCorrection(t *testing.T) {
	app := newApp(t, false)
	ac := refsAccount("acc")
	ac.OpeningBalance = money.New(1000, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	ac.Currency = "EUR"
	ac.OpeningBalance = money.New(1000, "EUR")
	if err := app.PutAccount(ac); err != nil {
		t.Fatalf("correcting the currency of a just-created account = %v, want it allowed", err)
	}
	if got := storedAccount(app, "acc").Currency; got != "EUR" {
		t.Fatalf("stored currency = %q, want EUR", got)
	}
	snaps := app.BalanceHistory("acc")
	if len(snaps) != 1 || snaps[0].Currency != "EUR" || snaps[0].BalanceMinor != 1000 {
		t.Fatalf("balance history = %+v, want the one opening point restated in EUR", snaps)
	}
}

func TestPutAccountCurrencyChangeRestampsEveryOpeningBalanceSnapshot(t *testing.T) {
	app := newApp(t, false)
	ac := refsAccount("acc")
	ac.OpeningBalance = money.New(1000, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	ac.OpeningBalance = money.New(2500, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	ac.OpeningBalance = money.New(2600, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	if n := len(app.BalanceHistory("acc")); n < 2 {
		t.Fatalf("%d balance snapshots after balance edits, want at least two", n)
	}
	other := refsAccount("other")
	other.OpeningBalance = money.New(700, "USD")
	if err := app.PutAccount(other); err != nil {
		t.Fatal(err)
	}

	ac.Currency = "EUR"
	ac.OpeningBalance = money.New(2600, "EUR")
	if err := app.PutAccount(ac); err != nil {
		t.Fatalf("currency change on an account with only opening-balance snapshots = %v, want it allowed", err)
	}
	if got := storedAccount(app, "acc").Currency; got != "EUR" {
		t.Fatalf("stored currency = %q, want EUR", got)
	}
	for _, s := range app.BalanceHistory("acc") {
		if s.Currency != "EUR" {
			t.Fatalf("snapshot %+v kept the old currency, want EUR", s)
		}
	}
	for _, s := range app.BalanceHistory("other") {
		if s.Currency != "USD" {
			t.Fatalf("another account's snapshot %+v changed currency", s)
		}
	}
}

func TestPutAccountCurrencyChangeWithABalanceEditAddsOnePointInTheNewCurrency(t *testing.T) {
	app := newApp(t, false)
	ac := refsAccount("acc")
	ac.OpeningBalance = money.New(1000, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	ac.Currency = "EUR"
	ac.OpeningBalance = money.New(1200, "EUR")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	snaps := app.BalanceHistory("acc")
	if len(snaps) != 2 {
		t.Fatalf("balance history = %+v, want the earlier point and the corrected one", snaps)
	}
	for _, s := range snaps {
		if s.Currency != "EUR" {
			t.Fatalf("snapshot %+v is not in EUR", s)
		}
	}
}

func TestPutAccountCurrencyChangeStaysRefusedWithATransactionAndSnapshots(t *testing.T) {
	app := newApp(t, false)
	ac := refsAccount("acc")
	ac.OpeningBalance = money.New(1000, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	ac.OpeningBalance = money.New(2500, "USD")
	if err := app.PutAccount(ac); err != nil {
		t.Fatal(err)
	}
	if err := app.PutTransaction(domain.Transaction{ID: "t1", AccountID: "acc", Desc: "x", Amount: money.New(100, "USD"), Date: time.Now()}); err != nil {
		t.Fatal(err)
	}
	ac.Currency = "EUR"
	ac.OpeningBalance = money.New(2500, "EUR")
	if err := app.PutAccount(ac); !errors.Is(err, ErrAccountCurrencyLocked) {
		t.Fatalf("PutAccount currency change = %v, want ErrAccountCurrencyLocked", err)
	}
	for _, s := range app.BalanceHistory("acc") {
		if s.Currency != "USD" {
			t.Fatalf("a refused change re-stamped snapshot %+v", s)
		}
	}
}
