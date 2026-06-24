package cardgraph

import (
	"encoding/json"
	"testing"
)

func TestCanFeed(t *testing.T) {
	tests := []struct {
		out, in PortType
		want    bool
	}{
		{TypeNumber, TypeNumber, true},
		{TypeBool, TypeNumber, true},  // bool→number coercion
		{TypeNumber, TypeText, true},  // number→text coercion
		{TypeBool, TypeText, true},    // bool→text coercion
		{TypeNumber, TypeBool, false}, // no number→bool
		{TypeText, TypeNumber, false}, // no text→number
		{TypeViz, TypeNumber, false},  // viz feeds nothing scalar
		{TypeNumber, TypeViz, false},  // nothing feeds a viz input scalarly
	}
	for _, tt := range tests {
		if got := CanFeed(tt.out, tt.in); got != tt.want {
			t.Errorf("CanFeed(%s,%s)=%v want %v", tt.out, tt.in, got, tt.want)
		}
	}
}

// kpiFromLiteral wires literal.number("42") → viz.kpi and returns the graph.
func kpiFromLiteral(value, format string) Graph {
	return Graph{
		Nodes: []Node{
			{ID: "lit", Kind: KindLiteralNumber, Props: map[string]string{"value": value}},
			{ID: "kpi", Kind: KindVizKPI, Props: map[string]string{"title": "Net worth", "format": format}},
		},
		Edges: []Edge{{From: PortRef{"lit", OutPort}, To: PortRef{"kpi", "value"}}},
		Root:  "kpi",
	}
}

func TestEvalLiteralKPI(t *testing.T) {
	res := Eval(kpiFromLiteral("42", "number"), Context{})
	if len(res.Issues) != 0 {
		t.Fatalf("unexpected issues: %+v", res.Issues)
	}
	if res.Render == nil {
		t.Fatal("no render")
	}
	if res.Render.Kind != "kpi" || res.Render.Title != "Net worth" || res.Render.Text != "42" {
		t.Errorf("render = %+v", *res.Render)
	}
}

func TestEvalSourceFormulaKPI(t *testing.T) {
	// source.scalar(net_worth=1000) → formula("a / 2") → viz.kpi(auto tone)
	g := Graph{
		Nodes: []Node{
			{ID: "src", Kind: KindSourceScalar, Props: map[string]string{"name": "net_worth"}},
			{ID: "f", Kind: KindFormula, Props: map[string]string{"expr": "a / 2"}},
			{ID: "kpi", Kind: KindVizKPI, Props: map[string]string{"title": "Half", "tone": "auto"}},
		},
		Edges: []Edge{
			{From: PortRef{"src", OutPort}, To: PortRef{"f", "a"}},
			{From: PortRef{"f", OutPort}, To: PortRef{"kpi", "value"}},
		},
		Root: "kpi",
	}
	res := Eval(g, Context{Vars: map[string]float64{"net_worth": 1000}})
	if len(res.Issues) != 0 {
		t.Fatalf("unexpected issues: %+v", res.Issues)
	}
	if res.Render.Text != "500" {
		t.Errorf("text = %q want 500", res.Render.Text)
	}
	if res.Render.Tone != "up" {
		t.Errorf("tone = %q want up", res.Render.Tone)
	}
}

func TestEvalPercentFormatAndCoercion(t *testing.T) {
	g := kpiFromLiteral("12.5", "percent")
	res := Eval(g, Context{})
	if res.Render.Text != "12.5%" {
		t.Errorf("text = %q want 12.5%%", res.Render.Text)
	}
}

func TestEvalDegradesOnBrokenNode(t *testing.T) {
	// A bad literal makes its node fatal; the KPI downstream then can't resolve, but
	// Eval returns issues instead of panicking.
	g := kpiFromLiteral("not-a-number", "number")
	res := Eval(g, Context{})
	if res.Render != nil {
		t.Fatal("expected no render for a broken graph")
	}
	if len(res.Issues) == 0 {
		t.Fatal("expected issues")
	}
	foundLit := false
	for _, is := range res.Issues {
		if is.Node == "lit" {
			foundLit = true
		}
	}
	if !foundLit {
		t.Errorf("expected an issue on the literal node, got %+v", res.Issues)
	}
}

