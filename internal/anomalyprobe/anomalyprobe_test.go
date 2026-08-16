// SPDX-License-Identifier: MIT

package anomalyprobe

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var probeNow = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

func day(d int) time.Time { return time.Date(2026, time.August, d, 9, 0, 0, 0, time.UTC) }

func txn(id, payee string, minor int64, at time.Time) domain.Transaction {
	return domain.Transaction{
		ID: id, Payee: payee, AccountID: "a1", CategoryID: "c1",
		Amount: money.New(-minor, "USD"), Date: at,
	}
}

func money0(v int64) string    { return fmt.Sprintf("$%d.%02d", v/100, v%100) }
func date0(t time.Time) string { return t.Format("2 Jan") }

func catOf(id string) string {
	if id == "c1" {
		return "Groceries"
	}
	return ""
}

func TestGatherFindsTheRowsBehindAFlag(t *testing.T) {
	txns := []domain.Transaction{
		txn("t1", "Trader Joe's", 4210, day(14)),
		txn("t2", "Trader Joe's", 3900, day(2)),
		txn("t3", "Shell", 5000, day(10)),
	}
	ev := Gather(Finding{Feature: "SMART-T6", Payee: "Trader Joe's"}, txns, nil, catOf, probeNow)
	if len(ev.Related) != 2 {
		t.Fatalf("related = %d, want the two Trader Joe's rows", len(ev.Related))
	}
	// Newest first: the charge being asked about is nearly always the recent one.
	if !ev.Related[0].Date.After(ev.Related[1].Date) {
		t.Fatalf("rows are not newest-first: %v then %v", ev.Related[0].Date, ev.Related[1].Date)
	}
	if ev.Related[0].Category != "Groceries" {
		t.Fatalf("category = %q", ev.Related[0].Category)
	}
}

func TestATypicalCostIsTheMedianNotTheMean(t *testing.T) {
	// The whole point of the investigation is one unusual charge. A mean would let
	// that outlier redefine "typical" and quietly excuse itself.
	txns := []domain.Transaction{
		txn("t1", "Vet", 4000, day(1)),
		txn("t2", "Vet", 4500, day(2)),
		txn("t3", "Vet", 5000, day(3)),
		txn("t4", "Vet", 90000, day(4)), // the charge under investigation
	}
	ev := Gather(Finding{Payee: "Vet"}, txns, nil, catOf, probeNow)
	if ev.MerchantHistory == nil {
		t.Fatal("no merchant history")
	}
	if got := ev.MerchantHistory.TypicalMinor; got != 5000 {
		t.Fatalf("typical = %d — a mean would report ~%d and hide the outlier", got, (4000+4500+5000+90000)/4)
	}
	if ev.MerchantHistory.MaxMinor != 90000 || ev.MerchantHistory.MinMinor != 4000 {
		t.Fatalf("range = %d..%d", ev.MerchantHistory.MinMinor, ev.MerchantHistory.MaxMinor)
	}
}

func TestASingleChargeIsNotAHistory(t *testing.T) {
	// A "typical" derived from one observation is that observation wearing a hat.
	ev := Gather(Finding{Payee: "Vet"}, []domain.Transaction{txn("t1", "Vet", 4000, day(1))}, nil, catOf, probeNow)
	if ev.MerchantHistory != nil {
		t.Fatalf("history from a single charge: %+v", ev.MerchantHistory)
	}
}

func TestARecurringScheduleIsFoundAndNamed(t *testing.T) {
	// A flag with a schedule behind it is usually not a problem, which makes this
	// the single most useful thing the probe can find.
	rec := []domain.Recurring{{ID: "r1", Label: "Netflix subscription", Amount: money.New(-1599, "USD")}}
	ev := Gather(Finding{Payee: "Netflix"}, []domain.Transaction{txn("t1", "Netflix", 1599, day(3))}, rec, catOf, probeNow)
	if ev.RecurringMatch != "Netflix subscription" {
		t.Fatalf("recurring = %q", ev.RecurringMatch)
	}
}

func TestAPausedScheduleDoesNotExplainACharge(t *testing.T) {
	rec := []domain.Recurring{{ID: "r1", Label: "Netflix subscription", Paused: true, Amount: money.New(-1599, "USD")}}
	ev := Gather(Finding{Payee: "Netflix"}, []domain.Transaction{txn("t1", "Netflix", 1599, day(3))}, rec, catOf, probeNow)
	if ev.RecurringMatch != "" {
		t.Fatalf("a paused schedule was offered as the explanation: %q", ev.RecurringMatch)
	}
}

func TestTransfersAreNeverEvidence(t *testing.T) {
	// A transfer moved the household's own money; presenting it as a charge would
	// send the verdict in the wrong direction.
	transfer := txn("t1", "Savings", 50000, day(4))
	transfer.TransferAccountID = "a2"
	txns := []domain.Transaction{transfer, txn("t2", "Savings", 1000, day(5))}
	ev := Gather(Finding{Payee: "Savings"}, txns, nil, catOf, probeNow)
	for _, r := range ev.Related {
		if r.AmountMinor == 50000 {
			t.Fatal("a transfer was gathered as evidence")
		}
	}
}

