// SPDX-License-Identifier: MIT

package formula

import (
	"fmt"
	"math"
	"strings"
)

// Value is a formula result: float64, string, or bool. Nothing else can be
// produced — the language has no objects, pointers, or host references.
type Value interface{}

// Env supplies the variables a formula may reference. Numeric names resolve from
// Vars, string names from Strs; anything else is an "unknown variable" error. A
// name present in both resolves as a number (Vars wins).
type Env struct {
	Vars map[string]float64
	Strs map[string]string
}

// Eval parses and evaluates an expression against env, returning a number,
// string, or bool. Errors on parse failure, unknown variable/function, arity
// mismatch, division by zero, or a type mismatch.
func Eval(input string, env Env) (Value, error) {
	ast, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return eval(ast, env)
}

func eval(n Node, env Env) (Value, error) {
	switch v := n.(type) {
	case NumberLit:
		return v.Value, nil
	case StringLit:
		return v.Value, nil
	case Ident:
		if num, ok := env.Vars[v.Name]; ok {
			return num, nil
		}
		if s, ok := env.Strs[v.Name]; ok {
			return s, nil
		}
		return nil, fmt.Errorf("formula: unknown variable %q", v.Name)
	case Unary:
		x, err := evalNumber(v.X, env)
		if err != nil {
			return nil, err
		}
		if v.Op == "-" {
			return -x, nil
		}
		return x, nil
	case Binary:
		return evalBinary(v, env)
	case Call:
		return evalCall(v, env)
	default:
		return nil, fmt.Errorf("formula: cannot evaluate %T", n)
	}
}

func evalBinary(b Binary, env Env) (Value, error) {
	switch b.Op {
	case "==", "!=":
		l, err := eval(b.L, env)
		if err != nil {
			return nil, err
		}
		r, err := eval(b.R, env)
		if err != nil {
			return nil, err
		}
		ls, lok := l.(string)
		rs, rok := r.(string)
		if lok && rok {
			if b.Op == "==" {
				return ls == rs, nil
			}
			return ls != rs, nil
		}
		ln, lerr := asNumber(l)
		rn, rerr := asNumber(r)
		if lerr != nil || rerr != nil {
			return nil, fmt.Errorf("formula: cannot compare %v and %v", l, r)
		}
		if b.Op == "==" {
			return ln == rn, nil
		}
		return ln != rn, nil
	}

	l, err := evalNumber(b.L, env)
	if err != nil {
		return nil, err
	}
	r, err := evalNumber(b.R, env)
	if err != nil {
		return nil, err
	}
	switch b.Op {
	case "+":
		return l + r, nil
	case "-":
		return l - r, nil
	case "*":
		return l * r, nil
	case "/":
		if r == 0 {
			return nil, fmt.Errorf("formula: division by zero")
		}
		return l / r, nil
	case "%":
		if r == 0 {
			return nil, fmt.Errorf("formula: modulo by zero")
		}
		return math.Mod(l, r), nil
	case "<":
		return l < r, nil
	case "<=":
		return l <= r, nil
	case ">":
		return l > r, nil
	case ">=":
		return l >= r, nil
	}
	return nil, fmt.Errorf("formula: unknown operator %q", b.Op)
}

