// SPDX-License-Identifier: MIT

package reports

import (
	"reflect"
	"testing"
)

func flowNames(id string) string {
	switch id {
	case "sal":
		return "Salary"
	case "side":
		return "Side work"
	case "gro":
		return "Groceries"
	case "rent":
		return "Rent"
	case "":
		return "Uncategorized"
	}
	return id
}

func TestBuildMoneyFlowThreeColumns(t *testing.T) {
	d := BuildMoneyFlow(MoneyFlowInputs{
		Income:   []CategorySpend{{CategoryID: "sal", Amount: 5000}, {CategoryID: "side", Amount: 1000}},
		Spending: []CategorySpend{{CategoryID: "rent", Amount: 2000}, {CategoryID: "gro", Amount: 500}},
		Net:      3500,
		Name:     flowNames,
	})
	if d.IncomeTotal != 6000 || d.SpendingTotal != 2500 || d.Net != 3500 {
		t.Fatalf("totals = %d/%d/%d, want 6000/2500/3500", d.IncomeTotal, d.SpendingTotal, d.Net)
	}
	hub, ok := d.Node("Income")
	if !ok || hub.Kind != FlowIncomeHub || hub.Value != 6000 {
		t.Fatalf("hub = %+v, ok=%v; want value 6000", hub, ok)
	}
	if e, ok := d.Edge("Salary", "Income"); !ok || e.Value != 5000 || e.Kind != FlowIncomeSource {
		t.Fatalf("salary edge = %+v, ok=%v", e, ok)
	}
	if e, ok := d.Edge("Income", "Rent"); !ok || e.Value != 2000 || !reflect.DeepEqual(e.CategoryIDs, []string{"rent"}) {
		t.Fatalf("rent edge = %+v, ok=%v", e, ok)
	}
	if e, ok := d.Edge("Income", "Savings"); !ok || e.Value != 3500 || e.Kind != FlowSavings {
		t.Fatalf("savings edge = %+v, ok=%v", e, ok)
	}
	if len(d.Flows()) != len(d.Edges) {
		t.Fatalf("Flows() dropped edges: %d vs %d", len(d.Flows()), len(d.Edges))
	}
}

func TestBuildMoneyFlowPoolsTheTail(t *testing.T) {
	var spending []CategorySpend
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		spending = append(spending, CategorySpend{CategoryID: id, Amount: 100})
	}
	d := BuildMoneyFlow(MoneyFlowInputs{
		Income:      []CategorySpend{{CategoryID: "sal", Amount: 5000}},
		Spending:    spending,
		Net:         3800,
		Name:        flowNames,
		TopSpending: 10,
	})
	pooled, ok := d.Node("Everything else")
	if !ok {
		t.Fatal("no pooled spending node")
	}
	if pooled.Value != 200 {
		t.Fatalf("pooled value = %d, want 200", pooled.Value)
	}
	if !reflect.DeepEqual(pooled.CategoryIDs, []string{"k", "l"}) {
		t.Fatalf("pooled ids = %v, want [k l]", pooled.CategoryIDs)
	}
	if !pooled.Kind.Pooled() {
		t.Fatal("pooled node should report Pooled()")
	}
	// Every category still reaches the diagram, so the ribbons reconcile.
	if d.SpendingTotal != 1200 {
		t.Fatalf("spending total = %d, want 1200", d.SpendingTotal)
	}
}

func TestBuildMoneyFlowPoolsIncomeTail(t *testing.T) {
	var income []CategorySpend
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		income = append(income, CategorySpend{CategoryID: id, Amount: 100})
	}
	d := BuildMoneyFlow(MoneyFlowInputs{Income: income, Net: 700, Name: flowNames, TopSources: 5})
	pooled, ok := d.Node("Other income")
	if !ok || pooled.Value != 200 || !reflect.DeepEqual(pooled.CategoryIDs, []string{"f", "g"}) {
		t.Fatalf("pooled income = %+v, ok=%v", pooled, ok)
	}
	if e, ok := d.Edge("Other income", "Income"); !ok || e.Kind != FlowOtherIncome {
		t.Fatalf("pooled income edge = %+v, ok=%v", e, ok)
	}
}