func TestLogicCompareBranchKPI(t *testing.T) {
	// "Is net worth positive?" → show 1 if net_worth > 0 else 0.
	//   src(net_worth) ─┐
	//   lit0(0) ────────┴► compare(>) ─► branch.cond
	//   lit1(1) ──────────────────────► branch.whenTrue
	//   lit0b(0) ─────────────────────► branch.whenFalse ─► kpi
	g := Graph{
		Nodes: []Node{
			{ID: "src", Kind: KindSourceScalar, Props: map[string]string{"name": "net_worth"}},
			{ID: "zero", Kind: KindLiteralNumber, Props: map[string]string{"value": "0"}},
			{ID: "cmp", Kind: KindCompare, Props: map[string]string{"op": ">"}},
			{ID: "one", Kind: KindLiteralNumber, Props: map[string]string{"value": "1"}},
			{ID: "zero2", Kind: KindLiteralNumber, Props: map[string]string{"value": "0"}},
			{ID: "br", Kind: KindBranchNumber},
			{ID: "kpi", Kind: KindVizKPI, Props: map[string]string{"title": "Positive?"}},
		},
		Edges: []Edge{
			{From: PortRef{"src", OutPort}, To: PortRef{"cmp", "a"}},
			{From: PortRef{"zero", OutPort}, To: PortRef{"cmp", "b"}},
			{From: PortRef{"cmp", OutPort}, To: PortRef{"br", "cond"}},
			{From: PortRef{"one", OutPort}, To: PortRef{"br", "whenTrue"}},
			{From: PortRef{"zero2", OutPort}, To: PortRef{"br", "whenFalse"}},
			{From: PortRef{"br", OutPort}, To: PortRef{"kpi", "value"}},
		},
		Root: "kpi",
	}
	if issues := Validate(g); len(issues) != 0 {
		t.Fatalf("validate: %+v", issues)
	}
	pos := Eval(g, Context{Vars: map[string]float64{"net_worth": 1000}})
	if pos.Render == nil || pos.Render.Text != "1" {
		t.Errorf("positive case = %+v", pos)
	}
	neg := Eval(g, Context{Vars: map[string]float64{"net_worth": -50}})
	if neg.Render == nil || neg.Render.Text != "0" {
		t.Errorf("negative case = %+v", neg)
	}
}

func TestVizText(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "t", Kind: KindLiteralText, Props: map[string]string{"value": "Hello"}},
			{ID: "v", Kind: KindVizText, Props: map[string]string{"title": "Greeting"}},
		},
		Edges: []Edge{{From: PortRef{"t", OutPort}, To: PortRef{"v", "value"}}},
		Root:  "v",
	}
	res := Eval(g, Context{})
	if len(res.Issues) != 0 || res.Render == nil {
		t.Fatalf("issues=%+v render=%v", res.Issues, res.Render)
	}
	if res.Render.Kind != "text" || res.Render.Text != "Hello" {
		t.Errorf("render = %+v", *res.Render)
	}
}

func TestVizProgress(t *testing.T) {
	// 750 of 1000 → 75% fill, not full (no "up" tone).
	g := Graph{
		Nodes: []Node{
			{ID: "v", Kind: KindLiteralNumber, Props: map[string]string{"value": "750"}},
			{ID: "m", Kind: KindLiteralNumber, Props: map[string]string{"value": "1000"}},
			{ID: "p", Kind: KindVizProgress, Props: map[string]string{"title": "Goal"}},
		},
		Edges: []Edge{
			{From: PortRef{"v", OutPort}, To: PortRef{"p", "value"}},
			{From: PortRef{"m", OutPort}, To: PortRef{"p", "max"}},
		},
		Root: "p",
	}
	res := Eval(g, Context{})
	if res.Render == nil {
		t.Fatalf("no render: %+v", res.Issues)
	}
	if res.Render.Kind != "progress" || res.Render.Pct != 0.75 {
		t.Errorf("render = %+v", *res.Render)
	}
	if res.Render.Sub != "of 1000" {
		t.Errorf("sub = %q", res.Render.Sub)
	}
	if res.Render.Tone != "" {
		t.Errorf("expected no tone below max, got %q", res.Render.Tone)
	}
}

func TestVizProgressZeroMaxNoPanic(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "v", Kind: KindLiteralNumber, Props: map[string]string{"value": "5"}},
			{ID: "p", Kind: KindVizProgress},
		},
		Edges: []Edge{{From: PortRef{"v", OutPort}, To: PortRef{"p", "value"}}},
		Root:  "p",
	}
	res := Eval(g, Context{})
	if res.Render == nil || res.Render.Pct != 0 {
		t.Errorf("zero/missing max should give 0%% fill, got %+v", res.Render)
	}
}

