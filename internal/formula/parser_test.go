package formula

import (
	"strconv"
	"strings"
	"testing"
)

// sexpr renders an AST canonically for assertions.
func sexpr(n Node) string {
	switch v := n.(type) {
	case NumberLit:
		return strconv.FormatFloat(v.Value, 'g', -1, 64)
	case StringLit:
		return `"` + v.Value + `"`
	case Ident:
		return v.Name
	case Unary:
		return "(" + v.Op + " " + sexpr(v.X) + ")"
	case Binary:
		return "(" + v.Op + " " + sexpr(v.L) + " " + sexpr(v.R) + ")"
	case Call:
		parts := make([]string, 0, len(v.Args))
		for _, a := range v.Args {
			parts = append(parts, sexpr(a))
		}
		return "(call " + v.Name + " " + strings.Join(parts, " ") + ")"
	default:
		return "?"
	}
}

func mustParse(t *testing.T, in string) Node {
	t.Helper()
	n, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse(%q): %v", in, err)
	}
	return n
}

func TestParsePrecedence(t *testing.T) {
	tests := []struct{ in, want string }{
		{"1 + 2 * 3", "(+ 1 (* 2 3))"},
		{"(1 + 2) * 3", "(* (+ 1 2) 3)"},
		{"1 - 2 - 3", "(- (- 1 2) 3)"}, // left-associative
		{"-a + 1", "(+ (- a) 1)"},
		{"a >= 3 + 1", "(>= a (+ 3 1))"},
		{"2 * 3 % 4", "(% (* 2 3) 4)"},
	}
	for _, tt := range tests {
		if got := sexpr(mustParse(t, tt.in)); got != tt.want {
			t.Errorf("Parse(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestParseCalls(t *testing.T) {
	if got := sexpr(mustParse(t, "sum(a, b * 2)")); got != "(call sum a (* b 2))" {
		t.Errorf("call = %s", got)
	}
	if got := sexpr(mustParse(t, "now()")); got != "(call now )" {
		t.Errorf("empty call = %q", got)
	}
	if got := sexpr(mustParse(t, "if(a > 0, a, 0)")); got != "(call if (> a 0) a 0)" {
		t.Errorf("nested call = %s", got)
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"1 +", "(1", "1 2", ")", "sum(a,)", "* 3"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) expected error", in)
		}
	}
}
