// SPDX-License-Identifier: MIT

// Package formula implements a small, sandboxed expression language for
// user-defined calculations: a tokenizer, parser, and allow-list evaluator over
// numbers, strings, arithmetic, comparisons, and a fixed set of functions. No
// access to the host, filesystem, or arbitrary code — only the values and
// functions explicitly provided. Pure Go, unit-tested on native Go.
//
// This file is the tokenizer (lexer).
package formula

import (
	"strings"
	"unicode/utf8"
)

// TokenKind classifies a lexical token.
type TokenKind string

// The token kinds.
const (
	TNumber TokenKind = "number"
	TIdent  TokenKind = "ident"
	TString TokenKind = "string"
	TOp     TokenKind = "op"
	TLParen TokenKind = "lparen"
	TRParen TokenKind = "rparen"
	TComma  TokenKind = "comma"
	TEOF    TokenKind = "eof"
)

// Token is a single lexical token with its source position.
type Token struct {
	Kind TokenKind
	Text string
	Pos  int
}

// MaxSourceLen caps how long a formula may be. Formulas are typed (or pasted)
// by people; the cap exists so a pathological string arriving through dataset
// import or an AI tool can't tie up the tokenizer or feed the parser something
// enormous.
const MaxSourceLen = 32 * 1024

// Tokenize splits an expression into tokens, ending with a TEOF token. It
// errors on an over-long input, an unterminated string, a malformed number, or
// an unexpected character. Numbers accept scientific notation (1e3, 2.5E-2).
func Tokenize(input string) ([]Token, error) {
	if len(input) > MaxSourceLen {
		return nil, &Error{Pos: -1, Msg: "formula is too long"}
	}
	var toks []Token
	i, n := 0, len(input)
	for i < n {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(input[i+1])):
			start := i
			dots := 0
			for i < n && (isDigit(input[i]) || input[i] == '.') {
				if input[i] == '.' {
					dots++
				}
				i++
			}
			// Optional exponent, only when a digit follows — so "2 e" or a
			// trailing "1e" never swallows an identifier.
			if i < n && (input[i] == 'e' || input[i] == 'E') {
				j := i + 1
				if j < n && (input[j] == '+' || input[j] == '-') {
					j++
				}
				if j < n && isDigit(input[j]) {
					i = j
					for i < n && isDigit(input[i]) {
						i++
					}
				}
			}
			if dots > 1 {
				return nil, errAt(start, "bad number %q", input[start:i])
			}
			toks = append(toks, Token{TNumber, input[start:i], start})
		case isAlpha(c) || c == '_':
			start := i
			for i < n && (isAlnum(input[i]) || input[i] == '_') {
				i++
			}
			toks = append(toks, Token{TIdent, input[start:i], start})
		case c == '"':
			start := i
			i++ // opening quote
			for i < n && input[i] != '"' {
				i++
			}
			if i >= n {
				return nil, errAt(start, "unterminated string")
			}
			toks = append(toks, Token{TString, input[start+1 : i], start})
			i++ // closing quote
		case c == '(':
			toks = append(toks, Token{TLParen, "(", i})
			i++
		case c == ')':
			toks = append(toks, Token{TRParen, ")", i})
			i++
		case c == ',':
			toks = append(toks, Token{TComma, ",", i})
			i++
		case strings.IndexByte("+-*/%", c) >= 0:
			toks = append(toks, Token{TOp, string(c), i})
			i++
		case c == '=' || c == '!' || c == '<' || c == '>':
			start := i
			op := string(c)
			if i+1 < n && input[i+1] == '=' {
				op += "="
				i += 2
			} else {
				if c == '=' || c == '!' {
					return nil, errAt(i, "unexpected %q (did you mean %q?)", string(c), string(c)+"=")
				}
				i++
			}
			toks = append(toks, Token{TOp, op, start})
		default:
			// Decode the full rune so a non-ASCII character reads as itself in
			// the error ("é"), not as its mangled first byte.
			r, _ := utf8.DecodeRuneInString(input[i:])
			return nil, errAt(i, "unexpected character %q", string(r))
		}
	}
	toks = append(toks, Token{TEOF, "", n})
	return toks, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isAlnum(c byte) bool { return isAlpha(c) || isDigit(c) }