// sampleTxns is a small transactions collection for data-node tests.
func sampleTxns() Collection {
	return Collection{
		Cols: []Column{{Name: "category", Type: TypeText}, {Name: "amount", Type: TypeNumber}},
		Rows: []Row{
			{"category": Text("Food"), "amount": Num(20)},
			{"category": Text("Food"), "amount": Num(30)},
			{"category": Text("Rent"), "amount": Num(900)},
			{"category": Text("Fun"), "amount": Num(50)},
		},
	}
}

func TestDatasetGroupByChart(t *testing.T) {
	// dataset(txns) → groupby category sum(amount) → chart. Rent(900) should lead.
	g := Graph{
		Nodes: []Node{
			{ID: "ds", Kind: KindSourceDataset, Props: map[string]string{"which": "txns"}},
			{ID: "gb", Kind: KindGroupBy, Props: map[string]string{"group": "category", "value": "amount", "fn": "sum"}},
			{ID: "ch", Kind: KindVizChart, Props: map[string]string{"chart": "bar", "title": "By category"}},
		},
		Edges: []Edge{
			{From: PortRef{"ds", OutPort}, To: PortRef{"gb", "in"}},
			{From: PortRef{"gb", OutPort}, To: PortRef{"ch", "series"}},
		},
		Root: "ch",
	}
	res := Eval(g, Context{Datasets: map[string]Collection{"txns": sampleTxns()}})
	if res.Render == nil {
		t.Fatalf("no render: %+v", res.Issues)
	}
	if res.Render.Kind != "chart" || res.Render.Chart != "bar" {
		t.Errorf("render = %+v", *res.Render)
	}
	if len(res.Render.Series) != 3 {
		t.Fatalf("want 3 groups, got %d: %+v", len(res.Render.Series), res.Render.Series)
	}
	if res.Render.Series[0].Label != "Rent" || res.Render.Series[0].Value != 900 {
		t.Errorf("top group = %+v, want Rent=900", res.Render.Series[0])
	}
	if res.Render.Series[1].Label != "Food" || res.Render.Series[1].Value != 50 {
		t.Errorf("second group = %+v, want Food=50", res.Render.Series[1])
	}
}

func TestGroupByChronologicalSort(t *testing.T) {
	// Group by month with sort=label → chronological order (not value-descending).
	c := Collection{
		Cols: []Column{{Name: "month", Type: TypeText}, {Name: "amount", Type: TypeNumber}},
		Rows: []Row{
			{"month": Text("2026-03"), "amount": Num(10)},
			{"month": Text("2026-01"), "amount": Num(99)},
			{"month": Text("2026-02"), "amount": Num(50)},
		},
	}
	g := Graph{
		Nodes: []Node{
			{ID: "ds", Kind: KindSourceDataset, Props: map[string]string{"which": "txns"}},
			{ID: "gb", Kind: KindGroupBy, Props: map[string]string{"group": "month", "value": "amount", "fn": "sum", "sort": "label"}},
			{ID: "ch", Kind: KindVizChart, Props: map[string]string{"chart": "line"}},
		},
		Edges: []Edge{
			{From: PortRef{"ds", OutPort}, To: PortRef{"gb", "in"}},
			{From: PortRef{"gb", OutPort}, To: PortRef{"ch", "series"}},
		},
		Root: "ch",
	}
	res := Eval(g, Context{Datasets: map[string]Collection{"txns": c}})
	if res.Render == nil || len(res.Render.Series) != 3 {
		t.Fatalf("series: %+v", res)
	}
	got := []string{res.Render.Series[0].Label, res.Render.Series[1].Label, res.Render.Series[2].Label}
	if got[0] != "2026-01" || got[1] != "2026-02" || got[2] != "2026-03" {
		t.Errorf("not chronological: %v", got)
	}
}

