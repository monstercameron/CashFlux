package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
)

func newStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAccountCRUD(t *testing.T) {
	s := newStore(t)

	if err := s.PutAccount(domain.Account{ID: "a1", Name: "Checking", Currency: "USD"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetAccount("a1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Name != "Checking" {
		t.Errorf("name = %q, want Checking", got.Name)
	}

	// Update via Put (upsert).
	if err := s.PutAccount(domain.Account{ID: "a1", Name: "Renamed", Currency: "USD"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ = s.GetAccount("a1")
	if got.Name != "Renamed" {
		t.Errorf("after update name = %q, want Renamed", got.Name)
	}

	list, _ := s.ListAccounts()
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}

	deleted, err := s.DeleteAccount("a1")
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetAccount("a1"); ok {
		t.Error("account still present after delete")
	}
}

func TestRuleCRUD(t *testing.T) {
	s := newStore(t)

	if err := s.PutRule(rules.Rule{ID: "r1", Match: "uber", SetCategoryID: "transport", SetTags: []string{"travel"}}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetRule("r1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.SetCategoryID != "transport" || len(got.SetTags) != 1 || got.SetTags[0] != "travel" {
		t.Errorf("rule round-trip wrong: %+v", got)
	}

	// Upsert.
	if err := s.PutRule(rules.Rule{ID: "r1", Match: "uber eats", SetCategoryID: "food"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _, _ = s.GetRule("r1")
	if got.Match != "uber eats" || got.SetCategoryID != "food" {
		t.Errorf("after update = %+v", got)
	}

	if list, _ := s.ListRules(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}

	deleted, err := s.DeleteRule("r1")
	if err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetRule("r1"); ok {
		t.Error("rule still present after delete")
	}
}

func TestDocumentCRUD(t *testing.T) {
	s := newStore(t)
	doc := domain.Document{
		ID: "d1", Filename: "stmt.csv", Kind: domain.DocCSV, UploadedAt: time.Now(),
		AccountID: "a1", Status: domain.DocExtracted,
		Extracted: []domain.DocumentRow{{Date: "2026-06-01", Description: "Coffee", Amount: "-4.50"}},
	}
	if err := s.PutDocument(doc); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetDocument("d1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Status != domain.DocExtracted || len(got.Extracted) != 1 || got.Extracted[0].Description != "Coffee" {
		t.Errorf("document round-trip wrong: %+v", got)
	}

	// Status transition via upsert.
	doc.Status = domain.DocImported
	if err := s.PutDocument(doc); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _, _ = s.GetDocument("d1"); got.Status != domain.DocImported {
		t.Errorf("after update status = %q", got.Status)
	}

	if list, _ := s.ListDocuments(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteDocument("d1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetDocument("d1"); ok {
		t.Error("document still present after delete")
	}
}

func TestSavedInsightCRUD(t *testing.T) {
	s := newStore(t)
	si := domain.SavedInsight{ID: "si1", Text: "Spending is down 12%.", CreatedAt: time.Now()}
	if err := s.PutSavedInsight(si); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetSavedInsight("si1")
	if err != nil || !ok || got.Text != "Spending is down 12%." {
		t.Fatalf("Get: ok=%v err=%v got=%+v", ok, err, got)
	}
	if list, _ := s.ListSavedInsights(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteSavedInsight("si1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetSavedInsight("si1"); ok {
		t.Error("saved insight still present after delete")
	}
}

func TestRecurringCRUD(t *testing.T) {
	s := newStore(t)
	r := domain.Recurring{
		ID: "rec1", Label: "Rent", Amount: money.New(-150000, "USD"), Cadence: domain.CadenceMonthly,
		NextDue: time.Now(), AccountID: "a1", CategoryID: "housing",
	}
	if err := s.PutRecurring(r); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetRecurring("rec1")
	if err != nil || !ok || got.Label != "Rent" || got.Amount.Amount != -150000 || got.Cadence != domain.CadenceMonthly {
		t.Fatalf("Get: ok=%v err=%v got=%+v", ok, err, got)
	}
	if list, _ := s.ListRecurring(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteRecurring("rec1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetRecurring("rec1"); ok {
		t.Error("recurring still present after delete")
	}
}

func TestAllocProfileCRUD(t *testing.T) {
	s := newStore(t)
	p := domain.AllocationProfile{ID: "ap1", Name: "Growth", Returns: 3, Stability: 1, Liquidity: 1, DebtReduction: 1, GoalProgress: 2}
	if err := s.PutAllocProfile(p); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetAllocProfile("ap1")
	if err != nil || !ok || got.Name != "Growth" || got.Returns != 3 || got.GoalProgress != 2 {
		t.Fatalf("Get: ok=%v err=%v got=%+v", ok, err, got)
	}
	if list, _ := s.ListAllocProfiles(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteAllocProfile("ap1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetAllocProfile("ap1"); ok {
		t.Error("alloc profile still present after delete")
	}
}

func TestFormulaCRUD(t *testing.T) {
	s := newStore(t)
	f := domain.Formula{ID: "f1", Name: "Net", Expr: "income - expense", Enabled: true}
	if err := s.PutFormula(f); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetFormula("f1")
	if err != nil || !ok || got.Expr != "income - expense" || !got.Enabled {
		t.Fatalf("Get: ok=%v err=%v got=%+v", ok, err, got)
	}
	if list, _ := s.ListFormulas(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
	if deleted, err := s.DeleteFormula("f1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetFormula("f1"); ok {
		t.Error("formula still present after delete")
	}
}

func TestPlanCRUD(t *testing.T) {
	s := newStore(t)
	p := domain.Plan{
		ID: "p1", Name: "Save for house", HorizonMonths: 12, StartBalance: 500000,
		Items: []domain.PlanItem{
			{ID: "i1", Label: "Savings", Kind: domain.PlanItemRecurring, Amount: 50000},
			{ID: "i2", Label: "Bonus", Kind: domain.PlanItemOneTime, Month: 6, Amount: 200000},
		},
	}
	if err := s.PutPlan(p); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok, err := s.GetPlan("p1")
	if err != nil || !ok || got.HorizonMonths != 12 || got.StartBalance != 500000 || len(got.Items) != 2 {
		t.Fatalf("Get: ok=%v err=%v got=%+v", ok, err, got)
	}
	if got.Items[1].Kind != domain.PlanItemOneTime || got.Items[1].Month != 6 || got.Items[1].Amount != 200000 {
		t.Errorf("one-time item lost: %+v", got.Items[1])
	}
	if list, _ := s.ListPlans(); len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}
	if deleted, err := s.DeletePlan("p1"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetPlan("p1"); ok {
		t.Error("plan still present after delete")
	}
}

func TestGetAndDeleteMissing(t *testing.T) {
	s := newStore(t)
	if _, ok, err := s.GetGoal("nope"); ok || err != nil {
		t.Errorf("missing get: ok=%v err=%v", ok, err)
	}
	if deleted, err := s.DeleteGoal("nope"); deleted || err != nil {
		t.Errorf("missing delete: deleted=%v err=%v", deleted, err)
	}
}

func TestPutRequiresID(t *testing.T) {
	s := newStore(t)
	if err := s.PutMember(domain.Member{Name: "noid"}); err == nil {
		t.Error("expected error putting entity without id")
	}
}

func TestTransactionAttachmentsCRUD(t *testing.T) {
	s := newStore(t)
	d, _ := dateutil.ParseDate("2026-06-03")
	tx := domain.Transaction{
		ID:        "t1",
		AccountID: "a1",
		Date:      d,
		Amount:    money.New(-1299, "USD"),
		Attachments: []domain.AttachmentRef{{
			ArtifactID: "art-receipt",
			Name:       "receipt.png",
			Kind:       "image",
			MIME:       "image/png",
		}},
	}
	if err := s.PutTransaction(tx); err != nil {
		t.Fatalf("PutTransaction: %v", err)
	}
	got, ok, err := s.GetTransaction("t1")
	if err != nil || !ok {
		t.Fatalf("GetTransaction: ok=%v err=%v", ok, err)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].ArtifactID != "art-receipt" || got.Attachments[0].MIME != "image/png" {
		t.Fatalf("attachments round-trip wrong: %+v", got.Attachments)
	}
}

func TestTransactionQueries(t *testing.T) {
	s := newStore(t)
	usd := func(n int64) money.Money { return money.New(n, "USD") }
	mk := func(id, acc, cat, member, day string, amt int64) domain.Transaction {
		d, _ := dateutil.ParseDate(day)
		return domain.Transaction{ID: id, AccountID: acc, CategoryID: cat, MemberID: member, Date: d, Amount: usd(amt), Desc: id}
	}
	txns := []domain.Transaction{
		mk("t1", "a1", "food", "m1", "2026-06-03", -100),
		mk("t2", "a1", "rent", "m2", "2026-06-10", -200),
		mk("t3", "a2", "food", "m1", "2026-07-05", -300),
	}
	for _, tx := range txns {
		if err := s.PutTransaction(tx); err != nil {
			t.Fatalf("put: %v", err)
		}
	}

	if got, _ := s.TransactionsByAccount("a1"); len(got) != 2 {
		t.Errorf("by account a1 = %d, want 2", len(got))
	}
	if got, _ := s.TransactionsByCategory("food"); len(got) != 2 {
		t.Errorf("by category food = %d, want 2", len(got))
	}
	if got, _ := s.TransactionsByMember("m1"); len(got) != 2 {
		t.Errorf("by member m1 = %d, want 2", len(got))
	}

	start, end := dateutil.MonthRange(mustMonth("2026-06-15"))
	if got, _ := s.TransactionsByDateRange(start, end); len(got) != 2 {
		t.Errorf("by date range June = %d, want 2", len(got))
	}
}

func TestTasksByStatus(t *testing.T) {
	s := newStore(t)
	_ = s.PutTask(domain.Task{ID: "k1", Title: "a", Status: domain.StatusOpen})
	_ = s.PutTask(domain.Task{ID: "k2", Title: "b", Status: domain.StatusDone})
	_ = s.PutTask(domain.Task{ID: "k3", Title: "c", Status: domain.StatusOpen})

	if got, _ := s.TasksByStatus(domain.StatusOpen); len(got) != 2 {
		t.Errorf("open tasks = %d, want 2", len(got))
	}
	if got, _ := s.TasksByStatus(domain.StatusDone); len(got) != 1 {
		t.Errorf("done tasks = %d, want 1", len(got))
	}
}

func mustMonth(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}
