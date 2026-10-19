// SPDX-License-Identifier: MIT

package regularspend

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/debtpay"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCadenceNormalization(t *testing.T) {
	tests := []struct {
		cadence      domain.RecurringCadence
		amount, want int64
	}{
		{domain.CadenceWeekly, 10000, 43333},
		{domain.CadenceBiweekly, 10000, 21667},
		{domain.CadenceMonthly, 10000, 10000},
		{domain.CadenceSemimonthly, 10000, 20000},
		{domain.CadenceQuarterly, 10000, 3333},
		{domain.CadenceYearly, 120000, 10000},
	}
	for _, tt := range tests {
		t.Run(string(tt.cadence), func(t *testing.T) {
			got, err := normalizeCadence(tt.amount, tt.cadence)
			if err != nil || got != tt.want {
				t.Fatalf("normalizeCadence = %d, %v; want %d", got, err, tt.want)
			}
		})
	}
	if _, err := normalizeCadence(500, "mystery"); err == nil {
		t.Fatal("unknown cadence was silently estimated")
	}
}

func TestBuildSampleHouseholdExplained(t *testing.T) {
	asof := date("2026-10-15")
	cats := []domain.Category{{ID: "needs", Name: "Household needs", Kind: domain.KindExpense}, {ID: "food", Name: "Groceries", Kind: domain.KindExpense, ParentID: "needs"}}
	rent := recurring("rent", "Rent", -120000, "USD", domain.CadenceMonthly, "2026-10-01")
	annual := recurring("insurance", "Home insurance", -120000, "USD", domain.CadenceYearly, "2027-03-10")
	quarter := recurring("water", "Water bill", -30000, "USD", domain.CadenceQuarterly, "2026-11-05")
	minimum := recurring("card-min", "Card minimum", -5000, "USD", domain.CadenceMonthly, "2026-10-20")
	transactions := []domain.Transaction{}
	amounts := []int64{50000, 52000, 51000, 100000, 48000, 53000}
	months := []string{"2026-04-12", "2026-05-12", "2026-06-12", "2026-07-12", "2026-08-12", "2026-09-12"}
	for i, dt := range months {
		transactions = append(transactions, txn("grocery-"+months[i], "food", -amounts[i], "USD", dt))
	}
	// One refund offsets 2,000 of June groceries, leaving 49,000 of net spend.
	transactions = append(transactions, txn("refund", "food", 2000, "USD", "2026-06-20"))
	// A one-off override affects only this baseline, and the row remains otherwise valid.
	transactions = append(transactions, txn("repair", "food", -40000, "USD", "2026-09-21"))
	transactions = append(transactions, txn("transfer", "food", -90000, "USD", "2026-09-22"))
	transactions[len(transactions)-1].TransferAccountID = "savings"
	transactions = append(transactions, txn("ignored-current", "food", -90000, "USD", "2026-10-01"))
	accounts := []domain.Account{{ID: "card", Name: "Card", Class: domain.ClassLiability, MinPayment: money.New(5000, "USD"), OwnerID: "m1"}}
	in := Input{AsOf: asof, Categories: cats, Transactions: transactions, Accounts: accounts, Rates: Rates{Base: "USD"}, Overrides: Overrides{EssentialCategoryIDs: []string{"needs"}, OneOffTransactionIDs: []string{"repair"}}, Schedules: []Schedule{
		{Recurring: rent, Confirmed: true, Owners: []string{"m1", "m2"}, TransactionIDs: []string{"historical-rent"}},
		{Recurring: annual, Confirmed: true},
		{Recurring: quarter, Confirmed: true},
		{Recurring: minimum, Confirmed: true, DebtAccountID: "card"},
	}, DetectedSeries: []DetectedSeries{{ID: "detected-rent-alias", Name: "Rent", Amount: money.New(120000, "USD"), Cadence: domain.CadenceMonthly, PaymentCount: 6, Confirmed: true, ScheduleID: "rent"}}}
	got, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	// Fixed = rent 1,200.00 + annual insurance reserve 100.00 + quarterly water reserve 100.00 + card minimum 50.00.
	if got.Fixed.Base.Amount != 145000 {
		t.Fatalf("Fixed = %d cents, want 145000", got.Fixed.Base.Amount)
	}
	// Grocery totals are 500, 520, 490 (after the refund), 1000, 480, and 530 dollars.
	// The one-off repair is ignored; the sorted middle pair is 500 and 520, median 510.00.
	// Therefore the total is 1,450 + 510 = 1,960.00.
	if got.TypicalEssential.Base.Amount != 196000 {
		t.Fatalf("TypicalEssential = %d cents, want 196000", got.TypicalEssential.Base.Amount)
	}
	if len(got.Contributors) != 5 {
		t.Fatalf("contributors = %d, want 5 (four fixed + one variable)", len(got.Contributors))
	}
	find := func(id string) Contributor {
		for _, line := range got.Contributors {
			if line.ID == id {
				return line
			}
		}
		t.Fatalf("missing contributor %q", id)
		return Contributor{}
	}
	if find("insurance").MonthlyEquivalent.Amount != 10000 || len(find("insurance").DueMonths) != 1 || find("insurance").DueMonths[0] != time.March {
		t.Fatalf("annual line not explained correctly: %+v", find("insurance"))
	}
	if find("water").MonthlyEquivalent.Amount != 10000 || len(find("water").DueMonths) != 4 {
		t.Fatalf("quarterly line not explained correctly: %+v", find("water"))
	}
	if find("essential:food:USD").Evidence != "median of 6 complete months" || find("essential:food:USD").MonthlyEquivalent.Amount != 51000 {
		t.Fatalf("unexpected evidence: %+v", find("essential:food:USD"))
	}
	shared := AllocateContributor(find("rent"))
	if shared.Shared.Amount != find("rent").MonthlyEquivalent.Amount || shared.ByMember["m1"].Amount+shared.ByMember["m2"].Amount != find("rent").MonthlyEquivalent.Amount {
		t.Fatalf("shared rent allocation changed household total: %+v", shared)
	}
	if len(got.SuggestedEssentials) != 1 || got.SuggestedEssentials[0] != "food" {
		t.Fatalf("suggested categories = %v", got.SuggestedEssentials)
	}
}