func TestFilterThenAggregate(t *testing.T) {
	// Keep Food rows, sum amount → 50.
	g := Graph{
		Nodes: []Node{
			{ID: "ds", Kind: KindSourceDataset, Props: map[string]string{"which": "txns"}},
			{ID: "f", Kind: KindFilter, Props: map[string]string{"col": "category", "op": "==", "value": "food"}},
			{ID: "a", Kind: KindAggregate, Props: map[string]string{"col": "amount", "fn": "sum"}},
			{ID: "k", Kind: KindVizKPI, Props: map[string]string{"title": "Food"}},
		},
		Edges: []Edge{
			{From: PortRef{"ds", OutPort}, To: PortRef{"f", "in"}},
			{From: PortRef{"f", OutPort}, To: PortRef{"a", "in"}},
			{From: PortRef{"a", OutPort}, To: PortRef{"k", "value"}},
		},
		Root: "k",
	}
	res := Eval(g, Context{Datasets: map[string]Collection{"txns": sampleTxns()}})
	if res.Render == nil || res.Render.Text != "50" {
		t.Errorf("filter+agg = %+v (want 50)", res.Render)
	}
}

func TestVizListLimit(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "ds", Kind: KindSourceDataset, Props: map[string]string{"which": "txns"}},
			{ID: "l", Kind: KindVizList, Props: map[string]string{"title": "Recent", "limit": "2"}},
		},
		Edges: []Edge{{From: PortRef{"ds", OutPort}, To: PortRef{"l", "in"}}},
		Root:  "l",
	}
	res := Eval(g, Context{Datasets: map[string]Collection{"txns": sampleTxns()}})
	if res.Render == nil || res.Render.Kind != "list" {
		t.Fatalf("no list render: %+v", res)
	}
	if len(res.Render.Rows) != 2 {
		t.Errorf("limit 2 → %d rows", len(res.Render.Rows))
	}
	if len(res.Render.Cols) != 2 {
		t.Errorf("expected 2 columns, got %d", len(res.Render.Cols))
	}
}

func TestVizStatDelta(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "v", Kind: KindLiteralNumber, Props: map[string]string{"value": "120"}},
			{ID: "p", Kind: KindLiteralNumber, Props: map[string]string{"value": "100"}},
			{ID: "s", Kind: KindVizStat, Props: map[string]string{"title": "Net worth"}},
		},
		Edges: []Edge{
			{From: PortRef{"v", OutPort}, To: PortRef{"s", "value"}},
			{From: PortRef{"p", OutPort}, To: PortRef{"s", "prev"}},
		},
		Root: "s",
	}
	res := Eval(g, Context{})
	if res.Render == nil || res.Render.Kind != "stat" {
		t.Fatalf("no stat render: %+v", res)
	}
	if res.Render.Tone != "up" || res.Render.Text != "120" {
		t.Errorf("stat = %+v (want up, 120)", *res.Render)
	}
	if res.Render.Sub != "▲ 20%" {
		t.Errorf("delta = %q want ▲ 20%%", res.Render.Sub)
	}
}

func TestNamedVariableReference(t *testing.T) {
	// Two named source nodes (income, rent) and a formula that references them BY NAME
	// — no wire from the sources into the formula. Topo order must still place the
	// named nodes before the formula because... they're unwired. So we force order via
	// a wire only for the formula→kpi; the named nodes are roots evaluated first by id.
	// income=5000, rent=1500 → "income - rent" = 3500.
	g := Graph{
		Nodes: []Node{
			{ID: "a_income", Kind: KindSourceScalar, Var: "income", Props: map[string]string{"name": "income"}},
			{ID: "b_rent", Kind: KindLiteralNumber, Var: "rent", Props: map[string]string{"value": "1500"}},
			{ID: "c_f", Kind: KindFormula, Props: map[string]string{"expr": "income - rent"}},
			{ID: "d_kpi", Kind: KindVizKPI, Props: map[string]string{"title": "Left over"}},
		},
		Edges: []Edge{{From: PortRef{"c_f", OutPort}, To: PortRef{"d_kpi", "value"}}},
		Root:  "d_kpi",
	}
	res := Eval(g, Context{Vars: map[string]float64{"income": 5000}})
	if res.Render == nil {
		t.Fatalf("no render: %+v", res.Issues)
	}
	if res.Render.Text != "3500" {
		t.Errorf("named-var formula = %q want 3500", res.Render.Text)
	}
}

