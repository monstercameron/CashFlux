// SPDX-License-Identifier: MIT

package householdview

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

func fixture() Input {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	account := func(id, owner string, value int64) domain.Account {
		return domain.Account{ID: id, OwnerID: owner, Currency: "USD", OpeningBalance: money.New(value, "USD")}
	}
	joint := account("joint", domain.GroupOwnerID, 10000)
	joint.OwnershipShares = map[string]int{"alice": 60, "bob": 40}
	return Input{Rates: currency.Rates{Base: "USD"}, PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0),
		Members:  []domain.Member{{ID: "bob", Name: "Bob"}, {ID: "alice", Name: "Alice"}},
		Accounts: []domain.Account{account("solo", "alice", 10000), joint, account("liability", "bob", -3000), account("shared", domain.GroupOwnerID, 1000), account("orphan", "ghost", 700)},
		Transactions: []domain.Transaction{
			{ID: "income", AccountID: "solo", Date: start, Amount: money.New(1001, "USD")},
			{ID: "sharedexpense", AccountID: "joint", Date: start, Amount: money.New(-1001, "USD"), MemberIDs: []string{"bob", "alice"}},
			{ID: "unattributed", AccountID: "shared", Date: start, Amount: money.New(-300, "USD")},
			{ID: "end", AccountID: "liability", Date: start.AddDate(0, 1, 0), Amount: money.New(-999, "USD"), MemberID: "bob"},
			{ID: "ghost", AccountID: "orphan", Date: start, Amount: money.New(-50, "USD"), MemberID: "ghost"},
			{ID: "before", AccountID: "solo", Date: start.Add(-time.Nanosecond), Amount: money.New(200, "USD")},
			{ID: "excluded", AccountID: "solo", Date: start, Amount: money.New(-5, "USD"), MemberID: "alice", ExcludeFromReports: true},
		},
	}
}

func TestPrepareKnownEmptyAndIncomeApplicability(t *testing.T) {
	in := fixture()
	in.Accounts = nil
	in.Transactions = nil
	in.Members = nil
	out := Prepare(in)
	if out.Worth.Err != nil || out.Worth.Total.Err != nil || out.Worth.Total.Value != money.Zero("USD") || out.Spending.Err != nil || out.Spending.Total.Value != money.Zero("USD") || out.Spending.Total.Err != nil {
		t.Fatalf("empty sources became unavailable: %+v", out)
	}
	if out.Income.Applicable || out.Income.Err != nil || !errors.Is(out.Income.Total.Err, ErrNoRecipients) || out.Holder.Kind != LeaderNone || out.Spender.Kind != LeaderNone {
		t.Fatalf("no recipients misreported as zero income or a leader: %+v", out)
	}
	if out.Worth.Owner("missing").Err != nil || out.Worth.Owner("missing").Value != money.Zero("USD") || out.Spending.Member("missing").Err != nil {
		t.Fatal("successful absent buckets must be known zero")
	}
	in.Members = []domain.Member{{ID: "one", Name: "One"}}
	out = Prepare(in)
	if !out.Income.Applicable || out.Income.Err != nil || out.Income.Total.Err != nil || out.Income.Total.Value != money.Zero("USD") || len(out.IncomeRows) != 1 || out.IncomeRows[0].Percent.Value != 0 {
		t.Fatalf("zero income with recipients became unavailable: %+v", out.Income)
	}
	in.Members = []domain.Member{{ID: domain.GroupOwnerID, Name: "Shared"}}
	if out = Prepare(in); out.Income.Applicable || !errors.Is(out.Income.Total.Err, ErrNoRecipients) {
		t.Fatalf("pseudo-member received income: %+v", out.Income)
	}
}

