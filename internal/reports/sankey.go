// SPDX-License-Identifier: MIT

package reports

import "sort"

// This file lays out the Annual Review's money-flow diagram: a three-column
// sankey (income sources → income → spending categories and savings) computed
// as pure geometry so the wasm layer only draws it. Node heights are
// value-proportional on one shared scale, thin-but-real flows get a minimum
// visible height, and each node's ribbons are ordered by the far end's
// position so the bands fan out without crossing.

// Flow is one weighted link in the money-flow diagram: Value units flow
// From → To. Values must be positive; non-positive flows are dropped.
type Flow struct {
	From, To string
	Value    int64
}

// FlowNode is one laid-out sankey node: a vertical bar at column Col whose
// height encodes the total value passing through it.
type FlowNode struct {
	Label string
	Col   int // 0 = sources, 1 = hub, 2 = sinks
	Value int64
	X     float64 // left edge
	Y     float64 // top edge
	H     float64 // bar height (≥ the layout's minimum)
}

// FlowLink is one laid-out ribbon between two nodes: it leaves the source's
// right edge occupying [SY, SY+H] and lands on the target's left edge
// occupying [TY, TY+H].
type FlowLink struct {
	From, To int // indices into Nodes
	Value    int64
	SY, TY   float64
	H        float64
}

// FlowLayout is the computed diagram geometry, in the caller's pixel space.
type FlowLayout struct {
	Nodes []FlowNode
	Links []FlowLink
}

// LayoutSankey computes a sankey layout for flows inside a width×height box
// with nodeW-wide bars, gap vertical spacing between stacked nodes, and minH
// as the smallest bar height a non-zero node may render at (so a $200 trickle
// stays visible beside a $50k river). Columns come from the flow topology:
// nodes that only send are sources (col 0), nodes that both receive and send
// are the hub column (col 1), and nodes that only receive are sinks (col 2).
// Within a column, nodes stack in first-appearance order — the caller controls
// ordering by ordering its flows. Each column's stack is vertically centered.
// Empty or all-non-positive input yields a layout with no nodes.
func LayoutSankey(flows []Flow, width, height, nodeW, gap, minH float64) FlowLayout {
	type nodeAcc struct {
		label   string
		in, out int64
		order   int
	}
	byLabel := map[string]*nodeAcc{}
	var order []string
	touch := func(label string) *nodeAcc {
		if n, ok := byLabel[label]; ok {
			return n
		}
		n := &nodeAcc{label: label, order: len(order)}
		byLabel[label] = n
		order = append(order, label)
		return n
	}
	var kept []Flow
	for _, f := range flows {
		if f.Value <= 0 || f.From == f.To {
			continue
		}
		touch(f.From).out += f.Value
		touch(f.To).in += f.Value
		kept = append(kept, f)
	}
	if len(kept) == 0 {
		return FlowLayout{}
	}

	col := func(n *nodeAcc) int {
		switch {
		case n.in > 0 && n.out > 0:
			return 1
		case n.in > 0:
			return 2
		default:
			return 0
		}
	}
	total := func(n *nodeAcc) int64 {
		if n.in > n.out {
			return n.in
		}
		return n.out
	}

	// One shared value→pixels scale, chosen so the fullest column still fits
	// after gaps and minimum-height floors are paid.
	colSum := [3]int64{}
	colN := [3]int{}
	for _, label := range order {
		n := byLabel[label]
		colSum[col(n)] += total(n)
		colN[col(n)]++
	}
	scale := 0.0
	for c := 0; c < 3; c++ {
		if colN[c] == 0 {
			continue
		}
		avail := height - gap*float64(colN[c]-1) - minH*float64(colN[c])
		if avail < 0 {
			avail = 0
		}
		s := avail / float64(colSum[c])
		if scale == 0 || s < scale {
			scale = s
		}
	}
	barH := func(v int64) float64 { return minH + float64(v)*scale }

	// Place nodes: stack each column in first-appearance order, centered.
	nodes := make([]FlowNode, 0, len(order))
	idx := map[string]int{}
	colX := [3]float64{0, (width - nodeW) / 2, width - nodeW}
	for c := 0; c < 3; c++ {
		stackH := -gap
		for _, label := range order {
			if n := byLabel[label]; col(n) == c {
				stackH += barH(total(n)) + gap
			}
		}
		y := (height - stackH) / 2
		if y < 0 {
			y = 0
		}
		for _, label := range order {
			n := byLabel[label]
			if col(n) != c {
				continue
			}
			h := barH(total(n))
			idx[label] = len(nodes)
			nodes = append(nodes, FlowNode{Label: label, Col: c, Value: total(n), X: colX[c], Y: y, H: h})
			y += h + gap
		}
	}

	// Ribbon offsets: at each node edge, order that node's ribbons by the far
	// end's vertical position so bands leave and land in reading order without
	// crossing each other at the bar.
	links := make([]FlowLink, len(kept))
	for i, f := range kept {
		links[i] = FlowLink{From: idx[f.From], To: idx[f.To], Value: f.Value, H: float64(f.Value) * scale}
	}
	perNodeOut := map[int][]int{}
	perNodeIn := map[int][]int{}
	for i, l := range links {
		perNodeOut[l.From] = append(perNodeOut[l.From], i)
		perNodeIn[l.To] = append(perNodeIn[l.To], i)
	}
	for ni, ls := range perNodeOut {
		sort.SliceStable(ls, func(a, b int) bool { return nodes[links[ls[a]].To].Y < nodes[links[ls[b]].To].Y })
		y := nodes[ni].Y
		for _, li := range ls {
			links[li].SY = y
			y += links[li].H
		}
	}
	for ni, ls := range perNodeIn {
		sort.SliceStable(ls, func(a, b int) bool { return nodes[links[ls[a]].From].Y < nodes[links[ls[b]].From].Y })
		y := nodes[ni].Y
		for _, li := range ls {
			links[li].TY = y
			y += links[li].H
		}
	}
	return FlowLayout{Nodes: nodes, Links: links}
}