func TestDuplicateVariableNameRejected(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "x", Kind: KindLiteralNumber, Var: "v", Props: map[string]string{"value": "1"}},
			{ID: "y", Kind: KindLiteralNumber, Var: "v", Props: map[string]string{"value": "2"}},
			{ID: "k", Kind: KindVizKPI},
		},
		Edges: []Edge{{From: PortRef{"x", OutPort}, To: PortRef{"k", "value"}}},
		Root:  "k",
	}
	issues := Validate(g)
	if len(issues) == 0 {
		t.Error("expected a duplicate-variable issue")
	}
}

func TestInvalidVariableNameRejected(t *testing.T) {
	if ValidIdent("1bad") || ValidIdent("has space") || ValidIdent("") {
		t.Error("ValidIdent accepted an invalid name")
	}
	if !ValidIdent("net_worth2") || !ValidIdent("_x") {
		t.Error("ValidIdent rejected a valid name")
	}
}

func TestTopoOrderCycle(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			{ID: "a", Kind: KindFormula, Props: map[string]string{"expr": "b"}},
			{ID: "b", Kind: KindFormula, Props: map[string]string{"expr": "a"}},
		},
		Edges: []Edge{
			{From: PortRef{"a", OutPort}, To: PortRef{"b", "a"}},
			{From: PortRef{"b", OutPort}, To: PortRef{"a", "a"}},
		},
		Root: "b",
	}
	if _, err := TopoOrder(g); err == nil {
		t.Fatal("expected a cycle error")
	}
	res := Eval(g, Context{})
	if res.Render != nil || len(res.Issues) == 0 {
		t.Errorf("cycle should yield issues and no render, got %+v", res)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		g         Graph
		wantClean bool
	}{
		{"good", kpiFromLiteral("1", "number"), true},
		{"unknown kind", Graph{Nodes: []Node{{ID: "x", Kind: "nope"}}, Root: "x"}, false},
		{"no root", func() Graph { g := kpiFromLiteral("1", ""); g.Root = ""; return g }(), false},
		{"root not viz", Graph{Nodes: []Node{{ID: "lit", Kind: KindLiteralNumber, Props: map[string]string{"value": "1"}}}, Root: "lit"}, false},
		{"type mismatch", Graph{
			Nodes: []Node{
				{ID: "t", Kind: KindLiteralText, Props: map[string]string{"value": "hi"}},
				{ID: "f", Kind: KindFormula, Props: map[string]string{"expr": "a"}},
				{ID: "kpi", Kind: KindVizKPI},
			},
			// text → formula's number input "a" is not allowed.
			Edges: []Edge{
				{From: PortRef{"t", OutPort}, To: PortRef{"f", "a"}},
				{From: PortRef{"f", OutPort}, To: PortRef{"kpi", "value"}},
			},
			Root: "kpi",
		}, false},
		{"double-wired input", Graph{
			Nodes: []Node{
				{ID: "l1", Kind: KindLiteralNumber, Props: map[string]string{"value": "1"}},
				{ID: "l2", Kind: KindLiteralNumber, Props: map[string]string{"value": "2"}},
				{ID: "kpi", Kind: KindVizKPI},
			},
			Edges: []Edge{
				{From: PortRef{"l1", OutPort}, To: PortRef{"kpi", "value"}},
				{From: PortRef{"l2", OutPort}, To: PortRef{"kpi", "value"}},
			},
			Root: "kpi",
		}, false},
		{"edge to missing node", Graph{
			Nodes: []Node{{ID: "kpi", Kind: KindVizKPI}},
			Edges: []Edge{{From: PortRef{"ghost", OutPort}, To: PortRef{"kpi", "value"}}},
			Root:  "kpi",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := Validate(tt.g)
			if tt.wantClean && len(issues) != 0 {
				t.Errorf("expected clean, got %+v", issues)
			}
			if !tt.wantClean && len(issues) == 0 {
				t.Error("expected issues, got none")
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	g := kpiFromLiteral("99", "currency")
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var back Graph
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	res := Eval(back, Context{})
	if res.Render == nil || res.Render.Text != "99" {
		t.Errorf("round-trip eval = %+v", res)
	}
}

func TestParsePortRef(t *testing.T) {
	p := ParsePortRef("node1:value")
	if p.Node != "node1" || p.Port != "value" {
		t.Errorf("got %+v", p)
	}
	if p.String() != "node1:value" {
		t.Errorf("String() = %q", p.String())
	}
	bare := ParsePortRef("solo")
	if bare.Node != "solo" || bare.Port != "" {
		t.Errorf("bare = %+v", bare)
	}
}
