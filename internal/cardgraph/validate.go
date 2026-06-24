package cardgraph

import "fmt"

// Validate reports structural problems with a graph before evaluation: unknown node
// kinds, edges to/from missing nodes or ports, type-incompatible wiring, an input port
// wired more than once, a missing/invalid root, and cycles. An empty result means the
// graph is well-formed (though nodes may still error at eval time on bad config).
func Validate(g Graph) []Issue {
	var issues []Issue

	ids := map[NodeID]Node{}
	seenVar := map[string]NodeID{}
	for _, n := range g.Nodes {
		if _, dup := ids[n.ID]; dup {
			issues = append(issues, Issue{Node: n.ID, Message: "duplicate node id", Fatal: true})
		}
		ids[n.ID] = n
		if _, ok := Lookup(n.Kind); !ok {
			issues = append(issues, Issue{Node: n.ID, Message: fmt.Sprintf("unknown node kind %q", n.Kind), Fatal: true})
		}
		// A named output must be a valid, unique identifier so downstream references
		// resolve unambiguously.
		if n.Var != "" {
			if !ValidIdent(n.Var) {
				issues = append(issues, Issue{Node: n.ID, Message: fmt.Sprintf("%q is not a valid variable name (use a letter or _ then letters/digits)", n.Var)})
			} else if prev, dup := seenVar[n.Var]; dup {
				issues = append(issues, Issue{Node: n.ID, Message: fmt.Sprintf("variable name %q is already used by node %s", n.Var, prev)})
			} else {
				seenVar[n.Var] = n.ID
			}
		}
	}

	// inPort/outType resolve a port ref's declared type; ok=false means the port
	// doesn't exist on that node kind.
	inType := func(ref PortRef) (PortType, bool) {
		n, ok := ids[ref.Node]
		if !ok {
			return "", false
		}
		spec, ok := Lookup(n.Kind)
		if !ok {
			return "", false
		}
		for _, p := range spec.Inputs {
			if p.Name == ref.Port {
				return p.Type, true
			}
		}
		return "", false
	}
	outType := func(ref PortRef) (PortType, bool) {
		n, ok := ids[ref.Node]
		if !ok {
			return "", false
		}
		spec, ok := Lookup(n.Kind)
		if !ok {
			return "", false
		}
		if ref.Port != OutPort {
			return "", false
		}
		return spec.Out, true
	}

	wired := map[string]bool{} // "node:port" of inputs already connected (one wire each)
	for _, e := range g.Edges {
		ot, okOut := outType(e.From)
		if !okOut {
			issues = append(issues, Issue{Node: e.From.Node, Message: fmt.Sprintf("edge from unknown output %s", e.From), Fatal: true})
			continue
		}
		it, okIn := inType(e.To)
		if !okIn {
			issues = append(issues, Issue{Node: e.To.Node, Message: fmt.Sprintf("edge into unknown input %s", e.To), Fatal: true})
			continue
		}
		if !CanFeed(ot, it) {
			issues = append(issues, Issue{Node: e.To.Node, Message: fmt.Sprintf("%s (%s) can't feed %s (%s)", e.From, ot, e.To, it), Fatal: true})
		}
		key := e.To.String()
		if wired[key] {
			issues = append(issues, Issue{Node: e.To.Node, Message: fmt.Sprintf("input %s is wired more than once", e.To), Fatal: true})
		}
		wired[key] = true
	}

	if g.Root == "" {
		issues = append(issues, Issue{Message: "no output node set", Fatal: true})
	} else if root, ok := ids[g.Root]; !ok {
		issues = append(issues, Issue{Node: g.Root, Message: "the output node doesn't exist", Fatal: true})
	} else if spec, ok := Lookup(root.Kind); ok && spec.Out != TypeViz {
		issues = append(issues, Issue{Node: g.Root, Message: "the output node must be a visualization", Fatal: true})
	}

	if _, err := TopoOrder(g); err != nil {
		issues = append(issues, Issue{Message: err.Error(), Fatal: true})
	}

	return issues
}
