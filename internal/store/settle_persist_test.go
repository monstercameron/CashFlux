package store

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func sampleSharedExpense() domain.SharedExpense {
	return domain.SharedExpense{
		ID: "se1", Desc: "Costco run", PayerID: "priya",
		Date: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Shares: []domain.SharedExpenseShare{
			{MemberID: "priya", Amount: money.New(3000, "USD")},
			{MemberID: "sam", Amount: money.New(3000, "USD")},
			{MemberID: "lee", Amount: money.New(3000, "USD")},
		},
	}
}

func TestSharedExpenseCRUD(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()

	e := sampleSharedExpense()
	if err := st.PutSharedExpense(e); err != nil {
		t.Fatalf("PutSharedExpense: %v", err)
	}
	got, ok, err := st.GetSharedExpense("se1")
	if err != nil || !ok {
		t.Fatalf("GetSharedExpense: ok=%v err=%v", ok, err)
	}
	if got.PayerID != "priya" || len(got.Shares) != 3 || got.Total().Amount != 9000 {
		t.Errorf("round-tripped expense wrong: %+v (total %d)", got, got.Total().Amount)
	}

	st2 := domain.Settlement{ID: "s1", FromID: "sam", ToID: "priya", Amount: money.New(3000, "USD")}
	if err := st.PutSettlement(st2); err != nil {
		t.Fatalf("PutSettlement: %v", err)
	}
	settlements, err := st.ListSettlements()
	if err != nil || len(settlements) != 1 || settlements[0].Amount.Amount != 3000 {
		t.Errorf("ListSettlements = %+v err=%v", settlements, err)
	}

	if ok, err := st.DeleteSharedExpense("se1"); err != nil || !ok {
		t.Errorf("DeleteSharedExpense ok=%v err=%v", ok, err)
	}
	if list, _ := st.ListSharedExpenses(); len(list) != 0 {
		t.Errorf("expense not deleted: %+v", list)
	}
}

func TestSharedExpenseExportImportRoundTrip(t *testing.T) {
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	defer st.Close()
	if err := st.PutSharedExpense(sampleSharedExpense()); err != nil {
		t.Fatalf("put expense: %v", err)
	}
	if err := st.PutSettlement(domain.Settlement{ID: "s1", FromID: "sam", ToID: "priya", Amount: money.New(3000, "USD")}); err != nil {
		t.Fatalf("put settlement: %v", err)
	}

	ds, err := st.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	blob, err := Export(ds)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	imported, err := Import(blob)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(imported.SharedExpenses) != 1 || len(imported.Settlements) != 1 {
		t.Fatalf("round-trip lost records: %d expenses, %d settlements", len(imported.SharedExpenses), len(imported.Settlements))
	}
	if imported.SharedExpenses[0].Total().Amount != 9000 {
		t.Errorf("expense total after round-trip = %d, want 9000", imported.SharedExpenses[0].Total().Amount)
	}
	if imported.Settlements[0].FromID != "sam" || imported.Settlements[0].ToID != "priya" {
		t.Errorf("settlement after round-trip wrong: %+v", imported.Settlements[0])
	}
}