func TestPrepareCanonicalParityOrderedRowsAndCompleteBuckets(t *testing.T) {
	in := fixture()
	out := Prepare(in)
	wantWorth, err := ledger.NetByOwner(in.Accounts, in.Transactions, in.Rates)
	if err != nil {
		t.Fatal(err)
	}
	wantSpend, err := reports.SpendingByMember(in.Transactions, in.PeriodStart, in.PeriodEnd, in.Rates)
	if err != nil {
		t.Fatal(err)
	}
	wantIncome, err := allocate.SplitPeriodIncome(in.Transactions, in.Members, in.PeriodStart, in.PeriodEnd, "USD", in.Rates)
	if err != nil {
		t.Fatal(err)
	}
	if out.Worth.Err != nil || !reflect.DeepEqual(out.Worth.ByOwner, wantWorth) || out.Worth.Total.Err != nil || out.Worth.Total.Value.Amount != 17546 {
		t.Fatalf("worth diverged from canonical/complete household total: %+v", out.Worth)
	}
	for _, row := range wantSpend {
		if out.Spending.Member(row.MemberID).Value.Amount != row.Amount {
			t.Fatalf("spending diverged for %s", row.MemberID)
		}
	}
	if out.Spending.Err != nil || out.Spending.Total.Err != nil || out.Spending.Total.Value.Amount != 1351 || out.Spending.Member("ghost").Value.Amount != 50 || out.Worth.Owner("ghost").Value.Amount != 650 {
		t.Fatalf("orphan/unattributed amounts dropped: %+v", out.Spending)
	}
	if !reflect.DeepEqual(out.Income.Splits, wantIncome) || out.Income.Total.Value.Amount != 1001 || out.Income.Total.Err != nil {
		t.Fatalf("income split diverged: %+v", out.Income)
	}
	if len(out.Members) != 2 || out.Members[0].ID != "bob" || out.Members[1].ID != "alice" || out.IncomeRows[0].MemberID != "alice" || out.IncomeRows[1].MemberID != "bob" {
		t.Fatal("roster order or canonical income tie order changed")
	}
	if len(out.WorthRows) != 3 || out.WorthRows[2].Kind != RowShared || out.WorthRows[2].MemberID != domain.GroupOwnerID || len(out.SpendRows) != 3 || out.SpendRows[2].MemberID != "" {
		t.Fatal("analytics shared buckets changed")
	}
	if out.Holder.Kind != LeaderMember || out.Holder.MemberID != "alice" || out.Holder.Err != nil || out.Spender.Kind != LeaderMember || out.Spender.MemberID != "bob" || out.Spender.Err != nil {
		t.Fatalf("takeaway decisions changed: %+v %+v", out.Holder, out.Spender)
	}
	if out.Members[1].WorthPercent.Value != 100 || out.Members[0].WorthPercent.Value != 2 || out.SpendRows[0].Percent.Value != 100 || out.SpendRows[1].Percent.Value != 99 {
		t.Fatal("prepared bar arithmetic changed ordinary readings")
	}
}

func TestPrepareFacetFailuresAreIndependent(t *testing.T) {
	readErr := errors.New("unavailable collection")
	for _, tc := range []struct {
		name                         string
		change                       func(*Input)
		worth, spend, income, roster bool
		identity                     error
	}{
		{"settings", func(in *Input) { in.Errors.Settings = readErr }, true, true, true, false, readErr},
		{"accounts", func(in *Input) { in.Errors.Accounts = readErr }, true, false, false, false, readErr},
		{"transactions", func(in *Input) { in.Errors.Transactions = readErr }, true, true, true, false, readErr},
		{"members", func(in *Input) { in.Errors.Members = readErr }, false, false, true, true, readErr},
		{"period", func(in *Input) { in.PeriodEnd = in.PeriodStart }, false, true, true, false, ErrPeriod},
		{"base", func(in *Input) { in.Rates.Base = "" }, true, true, true, false, ErrBaseCurrency},
		{"worth FX", func(in *Input) {
			in.Accounts = append(in.Accounts, domain.Account{ID: "foreign", Currency: "EUR", OpeningBalance: money.New(1, "EUR")})
		}, true, false, false, false, currency.ErrUnknownRate},
		{"expense FX", func(in *Input) {
			in.Transactions = append(in.Transactions, domain.Transaction{ID: "foreign", Date: in.PeriodStart, Amount: money.New(-1, "EUR")})
		}, false, true, false, false, currency.ErrUnknownRate},
		{"income FX", func(in *Input) {
			in.Transactions = append(in.Transactions, domain.Transaction{ID: "foreign", Date: in.PeriodStart, Amount: money.New(1, "EUR")})
		}, false, false, true, false, currency.ErrUnknownRate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := fixture()
			tc.change(&in)
			out := Prepare(in)
			if (out.Worth.Err != nil) != tc.worth || (out.Spending.Err != nil) != tc.spend || (out.Income.Err != nil) != tc.income || (out.RosterErr != nil) != tc.roster {
				t.Fatalf("failure leaked across facets: worth%v spend%v income%v roster%v", out.Worth.Err, out.Spending.Err, out.Income.Err, out.RosterErr)
			}
			for _, fact := range []struct {
				failed bool
				err    error
			}{{tc.worth, out.Worth.Err}, {tc.spend, out.Spending.Err}, {tc.income, out.Income.Err}, {tc.roster, out.RosterErr}} {
				if fact.failed && !errors.Is(fact.err, tc.identity) {
					t.Fatalf("error identity lost: %v", fact.err)
				}
			}
			if tc.worth && (out.Worth.Owner("missing").Err == nil || out.Worth.Total.Err == nil || out.Holder.Err == nil) {
				t.Fatal("failed owner readings became known zero/leader")
			}
			if tc.spend && (out.Spending.Member("missing").Err == nil || out.Spending.Total.Err == nil || out.Spender.Err == nil) {
				t.Fatal("failed spending became known zero/leader")
			}
			if tc.income && (len(out.IncomeRows) > 0 || out.Income.Total.Err == nil) {
				t.Fatal("failed income exposed successful rows/zero")
			}
			if tc.roster {
				if len(out.Members) > 0 || out.Holder.Err == nil || out.Spender.Err == nil {
					t.Fatal("failed roster exposed stale member rows/labels")
				}
				for _, row := range append(out.WorthRows, out.SpendRows...) {
					if row.Percent.Err == nil {
						t.Fatal("missing roster made an unavailable row scale confident")
					}
				}
			}
		})
	}
}