func TestBuildMoneyFlowOverspendNamesTheGap(t *testing.T) {
	d := BuildMoneyFlow(MoneyFlowInputs{
		Income:   []CategorySpend{{CategoryID: "sal", Amount: 1000}},
		Spending: []CategorySpend{{CategoryID: "rent", Amount: 1500}},
		Net:      -500,
		Name:     flowNames,
	})
	e, ok := d.Edge("Drawn from savings", "Income")
	if !ok || e.Value != 500 || e.Kind != FlowFromSavings {
		t.Fatalf("deficit edge = %+v, ok=%v", e, ok)
	}
	// The gap ribbon is appended last so the layout stacks it under the real
	// sources; and with it, what reaches the hub equals what leaves.
	if last := d.Edges[len(d.Edges)-1]; last.Kind != FlowFromSavings {
		t.Fatalf("deficit edge is not last: %+v", last)
	}
	hub, _ := d.Node("Income")
	if hub.Value != 1500 {
		t.Fatalf("hub = %d, want 1500 (inflow matches outflow)", hub.Value)
	}
	if _, ok := d.Node("Savings"); ok {
		t.Fatal("an overspending period should not also show savings")
	}
}

func TestBuildMoneyFlowDisambiguatesACategoryNamedLikeTheHub(t *testing.T) {
	// A salary filed under a category literally called "Income" used to form a
	// From==To self-loop the layout drops, so the money vanished from the
	// picture while the headline still counted it.
	d := BuildMoneyFlow(MoneyFlowInputs{
		Income: []CategorySpend{{CategoryID: "inc", Amount: 4000}},
		Net:    4000,
		Name:   func(string) string { return "Income" },
	})
	if _, ok := d.Edge("Income (category)", "Income"); !ok {
		t.Fatalf("collision not disambiguated: %+v", d.Edges)
	}
	for _, e := range d.Edges {
		if e.From == e.To {
			t.Fatalf("self-loop survived: %+v", e)
		}
	}
}

func TestBuildMoneyFlowSkipsEmptyAndSignFlips(t *testing.T) {
	d := BuildMoneyFlow(MoneyFlowInputs{
		Income:   []CategorySpend{{CategoryID: "sal", Amount: 0}, {CategoryID: "side", Amount: -5}},
		Spending: []CategorySpend{{CategoryID: "gro", Amount: -300}, {CategoryID: "rent", Amount: 0}},
		Net:      -300,
		Name:     flowNames,
	})
	// A signed spending row still draws with its magnitude, never inverted.
	if e, ok := d.Edge("Income", "Groceries"); !ok || e.Value != 300 {
		t.Fatalf("groceries edge = %+v, ok=%v", e, ok)
	}
	if _, ok := d.Node("Salary"); ok {
		t.Fatal("a zero row should not get a node")
	}
	if _, ok := d.Node("Side work"); ok {
		t.Fatal("a negative income row should not get a node")
	}
}

func TestBuildMoneyFlowEmpty(t *testing.T) {
	d := BuildMoneyFlow(MoneyFlowInputs{Name: flowNames})
	if len(d.Nodes) != 0 || len(d.Edges) != 0 {
		t.Fatalf("empty input yielded %d nodes / %d edges", len(d.Nodes), len(d.Edges))
	}
}

func TestBuildMoneyFlowLaysOut(t *testing.T) {
	// The diagram feeds LayoutSankey unchanged: three columns, no dropped links.
	d := BuildMoneyFlow(MoneyFlowInputs{
		Income:   []CategorySpend{{CategoryID: "sal", Amount: 5000}},
		Spending: []CategorySpend{{CategoryID: "rent", Amount: 2000}},
		Net:      3000,
		Name:     flowNames,
	})
	l := LayoutSankey(d.Flows(), 1000, 400, 14, 12, 4)
	if len(l.Links) != len(d.Edges) {
		t.Fatalf("layout kept %d of %d links", len(l.Links), len(d.Edges))
	}
	cols := map[int]int{}
	for _, n := range l.Nodes {
		cols[n.Col]++
	}
	if cols[0] != 1 || cols[1] != 1 || cols[2] != 2 {
		t.Fatalf("columns = %v, want 1 source / 1 hub / 2 sinks", cols)
	}
}

func TestFlowCategorySetDedupesAndKeepsUncategorized(t *testing.T) {
	got := FlowCategorySet([]string{"b", "a"}, []string{"a", ""})
	if !reflect.DeepEqual(got, []string{"", "a", "b"}) {
		t.Fatalf("got %#v", got)
	}
}
