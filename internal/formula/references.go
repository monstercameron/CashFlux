// SPDX-License-Identifier: MIT

package formula

import "sort"

// References returns the distinct variable identifiers an expression reads, in
// sorted order. It is the audit primitive: given a compound formula (a molecule
// like "assets - liabilities"), it lists the atoms the figure depends on so the
// derivation can be traced. Function names (sum, clamp, …) are not identifiers and
// are excluded.
func References(expr string) ([]string, error) {
	ast, err := Parse(expr)
	if err != nil {
		return nil, err
	}
	return collectRefs(ast), nil
}

// collectRefs walks an AST and returns its distinct identifiers, sorted.
func collectRefs(ast Node) []string {
	seen := map[string]bool{}
	var walk func(Node)
	walk = func(n Node) {
		switch v := n.(type) {
		case Ident:
			seen[v.Name] = true
		case Unary:
			walk(v.X)
		case Binary:
			walk(v.L)
			walk(v.R)
		case Call:
			for _, a := range v.Args {
				walk(a)
			}
		}
	}
	walk(ast)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
