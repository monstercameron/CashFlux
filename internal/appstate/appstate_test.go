package appstate

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func newApp(t *testing.T, seed bool) *App {
	t.Helper()
	a, err := New(&bytes.Buffer{}, seed)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return a
}

func TestNewSeedsSampleData(t *testing.T) {
	a := newApp(t, true)
	if len(a.Accounts()) == 0 {
		t.Error("expected seeded accounts")
	}
	if len(a.Transactions()) == 0 {
		t.Error("expected seeded transactions")
	}
	if a.Settings().BaseCurrency != "USD" {
		t.Errorf("base currency = %q, want USD", a.Settings().BaseCurrency)
	}
}

func TestNewEmpty(t *testing.T) {
	a := newApp(t, false)
	if len(a.Accounts()) != 0 {
		t.Errorf("expected empty store, got %d accounts", len(a.Accounts()))
	}
}

func TestRuleValidationAndRoundTrip(t *testing.T) {
	a := newApp(t, false)

	// Validation: id, match phrase, and category are all required.
	bad := []rules.Rule{
		{Match: "x", SetCategoryID: "c1"}, // no id
		{ID: "r", SetCategoryID: "c1"},    // no match
		{ID: "r", Match: "   "},           // blank match
		{ID: "r", Match: "x"},             // no category
	}
	for i, r := range bad {
		if err := a.PutRule(r); err == nil {
			t.Errorf("bad rule %d accepted: %+v", i, r)
		}
	}

	if err := a.PutRule(rules.Rule{ID: "r1", Match: "coffee", SetCategoryID: "cafe"}); err != nil {
		t.Fatalf("PutRule: %v", err)
	}
	got := a.Rules()
	if len(got) != 1 || got[0].Match != "coffee" {
		t.Fatalf("Rules() = %+v", got)
	}
	if err := a.DeleteRule("r1"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if len(a.Rules()) != 0 {
		t.Error("rule still present after delete")
	}
}

func TestApplyRules(t *testing.T) {
	a := newApp(t, false)
	acc := domain.Account{
		ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutRule(rules.Rule{ID: "r1", Match: "uber", SetCategoryID: "transport", SetTags: []string{"travel"}}); err != nil {
		t.Fatalf("PutRule: %v", err)
	}
	mk := func(id, desc, cat string) domain.Transaction {
		return domain.Transaction{ID: id, AccountID: "a1", Desc: desc, CategoryID: cat, Date: time.Now(), Amount: money.New(-500, "USD")}
	}
	for _, tx := range []domain.Transaction{
		mk("t1", "Uber ride home", ""),       // matches, uncategorized -> updated
		mk("t2", "Uber Eats dinner", "food"), // matches but already categorized -> untouched
		mk("t3", "Grocery store", ""),        // no match -> stays uncategorized
		{
			ID: "t4", AccountID: "a1", TransferAccountID: "a2", Desc: "Uber transfer",
			Date: time.Now(), Amount: money.New(-500, "USD"),
		},
		{
			ID: "t5", AccountID: "a1", Desc: "Uber rewards", Tags: []string{"existing"},
			Date: time.Now(), Amount: money.New(-500, "USD"),
		},
	} {
		if err := a.PutTransaction(tx); err != nil {
			t.Fatalf("PutTransaction %s: %v", tx.ID, err)
		}
	}

	n, err := a.ApplyRules()
	if err != nil {
		t.Fatalf("ApplyRules: %v", err)
	}
	if n != 2 {
		t.Errorf("updated = %d, want 2", n)
	}
	byID := map[string]domain.Transaction{}
	for _, tx := range a.Transactions() {
		byID[tx.ID] = tx
	}
	if got := byID["t1"]; got.CategoryID != "transport" || len(got.Tags) != 1 || got.Tags[0] != "travel" {
		t.Errorf("t1 not categorized by rule: %+v", got)
	}
	if byID["t2"].CategoryID != "food" {
		t.Errorf("t2 should keep its category, got %q", byID["t2"].CategoryID)
	}
	if byID["t3"].CategoryID != "" {
		t.Errorf("t3 should stay uncategorized, got %q", byID["t3"].CategoryID)
	}
	if byID["t4"].CategoryID != "" {
		t.Errorf("t4 transfer should be skipped, got %q", byID["t4"].CategoryID)
	}
	if got := byID["t5"]; got.CategoryID != "transport" || len(got.Tags) != 1 || got.Tags[0] != "existing" {
		t.Errorf("t5 should be categorized while preserving existing tags: %+v", got)
	}
}

func TestDocumentRoundTrip(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutDocument(domain.Document{}); err == nil {
		t.Error("expected error putting a document without an id")
	}
	doc := domain.Document{ID: "d1", Kind: domain.DocImage, Status: domain.DocPending, UploadedAt: time.Now()}
	if err := a.PutDocument(doc); err != nil {
		t.Fatalf("PutDocument: %v", err)
	}
	if got := a.Documents(); len(got) != 1 || got[0].Kind != domain.DocImage {
		t.Fatalf("Documents() = %+v", got)
	}
	if err := a.DeleteDocument("d1"); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if len(a.Documents()) != 0 {
		t.Error("document still present after delete")
	}
}

func TestSavedInsightRoundTrip(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutSavedInsight(domain.SavedInsight{ID: "si1"}); err == nil {
		t.Error("expected error for a saved insight with no text")
	}
	if err := a.PutSavedInsight(domain.SavedInsight{Text: "x"}); err == nil {
		t.Error("expected error for a saved insight with no id")
	}
	if err := a.PutSavedInsight(domain.SavedInsight{ID: "si1", Text: "Net worth is up.", CreatedAt: time.Now()}); err != nil {
		t.Fatalf("PutSavedInsight: %v", err)
	}
	if got := a.SavedInsights(); len(got) != 1 || got[0].Text != "Net worth is up." {
		t.Fatalf("SavedInsights() = %+v", got)
	}
	if err := a.DeleteSavedInsight("si1"); err != nil {
		t.Fatalf("DeleteSavedInsight: %v", err)
	}
	if len(a.SavedInsights()) != 0 {
		t.Error("saved insight still present after delete")
	}
}

func TestRecurringRoundTrip(t *testing.T) {
	a := newApp(t, false)
	bad := []domain.Recurring{
		{Label: "x", Amount: money.New(1, "USD"), Cadence: domain.CadenceMonthly}, // no id
		{ID: "r", Amount: money.New(1, "USD"), Cadence: domain.CadenceMonthly},    // no label
		{ID: "r", Label: "x", Cadence: domain.CadenceMonthly},                     // no currency
		{ID: "r", Label: "x", Amount: money.New(1, "USD")},                        // no cadence
	}
	for i, r := range bad {
		if err := a.PutRecurring(r); err == nil {
			t.Errorf("bad recurring %d accepted: %+v", i, r)
		}
	}
	good := domain.Recurring{ID: "r1", Label: "Netflix", Amount: money.New(-1599, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Now()}
	if err := a.PutRecurring(good); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}
	if got := a.Recurring(); len(got) != 1 || got[0].Label != "Netflix" {
		t.Fatalf("Recurring() = %+v", got)
	}
	if err := a.DeleteRecurring("r1"); err != nil {
		t.Fatalf("DeleteRecurring: %v", err)
	}
	if len(a.Recurring()) != 0 {
		t.Error("recurring still present after delete")
	}
}

func TestAllocProfileRoundTrip(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAllocProfile(domain.AllocationProfile{Name: "x"}); err == nil {
		t.Error("expected error for profile with no id")
	}
	if err := a.PutAllocProfile(domain.AllocationProfile{ID: "p"}); err == nil {
		t.Error("expected error for profile with no name")
	}
	if err := a.PutAllocProfile(domain.AllocationProfile{ID: "p1", Name: "Safety", Stability: 3, Liquidity: 2}); err != nil {
		t.Fatalf("PutAllocProfile: %v", err)
	}
	if got := a.AllocProfiles(); len(got) != 1 || got[0].Name != "Safety" {
		t.Fatalf("AllocProfiles() = %+v", got)
	}
	if err := a.DeleteAllocProfile("p1"); err != nil {
		t.Fatalf("DeleteAllocProfile: %v", err)
	}
	if len(a.AllocProfiles()) != 0 {
		t.Error("alloc profile still present after delete")
	}
}

func TestFormulaRoundTrip(t *testing.T) {
	a := newApp(t, false)
	bad := []domain.Formula{
		{Name: "x", Expr: "1"}, // no id
		{ID: "f", Expr: "1"},   // no name
		{ID: "f", Name: "x"},   // no expr
	}
	for i, f := range bad {
		if err := a.PutFormula(f); err == nil {
			t.Errorf("bad formula %d accepted: %+v", i, f)
		}
	}
	if err := a.PutFormula(domain.Formula{ID: "f1", Name: "Savings", Expr: "income - expense", Enabled: true}); err != nil {
		t.Fatalf("PutFormula: %v", err)
	}
	if got := a.Formulas(); len(got) != 1 || got[0].Name != "Savings" {
		t.Fatalf("Formulas() = %+v", got)
	}
	if err := a.DeleteFormula("f1"); err != nil {
		t.Fatalf("DeleteFormula: %v", err)
	}
	if len(a.Formulas()) != 0 {
		t.Error("formula still present after delete")
	}
}

func TestCustomFieldFormulaExportImportRoundTrip(t *testing.T) {
	a := newApp(t, false)
	def := customfields.Def{
		ID: "cf1", EntityType: "account", Key: "apr", Label: "Annual percentage rate", Type: customfields.TypeNumber,
	}
	if err := a.PutCustomFieldDef(def); err != nil {
		t.Fatalf("PutCustomFieldDef: %v", err)
	}
	if err := a.PutAccount(domain.Account{
		ID: "a1", Name: "Rewards Card", Currency: "USD", Type: domain.TypeCreditCard, Class: domain.ClassLiability,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Custom: map[string]any{"apr": 19.99},
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutFormula(domain.Formula{ID: "f1", Name: "APR stress", Expr: "round(apr * 2)", Enabled: true}); err != nil {
		t.Fatalf("PutFormula: %v", err)
	}

	data, err := a.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	b := newApp(t, false)
	if err := b.ImportJSON(data); err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	defs := b.CustomFieldDefs()
	accounts := b.Accounts()
	if issues := customfields.Validate(defs, accounts[0].Custom); len(issues) != 0 {
		t.Fatalf("imported custom fields failed validation: %v", issues)
	}
	apr, ok := accounts[0].Custom["apr"].(float64)
	if !ok {
		t.Fatalf("imported apr = %v (%T), want float64", accounts[0].Custom["apr"], accounts[0].Custom["apr"])
	}
	got, err := formula.Eval(b.Formulas()[0].Expr, formula.Env{Vars: map[string]float64{"apr": apr}})
	if err != nil {
		t.Fatalf("Eval imported formula: %v", err)
	}
	if got != float64(40) {
		t.Errorf("imported formula result = %v, want 40", got)
	}
}

func TestPlanRoundTrip(t *testing.T) {
	a := newApp(t, false)
	bad := []domain.Plan{
		{Name: "x", HorizonMonths: 6},           // no id
		{ID: "p", HorizonMonths: 6},             // no name
		{ID: "p", Name: "x"},                    // non-positive horizon
		{ID: "p", Name: "x", HorizonMonths: -1}, // negative horizon
	}
	for i, p := range bad {
		if err := a.PutPlan(p); err == nil {
			t.Errorf("bad plan %d accepted: %+v", i, p)
		}
	}
	if err := a.PutPlan(domain.Plan{ID: "p1", Name: "Runway", HorizonMonths: 6, StartBalance: 300000}); err != nil {
		t.Fatalf("PutPlan: %v", err)
	}
	if got := a.Plans(); len(got) != 1 || got[0].Name != "Runway" {
		t.Fatalf("Plans() = %+v", got)
	}
	if err := a.DeletePlan("p1"); err != nil {
		t.Fatalf("DeletePlan: %v", err)
	}
	if len(a.Plans()) != 0 {
		t.Error("plan still present after delete")
	}
}

func TestPostDueRecurring(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutAccount(domain.Account{
		ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	now := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Autopost recurring with an account → posts and catches up.
	if err := a.PutRecurring(domain.Recurring{
		ID: "r1", Label: "Salary", Amount: money.New(420000, "USD"), Cadence: domain.CadenceMonthly,
		NextDue: due, AccountID: "a1", CategoryID: "income", Autopost: true,
	}); err != nil {
		t.Fatalf("PutRecurring autopost: %v", err)
	}
	// Autopost but no account → skipped.
	if err := a.PutRecurring(domain.Recurring{
		ID: "r2", Label: "Mystery", Amount: money.New(-1000, "USD"), Cadence: domain.CadenceMonthly,
		NextDue: due, Autopost: true,
	}); err != nil {
		t.Fatalf("PutRecurring no-account: %v", err)
	}
	// Not autopost → skipped even though due.
	if err := a.PutRecurring(domain.Recurring{
		ID: "r3", Label: "Manual", Amount: money.New(-500, "USD"), Cadence: domain.CadenceMonthly,
		NextDue: due, AccountID: "a1",
	}); err != nil {
		t.Fatalf("PutRecurring manual: %v", err)
	}

	n, err := a.PostDueRecurring(now)
	if err != nil {
		t.Fatalf("PostDueRecurring: %v", err)
	}
	if n != 4 {
		t.Errorf("posted = %d, want 4 (Jan-Apr catch-up months)", n)
	}
	// Every posted transaction is the salary; mystery/manual posted nothing.
	for _, tx := range a.Transactions() {
		if tx.Desc != "Salary" || tx.AccountID != "a1" || tx.Amount.Amount != 420000 {
			t.Errorf("unexpected posted txn: %+v", tx)
		}
	}
	for _, r := range a.Recurring() {
		if r.ID == "r1" && !r.NextDue.Equal(time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("r1 NextDue = %s, want 2026-05-01", r.NextDue.Format("2006-01-02"))
		}
	}
	// r1's NextDue is now advanced past now; re-posting does not double-count the catch-up months.
	if again, _ := a.PostDueRecurring(now); again != 0 {
		t.Errorf("second post = %d, want 0 (already caught up)", again)
	}
	if got := len(a.Transactions()); got != 4 {
		t.Errorf("transactions after second post = %d, want 4", got)
	}
}

func TestPutAccountValidatesCustomFields(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCustomFieldDef(customfields.Def{
		ID: "cf1", EntityType: "account", Key: "branch", Label: "Branch", Type: customfields.TypeText, Required: true,
	}); err != nil {
		t.Fatalf("PutCustomFieldDef: %v", err)
	}

	acc := domain.Account{
		ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}

	// Missing the required custom field → rejected.
	if err := a.PutAccount(acc); err == nil {
		t.Error("expected error for missing required custom field")
	}

	// Wrong type for the custom field → rejected.
	acc.Custom = map[string]any{"branch": 42}
	if err := a.PutAccount(acc); err == nil {
		t.Error("expected error for wrong-typed custom field")
	}

	// Correct value → accepted and persisted.
	acc.Custom = map[string]any{"branch": "Downtown"}
	if err := a.PutAccount(acc); err != nil {
		t.Fatalf("expected valid account to save, got %v", err)
	}
	got := a.Accounts()
	if len(got) != 1 || got[0].Custom["branch"] != "Downtown" {
		t.Errorf("custom value not persisted: %+v", got)
	}
}

func TestReassignCategory(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutCategory(domain.Category{ID: "old", Name: "Old", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory old: %v", err)
	}
	if err := a.PutCategory(domain.Category{ID: "new", Name: "New", Kind: domain.KindExpense}); err != nil {
		t.Fatalf("PutCategory new: %v", err)
	}
	if err := a.PutAccount(domain.Account{
		ID: "a1", Name: "Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutTransaction(domain.Transaction{
		ID: "t1", AccountID: "a1", CategoryID: "old", Desc: "Lunch",
		Date: time.Now(), Amount: money.New(-500, "USD"),
	}); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	if err := a.PutBudget(domain.Budget{
		ID: "b1", Name: "Food", CategoryID: "old", Period: domain.PeriodMonthly,
		Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Limit: money.New(10000, "USD"),
	}); err != nil {
		t.Fatalf("PutBudget: %v", err)
	}

	moved, err := a.ReassignCategory("old", "new")
	if err != nil {
		t.Fatalf("ReassignCategory: %v", err)
	}
	if moved != 2 {
		t.Errorf("moved = %d, want 2", moved)
	}
	for _, tr := range a.Transactions() {
		if tr.ID == "t1" && tr.CategoryID != "new" {
			t.Errorf("transaction not reassigned: %q", tr.CategoryID)
		}
	}
	for _, b := range a.Budgets() {
		if b.ID == "b1" && b.CategoryID != "new" {
			t.Errorf("budget not reassigned: %q", b.CategoryID)
		}
	}
}

func TestReassignOwner(t *testing.T) {
	a := newApp(t, false)
	if err := a.PutMember(domain.Member{ID: "m1", Name: "Alex"}); err != nil {
		t.Fatalf("PutMember: %v", err)
	}
	if err := a.PutAccount(domain.Account{
		ID: "a1", Name: "Alex Checking", Currency: "USD", Type: domain.TypeChecking, Class: domain.ClassAsset,
		OwnerID: "m1", Scope: domain.ScopeIndividual,
	}); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if err := a.PutGoal(domain.Goal{
		ID: "g1", Name: "Trip", OwnerID: "m1", Scope: domain.ScopeIndividual,
		TargetAmount: money.New(100000, "USD"),
	}); err != nil {
		t.Fatalf("PutGoal: %v", err)
	}

	// Reassign to the group owner: scope becomes shared.
	moved, err := a.ReassignOwner("m1", domain.GroupOwnerID)
	if err != nil {
		t.Fatalf("ReassignOwner: %v", err)
	}
	if moved != 2 {
		t.Errorf("moved = %d, want 2", moved)
	}
	for _, ac := range a.Accounts() {
		if ac.ID == "a1" && (ac.OwnerID != domain.GroupOwnerID || ac.Scope != domain.ScopeShared) {
			t.Errorf("account not reassigned: owner=%q scope=%v", ac.OwnerID, ac.Scope)
		}
	}
	for _, g := range a.Goals() {
		if g.ID == "g1" && g.OwnerID != domain.GroupOwnerID {
			t.Errorf("goal not reassigned: owner=%q", g.OwnerID)
		}
	}
}

func TestPutValidatesAndPersists(t *testing.T) {
	a := newApp(t, false)

	// Invalid account is rejected.
	if err := a.PutAccount(domain.Account{ID: "x"}); err == nil {
		t.Error("expected validation error for incomplete account")
	}
	if len(a.Accounts()) != 0 {
		t.Error("invalid account should not be stored")
	}

	// Valid account persists.
	ok := domain.Account{
		ID: "a1", Name: "Checking", OwnerID: "m1", Scope: domain.ScopeIndividual,
		Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
		OpeningBalance: money.New(1000, "USD"),
	}
	if err := a.PutAccount(ok); err != nil {
		t.Fatalf("PutAccount: %v", err)
	}
	if len(a.Accounts()) != 1 {
		t.Fatalf("expected 1 account, got %d", len(a.Accounts()))
	}

	if err := a.DeleteAccount("a1"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if len(a.Accounts()) != 0 {
		t.Error("account not deleted")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	a := newApp(t, true)
	data, err := a.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	b := newApp(t, false)
	if err := b.ImportJSON(data); err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if len(b.Accounts()) != len(a.Accounts()) {
		t.Errorf("imported accounts = %d, want %d", len(b.Accounts()), len(a.Accounts()))
	}

	again, _ := b.ExportJSON()
	if !bytes.Equal(data, again) {
		t.Error("export/import not lossless across apps")
	}
}

func TestLoadSampleAndWipe(t *testing.T) {
	a := newApp(t, false)
	if len(a.Accounts()) != 0 {
		t.Fatalf("expected empty store, got %d accounts", len(a.Accounts()))
	}

	if err := a.LoadSample(); err != nil {
		t.Fatalf("LoadSample: %v", err)
	}
	if len(a.Accounts()) == 0 || len(a.Transactions()) == 0 {
		t.Error("LoadSample should populate accounts and transactions")
	}

	if err := a.Wipe(); err != nil {
		t.Fatalf("Wipe: %v", err)
	}
	if len(a.Accounts()) != 0 || len(a.Transactions()) != 0 || len(a.Members()) != 0 {
		t.Error("Wipe should leave the store empty")
	}
}

func TestExportCSV(t *testing.T) {
	a := newApp(t, true)
	data, err := a.ExportCSV()
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if len(data) == 0 {
		t.Error("ExportCSV should produce output for seeded data")
	}
}

func TestInitSetsDefault(t *testing.T) {
	if err := Init(&bytes.Buffer{}, true); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if Default == nil || len(Default.Accounts()) == 0 {
		t.Error("Init should set a seeded Default app")
	}
}
