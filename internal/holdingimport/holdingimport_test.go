// SPDX-License-Identifier: MIT

package holdingimport

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

var fidelity = [][]string{
	{"Symbol", "Description", "Quantity", "Last Price", "Cost Basis Total"},
	{"AAPL", "APPLE INC", "12", "$189.50", "$1,800.00"},
	{"VTI", "VANGUARD TOTAL STOCK MKT ETF", "40.5", "$268.12", "$9,400.00"},
}

func TestGuessProfileReadsACommonExport(t *testing.T) {
	p := GuessProfile(fidelity[0], 2)
	want := []Field{FieldTicker, FieldName, FieldShares, FieldPrice, FieldCostBasis}
	for i, w := range want {
		if p.Field(i) != w {
			t.Errorf("column %d (%q) = %q, want %q", i, fidelity[0][i], p.Field(i), w)
		}
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// An unrecognised header must map to nothing. A column silently read as "price"
// when it was "day change" would corrupt every position in the file.
func TestGuessProfileDoesNotGuessAtUnknownColumns(t *testing.T) {
	p := GuessProfile([]string{"Symbol", "Day Change %", "Quantity"}, 2)
	if p.Field(1) != FieldNone {
		t.Errorf("%q mapped to %q, want nothing", "Day Change %", p.Field(1))
	}
}

func TestValidateRequiresAnIdentityAndAQuantity(t *testing.T) {
	if err := (Profile{Columns: []Field{FieldShares}}).Validate(); err != ErrNoIdentity {
		t.Errorf("a profile with no ticker or name was accepted")
	}
	if err := (Profile{Columns: []Field{FieldTicker}}).Validate(); err != ErrNoShares {
		t.Errorf("a profile with no shares column was accepted")
	}
	// Name alone is a legitimate identity — plenty of funds have no ticker.
	if err := (Profile{Columns: []Field{FieldName, FieldShares}}).Validate(); err != nil {
		t.Errorf("name+shares was rejected: %v", err)
	}
}

func TestParseHandlesExportDecoration(t *testing.T) {
	rows := Parse(GuessProfile(fidelity[0], 2), fidelity)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (the header must be skipped)", len(rows))
	}
	a := rows[0]
	if a.Ticker != "AAPL" || a.Name != "APPLE INC" {
		t.Errorf("row = %+v", a)
	}
	if a.Shares != 12 {
		t.Errorf("shares = %v", a.Shares)
	}
	// "$189.50" and "$1,800.00" — symbol and thousands separator both stripped.
	if a.PriceMinor != 18950 || !a.HasPrice {
		t.Errorf("price = %d,%v want 18950,true", a.PriceMinor, a.HasPrice)
	}
	if a.CostMinor != 180000 || !a.HasCost {
		t.Errorf("cost = %d,%v want 180000,true", a.CostMinor, a.HasCost)
	}
	if rows[1].Shares != 40.5 {
		t.Errorf("fractional shares = %v", rows[1].Shares)
	}
}

// A preview that silently drops unreadable rows is how someone commits an
// import believing it covered everything.
func TestParseKeepsBadRowsWithTheirReason(t *testing.T) {
	p := GuessProfile([]string{"Symbol", "Quantity", "Last Price"}, 2)
	rows := Parse(p, [][]string{
		{"Symbol", "Quantity", "Last Price"},
		{"AAPL", "twelve", "100"},
		{"", "5", "100"},
		{"VTI", "", "100"},
		{"BAD", "-3", "100"},
		{"OK", "1", "100"},
	})
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want all 5 kept", len(rows))
	}
	wantErr := []bool{true, true, true, true, false}
	for i, w := range wantErr {
		if (rows[i].Err != "") != w {
			t.Errorf("row %d (line %d) err=%q, want error=%v", i, rows[i].Line, rows[i].Err, w)
		}
	}
	if rows[0].Line != 1 || rows[4].Line != 5 {
		t.Errorf("line numbers = %d..%d, want 1..5", rows[0].Line, rows[4].Line)
	}
}

func TestParseAccountingNegatives(t *testing.T) {
	p := Profile{Columns: []Field{FieldTicker, FieldShares, FieldCostBasis}, Decimals: 2}
	rows := Parse(p, [][]string{{"X", "1", "(250.00)"}})
	if rows[0].CostMinor != -25000 {
		t.Errorf("cost = %d, want -25000", rows[0].CostMinor)
	}
}

// ─── planning ────────────────────────────────────────────────────────────────

func held(id, acct, ticker, name string, shares float64, cost, price int64) domain.Holding {
	return domain.Holding{ID: id, AccountID: acct, Ticker: ticker, Name: name,
		Shares: shares, CostBasisMinor: cost, CurrentPriceMinorPerShare: price}
}

func ids() func() string {
	n := 0
	return func() string { n++; return "new-" + string(rune('a'+n-1)) }
}

// Importing Monday's export and then Friday's should leave you holding what
// Friday said, not both.
func TestPlanUpdatesAMatchInsteadOfAdding(t *testing.T) {
	existing := []domain.Holding{held("h1", "acct", "AAPL", "Apple", 10, 150000, 18000)}
	rows := Parse(GuessProfile(fidelity[0], 2), fidelity)

	plan := Plan("acct", existing, rows, ids())
	if len(plan) != 2 {
		t.Fatalf("got %d changes", len(plan))
	}
	if plan[0].Action != ActionUpdate || plan[0].Before.ID != "h1" {
		t.Errorf("AAPL = %+v, want an update to h1", plan[0].Action)
	}
	if plan[0].After.ID != "h1" {
		t.Errorf("the update lost the holding's id: %q", plan[0].After.ID)
	}
	if plan[0].After.Shares != 12 {
		t.Errorf("shares after = %v, want 12", plan[0].After.Shares)
	}
	if plan[1].Action != ActionAdd || plan[1].After.Ticker != "VTI" {
		t.Errorf("VTI = %+v, want an add", plan[1])
	}
	if plan[1].After.ID == "" {
		t.Error("an added holding got no id")
	}

	s := Summarize(plan)
	if s.Add != 1 || s.Update != 1 || s.Writes() != 2 || s.Total() != 2 {
		t.Errorf("Summarize = %+v", s)
	}
}