func evalCall(c Call, env Env) (Value, error) {
	args := make([]Value, len(c.Args))
	for i, a := range c.Args {
		v, err := eval(a, env)
		if err != nil {
			return nil, err
		}
		args[i] = v
	}

	nums := func() ([]float64, error) {
		out := make([]float64, len(args))
		for i, a := range args {
			f, err := asNumber(a)
			if err != nil {
				return nil, fmt.Errorf("formula: %s() expects numbers", c.Name)
			}
			out[i] = f
		}
		return out, nil
	}

	switch c.Name {
	case "sum":
		ns, err := nums()
		if err != nil {
			return nil, err
		}
		total := 0.0
		for _, n := range ns {
			total += n
		}
		return total, nil
	case "avg":
		if len(args) == 0 {
			return nil, fmt.Errorf("formula: avg() needs at least one argument")
		}
		ns, err := nums()
		if err != nil {
			return nil, err
		}
		total := 0.0
		for _, n := range ns {
			total += n
		}
		return total / float64(len(ns)), nil
	case "min", "max":
		if len(args) == 0 {
			return nil, fmt.Errorf("formula: %s() needs at least one argument", c.Name)
		}
		ns, err := nums()
		if err != nil {
			return nil, err
		}
		best := ns[0]
		for _, n := range ns[1:] {
			if (c.Name == "min" && n < best) || (c.Name == "max" && n > best) {
				best = n
			}
		}
		return best, nil
	case "count":
		return float64(len(args)), nil
	case "abs":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: abs() takes 1 argument")
		}
		f, err := asNumber(args[0])
		if err != nil {
			return nil, err
		}
		return math.Abs(f), nil
	case "round":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: round() takes 1 argument")
		}
		f, err := asNumber(args[0])
		if err != nil {
			return nil, err
		}
		return math.Round(f), nil
	case "floor":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: floor() takes 1 argument")
		}
		f, err := asNumber(args[0])
		if err != nil {
			return nil, err
		}
		return math.Floor(f), nil
	case "ceil":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: ceil() takes 1 argument")
		}
		f, err := asNumber(args[0])
		if err != nil {
			return nil, err
		}
		return math.Ceil(f), nil
	case "clamp":
		// clamp(value, lo, hi) → value bounded to [lo, hi]. The readable way to keep a
		// derived figure (e.g. a savings-rate %) within sensible bounds.
		if len(args) != 3 {
			return nil, fmt.Errorf("formula: clamp() takes 3 arguments")
		}
		v, err := asNumber(args[0])
		if err != nil {
			return nil, err
		}
		lo, err := asNumber(args[1])
		if err != nil {
			return nil, err
		}
		hi, err := asNumber(args[2])
		if err != nil {
			return nil, err
		}
		if lo > hi {
			lo, hi = hi, lo
		}
		return math.Max(lo, math.Min(hi, v)), nil
	case "safediv":
		// safediv(a, b, fallback) → a/b, or fallback when b == 0. The zero-guarded
		// division every ratio KPI needs (savings rate, utilization, …).
		if len(args) != 3 {
			return nil, fmt.Errorf("formula: safediv() takes 3 arguments")
		}
		a, err := asNumber(args[0])
		if err != nil {
			return nil, err
		}
		b, err := asNumber(args[1])
		if err != nil {
			return nil, err
		}
		fb, err := asNumber(args[2])
		if err != nil {
			return nil, err
		}
		if b == 0 {
			return fb, nil
		}
		return a / b, nil
	case "if":
		if len(args) != 3 {
			return nil, fmt.Errorf("formula: if() takes 3 arguments")
		}
		if truthy(args[0]) {
			return args[1], nil
		}
		return args[2], nil
	case "contains":
		// contains(haystack, needle) → case-insensitive substring test; the
		// everyday matcher for payee/description rules ("merchant" in payee).
		if len(args) != 2 {
			return nil, fmt.Errorf("formula: contains() takes 2 arguments")
		}
		hay, ok1 := args[0].(string)
		needle, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("formula: contains() expects strings")
		}
		return strings.Contains(strings.ToLower(hay), strings.ToLower(needle)), nil
	case "lower":
		if len(args) != 1 {
			return nil, fmt.Errorf("formula: lower() takes 1 argument")
		}
		s, ok := args[0].(string)
		if !ok {
			return nil, fmt.Errorf("formula: lower() expects a string")
		}
		return strings.ToLower(s), nil
	default:
		return nil, fmt.Errorf("formula: unknown function %q", c.Name)
	}
}

func evalNumber(n Node, env Env) (float64, error) {
	v, err := eval(n, env)
	if err != nil {
		return 0, err
	}
	return asNumber(v)
}

func asNumber(v Value) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("formula: expected a number, got %v", v)
	}
}

func truthy(v Value) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	default:
		return false
	}
}
