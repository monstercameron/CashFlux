package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func TestAnnualAmounts(t *testing.T) {
	accounts := []domain.Account{
		// Qualifying liability: monthly statement → minPayment ×12.
		{ID: "card", Class: domain.ClassLiability, DueDayOfMonth: 1, MinPayment: money.New(-10000, "USD")},
		// Skipped: asset, no due-day, zero min payment, archived.
		{ID: "savings", Class: domain.ClassAsset, MinPayment: money.New(-10000, "USD")},
		{ID: "loan-nodue", Class: domain.ClassLiability, DueDayOfMonth: 0, MinPayment: money.New(-5000, "USD")},
		{ID: "loan-nomin", Class: domain.ClassLiability, DueDayOfMonth: 5, MinPayment: money.New(0, "USD")},
		{ID: "archived", Class: domain.ClassLiability, DueDayOfMonth: 5, MinPayment: money.New(-1000, "USD"), Archived: true},
	}
	recurring := []domain.Recurring{
		{ID: "rent", Label: "Rent", Amount: money.New(-120000, "USD"), Cadence: domain.CadenceMonthly}, // ×12 = 1,440,000
		{ID: "gym", Label: "Gym", Amount: money.New(-1200, "USD"), Cadence: domain.CadenceWeekly},      // ×52 = 62,400
		{ID: "tax", Label: "Tax", Amount: money.New(-30000, "USD"), Cadence: domain.CadenceQuarterly},  // ×4 = 120,000
		{ID: "dom", Label: "Domain", Amount: money.New(-9000, "USD"), Cadence: domain.CadenceYearly},   // ×1 = 9,000
		{ID: "sal", Label: "Salary", Amount: money.New(500000, "USD"), Cadence: domain.CadenceMonthly}, // income → skipped
	}

	got := AnnualAmounts(accounts, recurring)
	var total int64
	for _, m := range got {
		if m.Currency != "USD" {
			t.Errorf("currency = %q, want USD", m.Currency)
		}
		if m.Amount < 0 {
			t.Errorf("annual amount should be positive, got %d", m.Amount)
		}
		total += m.Amount
	}
	// liability 10,000×12=120,000 + rent 1,440,000 + gym 62,400 + tax 120,000 + domain 9,000.
	want := int64(120_000 + 1_440_000 + 62_400 + 120_000 + 9_000)
	if total != want {
		t.Errorf("annual total = %d, want %d (entries: %v)", total, want, got)
	}
	if len(got) != 5 {
		t.Errorf("AnnualAmounts len = %d, want 5 (1 liability + 4 negative recurring)", len(got))
	}
}