// Importing into the wrong account must not silently rewrite the right one.
func TestPlanIgnoresOtherAccountsHoldings(t *testing.T) {
	existing := []domain.Holding{held("h1", "other", "AAPL", "Apple", 10, 0, 0)}
	rows := Parse(Profile{Columns: []Field{FieldTicker, FieldShares}, Decimals: 2},
		[][]string{{"AAPL", "12"}})
	plan := Plan("acct", existing, rows, ids())
	if plan[0].Action != ActionAdd {
		t.Errorf("action = %q, want add — another account's holding was matched", plan[0].Action)
	}
}

// A blank cell means "leave it alone". Brokerage exports routinely omit cost
// basis, and treating that as $0 would report every position as pure gain.
func TestBlankCellsDoNotZeroExistingValues(t *testing.T) {
	existing := []domain.Holding{held("h1", "acct", "AAPL", "Apple", 10, 150000, 18000)}
	p := Profile{Columns: []Field{FieldTicker, FieldShares, FieldCostBasis}, Decimals: 2}
	plan := Plan("acct", existing, Parse(p, [][]string{{"AAPL", "12", ""}}), ids())

	if plan[0].Action != ActionUpdate {
		t.Fatalf("action = %q", plan[0].Action)
	}
	if plan[0].After.CostBasisMinor != 150000 {
		t.Errorf("cost basis = %d, want the existing 150000 kept", plan[0].After.CostBasisMinor)
	}
	if plan[0].After.CurrentPriceMinorPerShare != 18000 {
		t.Errorf("price = %d, want the existing 18000 kept", plan[0].After.CurrentPriceMinorPerShare)
	}
}

// Re-importing the identical file must plan nothing, so "12 rows imported" never
// means "12 rows rewritten to what they already said".
func TestPlanSkipsRowsThatWouldChangeNothing(t *testing.T) {
	existing := []domain.Holding{held("h1", "acct", "AAPL", "APPLE INC", 12, 180000, 18950)}
	p := Profile{Columns: []Field{FieldTicker, FieldName, FieldShares, FieldPrice, FieldCostBasis},
		HasHeader: true, Decimals: 2}
	plan := Plan("acct", existing, Parse(p, fidelity[:2]), ids())

	if plan[0].Action != ActionSkip {
		t.Errorf("action = %q, want skip: %+v", plan[0].Action, plan[0].After)
	}
	if plan[0].Reason == "" {
		t.Error("a skip with no reason tells the reader nothing")
	}
	if s := Summarize(plan); s.Writes() != 0 {
		t.Errorf("Writes = %d, want 0", s.Writes())
	}
}

// A file listing the same position twice must not produce two holdings.
func TestPlanFoldsDuplicateRowsWithinOneFile(t *testing.T) {
	p := Profile{Columns: []Field{FieldTicker, FieldShares}, Decimals: 2}
	plan := Plan("acct", nil, Parse(p, [][]string{{"AAPL", "5"}, {"AAPL", "7"}}), ids())
	if plan[0].Action != ActionAdd || plan[1].Action != ActionUpdate {
		t.Errorf("actions = %q,%q want add,update", plan[0].Action, plan[1].Action)
	}
	if plan[1].After.Shares != 7 {
		t.Errorf("the second row left %v shares, want the later value", plan[1].After.Shares)
	}
}

// A position with no ticker matches by name — plenty of funds have none.
func TestPlanMatchesByNameWhenThereIsNoTicker(t *testing.T) {
	existing := []domain.Holding{held("h1", "acct", "", "Company 401k Fund", 100, 0, 0)}
	p := Profile{Columns: []Field{FieldName, FieldShares}, Decimals: 2}
	plan := Plan("acct", existing, Parse(p, [][]string{{"company 401k fund", "120"}}), ids())
	if plan[0].Action != ActionUpdate || plan[0].Before.ID != "h1" {
		t.Errorf("action = %q — a name-only position was not matched", plan[0].Action)
	}
}

// An unusable row is planned as a skip carrying its reason, never silently
// dropped from the preview.
func TestPlanKeepsUnusableRowsAsExplainedSkips(t *testing.T) {
	p := Profile{Columns: []Field{FieldTicker, FieldShares}, Decimals: 2}
	plan := Plan("acct", nil, Parse(p, [][]string{{"AAPL", "nope"}}), ids())
	if len(plan) != 1 || plan[0].Action != ActionSkip {
		t.Fatalf("plan = %+v", plan)
	}
	if plan[0].Reason == "" {
		t.Error("the skip carries no reason")
	}
}

// A ticker with no name would render as a blank row in the holdings list.
func TestAddedHoldingFallsBackToItsTickerForAName(t *testing.T) {
	p := Profile{Columns: []Field{FieldTicker, FieldShares}, Decimals: 2}
	plan := Plan("acct", nil, Parse(p, [][]string{{"AAPL", "3"}}), ids())
	if plan[0].After.Name != "AAPL" {
		t.Errorf("name = %q, want the ticker as a fallback", plan[0].After.Name)
	}
}