func TestBuildFixedDedupeActivityAndUnknowns(t *testing.T) {
	asof := date("2026-10-15")
	active := recurring("active", "Active", -1200, "USD", domain.CadenceMonthly, "2026-10-20")
	paused := recurring("paused", "Paused", -800, "USD", domain.CadenceMonthly, "2026-10-20")
	paused.Paused = true
	ended := recurring("ended", "Ended", -700, "USD", domain.CadenceMonthly, "2026-10-20")
	unknown := recurring("unknown", "Unknown", -500, "USD", "", "2026-10-20")
	in := Input{AsOf: asof, Rates: Rates{Base: "USD"}, Schedules: []Schedule{{Recurring: active, Confirmed: true}, {Recurring: paused, Confirmed: true}, {Recurring: ended, Confirmed: true, EndedAt: date("2026-10-01")}, {Recurring: unknown, Confirmed: true}}, DetectedSeries: []DetectedSeries{{ID: "alias", Name: "Active", Amount: money.New(1200, "USD"), Cadence: domain.CadenceMonthly, Confirmed: true, ScheduleID: "active"}, {ID: "paused-series", Name: "Paused series", Amount: money.New(900, "USD"), Cadence: domain.CadenceMonthly, Confirmed: true, Paused: true}, {ID: "unconfirmed", Name: "No approval", Amount: money.New(700, "USD"), Cadence: domain.CadenceMonthly}}}
	got, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fixed.Base.Amount != 1200 || len(got.Contributors) != 2 {
		t.Fatalf("fixed/dedupe = %d / %d, want 1200 / 2 (including the unestimated row)", got.Fixed.Base.Amount, len(got.Contributors))
	}
	for _, line := range got.Contributors {
		if line.ID == "unknown" && !line.Unestimated {
			t.Fatalf("unknown cadence not marked unestimated: %+v", line)
		}
	}
	if len(got.DataQualityNotes) == 0 {
		t.Fatal("missing unknown cadence note")
	}
}