func TestNextDue(t *testing.T) {
	cases := []struct {
		name   string
		dueDay int
		from   time.Time
		want   time.Time
	}{
		{"later this month", 15, d(2026, time.June, 10), d(2026, time.June, 15)},
		{"due today", 15, d(2026, time.June, 15), d(2026, time.June, 15)},
		{"already passed -> next month", 15, d(2026, time.June, 20), d(2026, time.July, 15)},
		{"month-end clamp Feb non-leap", 31, d(2026, time.February, 5), d(2026, time.February, 28)},
		{"month-end clamp Feb leap", 31, d(2024, time.February, 5), d(2024, time.February, 29)},
		{"31st in a 31-day month", 31, d(2026, time.January, 5), d(2026, time.January, 31)},
		{"year rollover", 10, d(2026, time.December, 20), d(2027, time.January, 10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextDue(tc.dueDay, tc.from); !got.Equal(tc.want) {
				t.Errorf("NextDue(%d, %s) = %s, want %s", tc.dueDay,
					tc.from.Format("2006-01-02"), got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func liability(id, name string, dueDay int, minPay int64) domain.Account {
	return domain.Account{
		ID: id, Name: name, Class: domain.ClassLiability,
		DueDayOfMonth: dueDay, MinPayment: money.New(minPay, "USD"),
	}
}

func TestUpcoming(t *testing.T) {
	now := d(2026, time.June, 10)
	accounts := []domain.Account{
		liability("card", "Visa", 15, 5000),     // due Jun 15
		liability("loan", "Car loan", 1, 30000), // due Jul 1 (the 1st already passed this month)
		{ID: "checking", Name: "Checking", Class: domain.ClassAsset},
		liability("nodue", "No due day", 0, 5000),   // no due day → skipped
		liability("nopay", "No min payment", 20, 0), // no min payment → skipped
		func() domain.Account { a := liability("arch", "Archived", 5, 1000); a.Archived = true; return a }(),
	}
	got := Upcoming(accounts, now)
	if len(got) != 2 {
		t.Fatalf("got %d bills, want 2 (Visa, Car loan): %+v", len(got), got)
	}
	// Soonest first: Visa Jun 15 before Car loan Jul 1.
	if got[0].AccountID != "card" || !got[0].DueDate.Equal(d(2026, time.June, 15)) || got[0].DaysUntil != 5 {
		t.Errorf("bill 0 = %+v, want Visa due Jun 15 in 5 days", got[0])
	}
	if got[1].AccountID != "loan" || !got[1].DueDate.Equal(d(2026, time.July, 1)) {
		t.Errorf("bill 1 = %+v, want Car loan due Jul 1", got[1])
	}
	if got[0].Amount.Amount != 5000 {
		t.Errorf("Visa amount = %d, want 5000", got[0].Amount.Amount)
	}
}

func TestUpcomingAllIncludesRecurringOutflows(t *testing.T) {
	now := d(2026, time.June, 10)
	accounts := []domain.Account{
		liability("card", "Visa", 15, 5000),
	}
	recurring := []domain.Recurring{
		{
			ID:      "rent",
			Label:   "Rent",
			Amount:  money.New(-150000, "USD"),
			Cadence: domain.CadenceMonthly,
			NextDue: d(2026, time.June, 1),
		},
		{
			ID:      "paycheck",
			Label:   "Paycheck",
			Amount:  money.New(250000, "USD"),
			Cadence: domain.CadenceMonthly,
			NextDue: d(2026, time.June, 14),
		},
		{
			ID:      "gym",
			Label:   "Gym",
			Amount:  money.New(-3000, "USD"),
			Cadence: domain.CadenceWeekly,
			NextDue: d(2026, time.June, 8),
		},
	}

	got := UpcomingAll(accounts, recurring, now)
	if len(got) != 3 {
		t.Fatalf("got %d bills, want account + 2 recurring outflows: %+v", len(got), got)
	}
	if got[0].Name != "Visa" || !got[0].DueDate.Equal(d(2026, time.June, 15)) {
		t.Fatalf("first = %+v, want Visa on Jun 15 by id tie-break", got[0])
	}
	if got[1].Name != "Gym" || !got[1].DueDate.Equal(d(2026, time.June, 15)) {
		t.Fatalf("second = %+v, want weekly gym advanced to Jun 15", got[1])
	}
	if got[2].Name != "Rent" || !got[2].DueDate.Equal(d(2026, time.July, 1)) || got[2].Amount.Amount != 150000 {
		t.Fatalf("third = %+v, want rent advanced to Jul 1 with positive amount", got[2])
	}
	for _, b := range got {
		if b.Name == "Paycheck" {
			t.Fatal("income recurring item should not become a bill")
		}
	}
}

func TestUpcomingAllSkipsInvalidRecurring(t *testing.T) {
	now := d(2026, time.June, 10)
	got := UpcomingAll(nil, []domain.Recurring{
		{ID: "", Label: "No ID", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
		{ID: "nolabel", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly, NextDue: now},
		{ID: "nodue", Label: "No due", Amount: money.New(-100, "USD"), Cadence: domain.CadenceMonthly},
	}, now)
	if len(got) != 0 {
		t.Fatalf("invalid recurring produced bills: %+v", got)
	}
}
