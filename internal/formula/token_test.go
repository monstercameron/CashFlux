package formula

import "testing"

// kinds extracts the token kinds (dropping the trailing EOF) for assertions.
func kinds(toks []Token) []TokenKind {
	out := make([]TokenKind, 0, len(toks))
	for _, t := range toks {
		if t.Kind == TEOF {
			break
		}
		out = append(out, t.Kind)
	}
	return out
}

func eqKinds(a, b []TokenKind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTokenizeArithmetic(t *testing.T) {
	toks, err := Tokenize("1 + 2.5 * (3 - 4)")
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []TokenKind{TNumber, TOp, TNumber, TOp, TLParen, TNumber, TOp, TNumber, TRParen}
	if !eqKinds(kinds(toks), want) {
		t.Errorf("kinds = %v, want %v", kinds(toks), want)
	}
	if toks[len(toks)-1].Kind != TEOF {
		t.Error("expected trailing EOF")
	}
}

func TestTokenizeCall(t *testing.T) {
	toks, err := Tokenize("sum(income, _expense2)")
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	want := []TokenKind{TIdent, TLParen, TIdent, TComma, TIdent, TRParen}
	if !eqKinds(kinds(toks), want) {
		t.Errorf("kinds = %v, want %v", kinds(toks), want)
	}
}

func TestTokenizeComparisonsAndString(t *testing.T) {
	toks, err := Tokenize(`a >= 3 != "x"`)
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if toks[1].Kind != TOp || toks[1].Text != ">=" {
		t.Errorf("token 1 = %+v, want op >=", toks[1])
	}
	if toks[3].Kind != TOp || toks[3].Text != "!=" {
		t.Errorf("token 3 = %+v, want op !=", toks[3])
	}
	if toks[4].Kind != TString || toks[4].Text != "x" {
		t.Errorf("token 4 = %+v, want string x", toks[4])
	}
}

func TestTokenizeLeadingDotNumber(t *testing.T) {
	toks, _ := Tokenize(".5")
	if toks[0].Kind != TNumber || toks[0].Text != ".5" {
		t.Errorf("token 0 = %+v, want number .5", toks[0])
	}
}

func TestTokenizeErrors(t *testing.T) {
	if _, err := Tokenize(`"oops`); err == nil {
		t.Error("expected unterminated-string error")
	}
	if _, err := Tokenize("1 @ 2"); err == nil {
		t.Error("expected unexpected-character error")
	}
	if _, err := Tokenize("a = 3"); err == nil {
		t.Error("expected error for single '=' (assignment not allowed)")
	}
}