func TestABriefWithNothingToShowSaysSo(t *testing.T) {
	// Silence would let the model answer as though it had evidence. Saying the
	// lookup came back empty tells the reader the flag stands on the detector's
	// own reasoning alone.
	ev := Gather(Finding{Payee: "Nobody"}, nil, nil, catOf, probeNow)
	brief := ev.Brief(Finding{Title: "Something looks odd"}, money0, date0)
	if !strings.Contains(brief, "Nothing else in their data") {
		t.Fatalf("brief = %q", brief)
	}
}

func TestAFindingWithNothingToSearchOnGathersNothing(t *testing.T) {
	// Guessing at relevance from the headline's wording would fill the brief with
	// unrelated rows, which is worse than an empty brief.
	ev := Gather(Finding{Title: "Spending is up"}, []domain.Transaction{txn("t1", "Anything", 100, day(1))}, nil, catOf, probeNow)
	if len(ev.Related) != 0 || ev.MerchantHistory != nil {
		t.Fatalf("evidence invented for an unanchored finding: %+v", ev)
	}
}

func TestTheBriefAsksForAVerdictNotASummary(t *testing.T) {
	// A brief that only supplies facts gets a summary of the facts back, which the
	// user can already read on the row they clicked.
	ev := Gather(Finding{Payee: "Vet"}, []domain.Transaction{
		txn("t1", "Vet", 4000, day(1)), txn("t2", "Vet", 90000, day(4)),
	}, nil, catOf, probeNow)
	brief := ev.Brief(Finding{Title: "Unusual charge at Vet", Detail: "much larger than usual"}, money0, date0)
	for _, want := range []string{"verdict", "one concrete thing", "Use only the figures above"} {
		if !strings.Contains(brief, want) {
			t.Errorf("brief does not ask for %q:\n%s", want, brief)
		}
	}
	if !strings.Contains(brief, "Unusual charge at Vet") {
		t.Errorf("brief omits the finding itself:\n%s", brief)
	}
	if !strings.Contains(brief, "$900.00") {
		t.Errorf("brief omits the figure under investigation:\n%s", brief)
	}
}

func TestTheBriefIsBoundedAndSaysWhenItTruncated(t *testing.T) {
	var txns []domain.Transaction
	for i := 1; i <= 20; i++ {
		txns = append(txns, txn(fmt.Sprintf("t%d", i), "Vet", int64(1000+i), day(i%28+1)))
	}
	ev := Gather(Finding{Payee: "Vet"}, txns, nil, catOf, probeNow)
	if len(ev.Related) != maxRelated {
		t.Fatalf("related = %d, want it capped at %d", len(ev.Related), maxRelated)
	}
	if !ev.Truncated {
		t.Fatal("more rows matched than were carried, but the brief does not say so")
	}
	if !strings.Contains(ev.Brief(Finding{Title: "x"}, money0, date0), "only the most recent") {
		t.Fatal("a truncated brief implies it saw everything")
	}
}

func TestOldChargesDoNotSkewTheTypicalCost(t *testing.T) {
	// A charge from three years ago says nothing about what this merchant costs
	// now, and including it would make an ordinary rise look like an anomaly.
	txns := []domain.Transaction{
		txn("t1", "Gym", 3000, probeNow.AddDate(-3, 0, 0)),
		txn("t2", "Gym", 5000, day(1)),
		txn("t3", "Gym", 5100, day(2)),
	}
	ev := Gather(Finding{Payee: "Gym"}, txns, nil, catOf, probeNow)
	if ev.MerchantHistory == nil {
		t.Fatal("no history")
	}
	if ev.MerchantHistory.Count != 2 {
		t.Fatalf("history counted %d charges, want only the two inside the window", ev.MerchantHistory.Count)
	}
}

func TestPayeeInMatchesAgainstRealMerchantsOnly(t *testing.T) {
	payees := []string{"Trader Joe's", "Shell", "Amazon", "Amazon Prime"}
	// The longest match wins, so a household with both gets the specific one.
	if got := PayeeIn("Amazon Prime charged twice this month", payees); got != "Amazon Prime" {
		t.Fatalf("payee = %q, want the more specific merchant", got)
	}
	if got := PayeeIn("Spending at Trader Joe's is up", payees); got != "Trader Joe's" {
		t.Fatalf("payee = %q", got)
	}
	// A finding about nothing the household has ever paid finds nothing, rather
	// than inventing a merchant from a sentence fragment.
	if got := PayeeIn("Your balance dipped before payday", payees); got != "" {
		t.Fatalf("payee = %q, want none", got)
	}
}

func TestPayeeInIgnoresUselesslyShortNames(t *testing.T) {
	// A two-letter merchant matches half the words in any sentence and would
	// attach the probe to the wrong rows constantly.
	if got := PayeeIn("Spending is up this month", []string{"BP", "is"}); got != "" {
		t.Fatalf("payee = %q, want none", got)
	}
}