func TestBuildHistorySparseAndCurrentMonthExcluded(t *testing.T) {
	cat := domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}
	three := []domain.Transaction{txn("a", "food", -1000, "USD", "2026-05-01"), txn("b", "food", -2000, "USD", "2026-07-01"), txn("c", "food", -3000, "USD", "2026-09-01"), txn("d", "food", -90000, "USD", "2026-10-01")}
	got, err := Build(Input{AsOf: date("2026-10-15"), Categories: []domain.Category{cat}, Transactions: three, Overrides: Overrides{EssentialCategoryIDs: []string{"food"}}, Rates: Rates{Base: "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.TypicalEssential.Base.Amount != 2000 {
		t.Fatalf("median/current month = %d, want 2000", got.TypicalEssential.Base.Amount)
	}
	if len(got.Contributors) != 1 || got.Contributors[0].SampleSize != 3 {
		t.Fatalf("sparse sample not reported: %+v", got.Contributors)
	}
	got, err = Build(Input{AsOf: date("2026-10-15"), Categories: []domain.Category{cat}, Transactions: three[:2], Overrides: Overrides{EssentialCategoryIDs: []string{"food"}}, Rates: Rates{Base: "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.TypicalEssential.Base.Amount != 0 || len(got.Contributors) != 0 {
		t.Fatalf("two months should remain unestimated: %+v", got)
	}
}

func TestBuildFXMissingAndRationalConversion(t *testing.T) {
	s := recurring("eur", "Euro bill", -10000, "EUR", domain.CadenceMonthly, "2026-10-01")
	got, err := Build(Input{AsOf: date("2026-10-15"), Rates: Rates{Base: "USD"}, Schedules: []Schedule{{Recurring: s, Confirmed: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Fixed.Incomplete || got.Fixed.Base.Amount != 0 || got.Fixed.ByCurrency["EUR"].Amount != 10000 {
		t.Fatalf("missing FX must preserve source subtotal: %+v", got.Fixed)
	}
	got, err = Build(Input{AsOf: date("2026-10-15"), Rates: Rates{Base: "USD", ByCurrency: map[string]Rate{"EUR": {Numerator: 11, Denominator: 10}}}, Schedules: []Schedule{{Recurring: s, Confirmed: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Fixed.Base.Amount != 11000 || got.Fixed.Incomplete {
		t.Fatalf("rational conversion = %+v", got.Fixed)
	}
}

func TestEssentialMedianConvertsToBaseWithRateEvidence(t *testing.T) {
	cat := domain.Category{ID: "food", Name: "Food", Kind: domain.KindExpense}
	txns := []domain.Transaction{txn("eur-apr", "food", -1000, "EUR", "2026-04-05"), txn("eur-may", "food", -1000, "EUR", "2026-05-05"), txn("eur-jun", "food", -1000, "EUR", "2026-06-05")}
	got, err := Build(Input{AsOf: date("2026-10-15"), Categories: []domain.Category{cat}, Transactions: txns, Overrides: Overrides{EssentialCategoryIDs: []string{"food"}}, Rates: Rates{Base: "USD", ByCurrency: map[string]Rate{"EUR": {Numerator: 11, Denominator: 10}}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.TypicalEssential.Base.Amount != 1100 || got.TypicalEssential.Incomplete {
		t.Fatalf("converted essential total: %+v", got.TypicalEssential)
	}
	if len(got.Contributors) != 1 || !got.Contributors[0].HasConversionRate || got.Contributors[0].ConversionRate.Numerator != 11 {
		t.Fatalf("conversion evidence missing: %+v", got.Contributors)
	}
}

func TestBuildOwnershipConservation(t *testing.T) {
	for _, amt := range []int64{101, -101, 100} {
		allocation := AllocateToMembers(amt, "USD", []string{"m1", "m2", "m3"})
		sum := int64(0)
		for _, v := range allocation.ByMember {
			sum += v.Amount
		}
		if sum != amt {
			t.Fatalf("member shares sum %d, want %d", sum, amt)
		}
		if allocation.Shared.Amount != amt {
			t.Fatalf("shared marker lost household amount: %+v", allocation)
		}
	}
	u := AllocateToMembers(525, "USD", nil)
	if u.Unassigned.Amount != 525 {
		t.Fatalf("unassigned amount = %+v", u)
	}
}

func TestSeasonalNoteOnTwelveMonthView(t *testing.T) {
	cat := domain.Category{ID: "fuel", Name: "Fuel", Kind: domain.KindExpense}
	txns := []domain.Transaction{}
	for i := 0; i < 12; i++ {
		d := date("2025-10-01").AddDate(0, i, 0)
		txns = append(txns, domain.Transaction{ID: d.Format("2006-01"), Date: d, CategoryID: "fuel", Amount: money.New(-100, "USD")})
	}
	got, err := Build(Input{AsOf: date("2026-10-15"), HistoryMonths: 12, Categories: []domain.Category{cat}, Transactions: txns, Overrides: Overrides{EssentialCategoryIDs: []string{"fuel"}}, Rates: Rates{Base: "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Contributors) != 1 || !got.Contributors[0].SeasonalUnavailable {
		t.Fatalf("seasonal availability note missing: %+v", got.Contributors)
	}
}

func TestDebtPaymentDoesNotDoubleCountCardPurchases(t *testing.T) {
	card := domain.Account{ID: "card", Name: "Card", Class: domain.ClassLiability, MinPayment: money.New(5000, "USD")}
	purchases := []domain.Transaction{txn("purchase-jul", "food", -12000, "USD", "2026-07-01"), txn("purchase-aug", "food", -12000, "USD", "2026-08-01"), txn("purchase-sep", "food", -12000, "USD", "2026-09-01")}
	for i := range purchases {
		purchases[i].AccountID = "card"
	}
	payment := txn("payment", "food", -5000, "USD", "2026-09-15")
	payment.TransferAccountID = "card"
	purchases = append(purchases, payment)
	got, err := Build(Input{AsOf: date("2026-10-15"), Categories: []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}}, Transactions: purchases, Accounts: []domain.Account{card}, Overrides: Overrides{EssentialCategoryIDs: []string{"food"}}, Rates: Rates{Base: "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Fixed.Base.Amount != 5000 {
		t.Fatalf("card minimum not included once: %+v", got.Fixed)
	}
	if got.TypicalEssential.Base.Amount != 17000 {
		t.Fatalf("purchase + minimum expected, card payment must not count: %+v", got.TypicalEssential)
	}
}

func TestClassifiedSingleLegDebtPaymentIsHeldForReview(t *testing.T) {
	transactions := []domain.Transaction{
		txn("food-jul", "food", -10000, "USD", "2026-07-01"),
		txn("food-aug", "food", -10000, "USD", "2026-08-01"),
		txn("food-sep", "food", -10000, "USD", "2026-09-01"),
		{ID: "candidate", AccountID: "checking", Date: date("2026-09-15"), Payee: "Card Co", CategoryID: "food", Amount: money.New(-5000, "USD")},
	}
	data := debtpay.Dataset{
		Accounts:   []debtpay.Account{{ID: "checking", Currency: "USD"}, {ID: "card", Name: "Card", Currency: "USD", Liability: true}},
		PayeeRules: []debtpay.PayeeRule{{Payee: "Card Co", DebtAccountID: "card"}},
	}
	got, err := Build(Input{AsOf: date("2026-10-15"), Categories: []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}}, Transactions: transactions, DebtPaymentData: data, Overrides: Overrides{EssentialCategoryIDs: []string{"food"}}, Rates: Rates{Base: "USD"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.TypicalEssential.Base.Amount != 10000 {
		t.Fatalf("classified debt payment should not be counted as new purchase spending: %+v", got.TypicalEssential)
	}
	if len(got.DataQualityNotes) == 0 {
		t.Fatal("classified single-leg payment lacks a review note")
	}
}

func TestMerchantOverrideConfirmsDetectedSeries(t *testing.T) {
	series := DetectedSeries{ID: "net", Name: "Example Stream", Amount: money.New(1299, "USD"), Cadence: domain.CadenceMonthly, PaymentCount: 5}
	got, err := Build(Input{AsOf: date("2026-10-15"), Rates: Rates{Base: "USD"}, DetectedSeries: []DetectedSeries{series}, Overrides: Overrides{TreatMerchantRegular: []string{"example stream"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Fixed.Base.Amount != 1299 || len(got.Contributors) != 1 {
		t.Fatalf("merchant override did not confirm detected series: %+v", got)
	}
}

func recurring(id, label string, amount int64, currency string, cadence domain.RecurringCadence, due string) domain.Recurring {
	return domain.Recurring{ID: id, Label: label, Amount: money.New(amount, currency), Cadence: cadence, NextDue: date(due)}
}
func txn(id, category string, amount int64, currency, dt string) domain.Transaction {
	return domain.Transaction{ID: id, Date: date(dt), CategoryID: category, Amount: money.New(amount, currency)}
}
func date(value string) time.Time {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return t
}
