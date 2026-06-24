// SPDX-License-Identifier: MIT

package cardgraph

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/monstercameron/CashFlux/internal/formula"
)

// referencedVars returns the variable names a node reads by name (not by wire): the
// identifiers in a formula expression, or a scalar source's figure name. These create
// implicit dependencies on the upstream node that defines each name, so evaluation
// order is correct even without a wire.
func referencedVars(n Node) []string {
	switch n.Kind {
	case KindFormula:
		toks, err := formula.Tokenize(n.Props["expr"])
		if err != nil {
			return nil
		}
		var out []string
		for _, t := range toks {
			if t.Kind == formula.TIdent {
				out = append(out, t.Text)
			}
		}
		return out
	case KindSourceScalar:
		if v := strings.TrimSpace(n.Props["name"]); v != "" {
			return []string{v}
		}
	}
	return nil
}

// implicitDeps returns synthetic edges for variable references: an edge from the node
// that defines a variable (Var) to each node that references that name. Used only for
// ordering (not as real input wires).
func implicitDeps(g Graph) []Edge {
	owner := map[string]NodeID{}
	for _, n := range g.Nodes {
		if n.Var != "" {
			owner[n.Var] = n.ID
		}
	}
	if len(owner) == 0 {
		return nil
	}
	var deps []Edge
	for _, n := range g.Nodes {
		for _, ref := range referencedVars(n) {
			if from, ok := owner[ref]; ok && from != n.ID {
				deps = append(deps, Edge{From: PortRef{Node: from, Port: OutPort}, To: PortRef{Node: n.ID, Port: "__var"}})
			}
		}
	}
	return deps
}

// Issue is a problem found while validating or evaluating a graph, tied to a node (or
// empty for a graph-level issue). Severity lets the UI show error vs. needs-input
// chips; the card degrades around issues rather than crashing.
type Issue struct {
	Node    NodeID
	Message string
	Fatal   bool // true = the node could not produce a value
}