func TestPrepareKnownRowsSurviveTotalOverflow(t *testing.T) {
	in := fixture()
	in.Members = []domain.Member{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	in.Accounts = []domain.Account{{ID: "a", OwnerID: "a", Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")}, {ID: "b", OwnerID: "b", Currency: "USD", OpeningBalance: money.New(1, "USD")}}
	in.Transactions = []domain.Transaction{{ID: "big", Date: in.PeriodStart, MemberID: "a", Amount: money.New(-math.MaxInt64, "USD")}, {ID: "one", Date: in.PeriodStart, MemberID: "b", Amount: money.New(-1, "USD")}}
	out := Prepare(in)
	if out.Worth.Err != nil || out.Spending.Err != nil || !errors.Is(out.Worth.Total.Err, money.ErrOverflow) || !errors.Is(out.Spending.Total.Err, money.ErrOverflow) {
		t.Fatalf("total overflow was hidden or discarded valid rows: %+v %+v", out.Worth, out.Spending)
	}
	for _, member := range out.Members {
		if member.Worth.Err != nil || member.Spent.Err != nil || member.WorthPercent.Err != nil {
			t.Fatalf("valid individual row became unavailable: %+v", member)
		}
	}
	if out.Worth.Owner("a").Value.Amount != math.MaxInt64 || out.Spending.Member("a").Value.Amount != math.MaxInt64 || out.Income.Total.Err != nil || out.Holder.Err != nil || !errors.Is(out.Spender.Err, money.ErrOverflow) {
		t.Fatal("separate total/facet availability was not retained")
	}
	in.Members[0], in.Members[1] = in.Members[1], in.Members[0]
	other := Prepare(in)
	if !errors.Is(other.Worth.Total.Err, money.ErrOverflow) || !errors.Is(other.Spending.Total.Err, money.ErrOverflow) {
		t.Fatal("checked totals depended on roster/map iteration order")
	}
}

func TestPrepareExtremeMagnitudesAndDistinctBarDenominators(t *testing.T) {
	in := fixture()
	in.Transactions = nil
	in.Members = []domain.Member{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	in.Accounts = []domain.Account{{ID: "a", OwnerID: "a", Currency: "USD", OpeningBalance: money.New(math.MinInt64, "USD")}, {ID: "b", OwnerID: "b", Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")}}
	out := Prepare(in)
	if out.Worth.Total.Err != nil || out.Worth.Total.Value.Amount != -1 || out.Members[0].WorthPercent.Value != 100 || out.Members[1].WorthPercent.Value != 99 || out.Holder.MemberID != "a" {
		t.Fatalf("MinInt64 magnitude/product wrapped: %+v", out)
	}
	in.Accounts[0].OpeningBalance = money.New(500, "USD")
	in.Accounts[1].OpeningBalance = money.New(-250, "USD")
	in.Accounts = append(in.Accounts, domain.Account{ID: "shared", OwnerID: domain.GroupOwnerID, Currency: "USD", OpeningBalance: money.New(-1000, "USD")})
	out = Prepare(in)
	if out.Members[0].WorthPercent.Value != 100 || out.Members[1].WorthPercent.Value != 50 || out.WorthRows[0].Percent.Value != 50 || out.WorthRows[1].Percent.Value != 25 || out.WorthRows[2].Percent.Value != 100 || out.Holder.Kind != LeaderShared {
		t.Fatal("roster/analytics denominator policies were merged or signed shared magnitude failed")
	}
	// The visible member-magnitude sum exceeds int64; the shared comparison
	// must remain exact and keep first-roster-member tie behavior.
	in.Accounts[0].OpeningBalance = money.New(math.MaxInt64, "USD")
	in.Accounts[1].OpeningBalance = money.New(math.MaxInt64, "USD")
	in.Accounts[2].OpeningBalance = money.New(math.MinInt64, "USD")
	out = Prepare(in)
	if out.Holder.Err != nil || out.Holder.Kind != LeaderMember || out.Holder.MemberID != "a" {
		t.Fatalf("large aggregate magnitude changed leader: %+v", out.Holder)
	}
	for _, row := range out.WorthRows {
		if row.Percent.Err != nil || row.Percent.Value < 0 || row.Percent.Value > 100 {
			t.Fatalf("invalid extreme share bar: %+v", row)
		}
	}
}

func TestPrepareChangesPeriodRatesAndRosterWithoutStaleFacts(t *testing.T) {
	in := fixture()
	first := Prepare(in)
	in.PeriodStart = in.PeriodEnd
	in.PeriodEnd = in.PeriodEnd.AddDate(0, 1, 0)
	second := Prepare(in)
	if !reflect.DeepEqual(first.Worth.ByOwner, second.Worth.ByOwner) || second.Spending.Total.Value.Amount != 999 || second.Income.Total.Value.Amount != 0 || second.Income.Total.Err != nil {
		t.Fatal("period boundaries contaminated all-ledger worth or failed to refresh period facts")
	}
	in = fixture()
	in.Accounts = append(in.Accounts, domain.Account{ID: "foreign", OwnerID: "alice", Currency: "EUR", OpeningBalance: money.New(100, "EUR")})
	missing := Prepare(in)
	if !errors.Is(missing.Worth.Err, currency.ErrUnknownRate) {
		t.Fatal("missing rate was hidden")
	}
	in.Rates.Rates = map[string]float64{"EUR": 1.5}
	in.Members[0].Name = "Renamed Bob"
	in.Members[0], in.Members[1] = in.Members[1], in.Members[0]
	restored := Prepare(in)
	if restored.Worth.Err != nil || restored.Worth.Total.Value.Amount != first.Worth.Total.Value.Amount+150 || restored.Members[0].ID != "alice" || restored.Members[1].Name != "Renamed Bob" {
		t.Fatal("restored FX/roster identity did not refresh")
	}
	if first.Members[0].ID != "bob" || first.Members[0].Name != "Bob" {
		t.Fatal("previous prepared roster aliased changed input")
	}
}

func TestPrepareUnrepresentedSpendingIsNotNobodySpent(t *testing.T) {
	in := fixture()
	in.Accounts = nil
	in.Transactions = []domain.Transaction{{ID: "ghost", Date: in.PeriodStart, MemberID: "ghost", Amount: money.New(-100, "USD")}}
	out := Prepare(in)
	if out.Spending.Total.Value.Amount != 100 || out.Spender.Err != nil || out.Spender.Kind != LeaderUnrepresented {
		t.Fatalf("positive orphan spending became no spending: %+v", out.Spender)
	}
	in.Transactions = append(in.Transactions, domain.Transaction{ID: "shared", Date: in.PeriodStart, Amount: money.New(-100, "USD")})
	if out = Prepare(in); out.Spender.Kind != LeaderShared {
		t.Fatalf("shared equal-majority policy changed: %+v", out.Spender)
	}
}

func TestPrepareDoesNotMutateOrAliasCallerData(t *testing.T) {
	in := fixture()
	in.Rates.Rates = map[string]float64{"EUR": 1.5}
	in.Members[0].Custom = map[string]any{"label": "keep"}
	before, _ := json.Marshal(in)
	out := Prepare(in)
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("preparation mutated input slices, maps or ownership")
	}
	worth := out.Worth.Owner("alice").Value
	in.Accounts[1].OwnershipShares["alice"] = 1
	in.Members[0].Name = "Changed"
	in.Transactions[1].MemberIDs[0] = "ghost"
	in.Rates.Rates["EUR"] = 9
	if out.Worth.Owner("alice").Value != worth || out.Members[0].Name != "Bob" || out.Spending.Member("bob").Value.Amount != 501 {
		t.Fatal("prepared values aliased caller-owned data")
	}
	out.Worth.ByOwner["alice"] = money.Zero("USD")
	out.Members[0].Name = "Result only"
	if in.Accounts[0].OpeningBalance.Amount != 10000 || in.Members[0].Name != "Changed" {
		t.Fatal("mutating result changed input")
	}
}
