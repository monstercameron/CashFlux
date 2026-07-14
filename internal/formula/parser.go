// SPDX-License-Identifier: MIT

package formula

import "strconv"

// Node is an expression AST node. Concrete types: NumberLit, StringLit, Ident,
// Unary, Binary, Call.
type Node interface{}

// NumberLit is a numeric literal.
type NumberLit struct{ Value float64 }

// StringLit is a string literal.
type StringLit struct{ Value string }

// Ident is a variable reference.
type Ident struct{ Name string }

// Unary is a prefix operation (- or +).
type Unary struct {
	Op string
	X  Node
}

// Binary is an infix operation (arithmetic or comparison).
type Binary struct {
	Op   string
	L, R Node
}

// Call is a function application by name.
type Call struct {
	Name string
	Args []Node
}

// maxDepth bounds expression nesting. Recursive descent means nesting depth is
// recursion depth; without a bound, a pathological input (deep parentheses or a
// long unary chain, e.g. from a corrupted import) overflows the goroutine stack
// — a fatal, unrecoverable crash. Real formulas nest a handful of levels.
const maxDepth = 2000

// Parse tokenizes and parses an expression into an AST, enforcing operator
// precedence (comparison < additive < multiplicative < unary < primary).
func Parse(input string) (Node, error) {
	toks, err := Tokenize(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	n, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.cur().Kind != TEOF {
		return nil, errAt(p.cur().Pos, "unexpected %q", p.cur().Text)
	}
	return n, nil
}

type parser struct {
	toks  []Token
	pos   int
	depth int
}

func (p *parser) cur() Token { return p.toks[p.pos] }

// enter guards a recursive descent step; every enter pairs with a leave.
func (p *parser) enter() error {
	p.depth++
	if p.depth > maxDepth {
		return errAt(p.cur().Pos, "formula is nested too deeply")
	}
	return nil
}

func (p *parser) leave() { p.depth-- }

func (p *parser) advance() Token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func isCompare(op string) bool {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	}
	return false
}

func (p *parser) parseExpr() (Node, error) { return p.parseComparison() }

func (p *parser) parseComparison() (Node, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	if p.cur().Kind == TOp && isCompare(p.cur().Text) {
		op := p.advance().Text
		right, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		// Chaining ("a < b < c") is rejected rather than parsed: the C-style
		// reading — compare the first result's 0/1 against c — is never what a
		// between-check means, and silently returning it is worse than an error.
		if p.cur().Kind == TOp && isCompare(p.cur().Text) {
			return nil, errAt(p.cur().Pos, "chained comparisons are not supported; write and(a %s b, b %s c)", op, p.cur().Text)
		}
		left = Binary{Op: op, L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseAdditive() (Node, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for p.cur().Kind == TOp && (p.cur().Text == "+" || p.cur().Text == "-") {
		op := p.advance().Text
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicative() (Node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.cur().Kind == TOp && (p.cur().Text == "*" || p.cur().Text == "/" || p.cur().Text == "%") {
		op := p.advance().Text
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = Binary{Op: op, L: left, R: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (Node, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	if p.cur().Kind == TOp && (p.cur().Text == "-" || p.cur().Text == "+") {
		op := p.advance().Text
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return Unary{Op: op, X: x}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.cur()
	switch t.Kind {
	case TNumber:
		p.advance()
		v, err := strconv.ParseFloat(t.Text, 64)
		if err != nil {
			return nil, errAt(t.Pos, "bad number %q", t.Text)
		}
		return NumberLit{Value: v}, nil
	case TString:
		p.advance()
		return StringLit{Value: t.Text}, nil
	case TIdent:
		p.advance()
		if p.cur().Kind == TLParen {
			return p.parseCall(t.Text)
		}
		return Ident{Name: t.Text}, nil
	case TLParen:
		if err := p.enter(); err != nil {
			return nil, err
		}
		defer p.leave()
		p.advance()
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.cur().Kind != TRParen {
			return nil, errAt(p.cur().Pos, "expected )")
		}
		p.advance()
		return inner, nil
	default:
		return nil, errAt(t.Pos, "unexpected %q", t.Text)
	}
}

func (p *parser) parseCall(name string) (Node, error) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()
	p.advance() // consume '('
	var args []Node
	if p.cur().Kind != TRParen {
		for {
			arg, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.cur().Kind == TComma {
				p.advance()
				continue
			}
			break
		}
	}
	if p.cur().Kind != TRParen {
		return nil, errAt(p.cur().Pos, "expected ) to close %s(", name)
	}
	p.advance()
	return Call{Name: name, Args: args}, nil
}