// TopoOrder returns the node ids in dependency order (every node appears after the
// nodes feeding it), using Kahn's algorithm. It errors if the graph contains a cycle —
// the language is a DAG, so a cycle is a build error, never an infinite loop.
func TopoOrder(g Graph) ([]NodeID, error) {
	indeg := map[NodeID]int{}
	for _, n := range g.Nodes {
		indeg[n.ID] = 0
	}
	// deps[x] = nodes that depend on x (x feeds them). Includes real wires plus the
	// implicit dependencies created by variable references (a node that reads "income"
	// must run after the node named "income").
	edges := append(append([]Edge(nil), g.Edges...), implicitDeps(g)...)
	deps := map[NodeID][]NodeID{}
	for _, e := range edges {
		from, to := e.From.Node, e.To.Node
		if _, ok := indeg[to]; !ok {
			continue // dangling edge; validation reports it separately
		}
		if _, ok := indeg[from]; !ok {
			continue
		}
		deps[from] = append(deps[from], to)
		indeg[to]++
	}

	// Seed the queue with zero-indegree nodes in stable id order for determinism.
	var queue []NodeID
	for _, n := range g.Nodes {
		if indeg[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	slices.Sort(queue)

	var order []NodeID
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		next := append([]NodeID(nil), deps[id]...)
		slices.Sort(next)
		for _, d := range next {
			indeg[d]--
			if indeg[d] == 0 {
				queue = append(queue, d)
			}
		}
	}
	if len(order) != len(g.Nodes) {
		return nil, fmt.Errorf("cardgraph: the graph has a cycle")
	}
	return order, nil
}

// Result is the outcome of evaluating a graph: the root's rendered Viz block (nil if
// the root could not produce one) and any issues collected along the way.
type Result struct {
	Render *VizBlock
	Issues []Issue
}

// Eval evaluates the graph against ctx and returns the rendered root plus issues.
// Evaluation is pure and deterministic: nodes run in topological order, each gathers
// its wired inputs, and a node that errors records a fatal issue and is skipped — its
// dependents see a missing input and degrade in turn, so a half-built card still
// previews what it can. A cycle is a single graph-level issue.
func Eval(g Graph, ctx Context) Result {
	var res Result
	order, err := TopoOrder(g)
	if err != nil {
		res.Issues = append(res.Issues, Issue{Message: err.Error(), Fatal: true})
		return res
	}

	// incoming[to] = the edges feeding node `to`, so we can resolve input ports.
	incoming := map[NodeID][]Edge{}
	for _, e := range g.Edges {
		incoming[e.To.Node] = append(incoming[e.To.Node], e)
	}

	// boundVars/boundStrs accumulate the named-node outputs (Node.Var) as evaluation
	// proceeds in topological order, so a node can reference any upstream node's value
	// by its variable name without a wire. They start from the base context and grow;
	// names shadow base figures of the same name.
	boundVars := map[string]float64{}
	maps.Copy(boundVars, ctx.Vars)
	boundStrs := map[string]string{}
	maps.Copy(boundStrs, ctx.Strs)

	values := map[NodeID]Value{} // successfully-computed node outputs
	for _, id := range order {
		n, ok := g.node(id)
		if !ok {
			continue
		}
		spec, ok := Lookup(n.Kind)
		if !ok {
			res.Issues = append(res.Issues, Issue{Node: id, Message: fmt.Sprintf("unknown node kind %q", n.Kind), Fatal: true})
			continue
		}
		// Gather resolved inputs (coercing to the input port's declared type). An input
		// whose source failed simply doesn't appear in the map.
		inputs := map[string]Value{}
		portType := map[string]PortType{}
		for _, p := range spec.Inputs {
			portType[p.Name] = p.Type
		}
		for _, e := range incoming[id] {
			src, ok := values[e.From.Node]
			if !ok {
				continue // upstream failed or unconnected
			}
			if want, ok := portType[e.To.Port]; ok {
				inputs[e.To.Port] = coerce(src, want)
			}
		}
		// Each node sees the base context plus all named upstream outputs (so formula/
		// source nodes can reference variables by name). Datasets pass through.
		effCtx := Context{Vars: boundVars, Strs: boundStrs, Datasets: ctx.Datasets}
		v, err := spec.Eval(inputs, n.Props, effCtx)
		if err != nil {
			res.Issues = append(res.Issues, Issue{Node: id, Message: err.Error(), Fatal: true})
			continue
		}
		values[id] = v
		// Bind this node's output to its variable name for downstream references.
		if n.Var != "" {
			if v.Type == TypeText {
				boundStrs[n.Var] = v.Str
			} else if num, ok := v.AsNumber(); ok {
				boundVars[n.Var] = num
			}
		}
	}

	if g.Root == "" {
		res.Issues = append(res.Issues, Issue{Message: "no output node set", Fatal: true})
		return res
	}
	root, ok := values[g.Root]
	if !ok {
		res.Issues = append(res.Issues, Issue{Node: g.Root, Message: "the card's output didn't produce a result", Fatal: true})
		return res
	}
	if root.Type != TypeViz || root.Viz == nil {
		res.Issues = append(res.Issues, Issue{Node: g.Root, Message: "the output node must be a visualization", Fatal: true})
		return res
	}
	res.Render = root.Viz
	return res
}

// coerce converts a value to the wanted port type using the language's safe coercions
// (bool→number, number/bool→text). An incompatible value is returned unchanged; the
// validator is responsible for rejecting impossible wiring before evaluation.
func coerce(v Value, want PortType) Value {
	if v.Type == want {
		return v
	}
	switch want {
	case TypeNumber:
		if n, ok := v.AsNumber(); ok {
			return Num(n)
		}
	case TypeText:
		switch v.Type {
		case TypeNumber:
			return Text(formatNumber(v.Num, ""))
		case TypeBool:
			if v.Bool {
				return Text("true")
			}
			return Text("false")
		}
	}
	return v
}
