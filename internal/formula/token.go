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
	"fmt"
	"strings"
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

// Tokenize splits an expression into tokens, ending with a TEOF token. It errors
// on an unterminated string or an unexpected character.
func Tokenize(input string) ([]Token, error) {
	var toks []Token
	i, n := 0, len(input)
	for i < n {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case isDigit(c) || (c == '.' && i+1 < n && isDigit(input[i+1])):
			start := i
			for i < n && (isDigit(input[i]) || input[i] == '.') {
				i++
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
				return nil, fmt.Errorf("formula: unterminated string at position %d", start)
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
					return nil, fmt.Errorf("formula: unexpected %q at position %d (did you mean %q=?)", c, i, c)
				}
				i++
			}
			toks = append(toks, Token{TOp, op, start})
		default:
			return nil, fmt.Errorf("formula: unexpected character %q at position %d", string(c), i)
		}
	}
	toks = append(toks, Token{TEOF, "", n})
	return toks, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isAlnum(c byte) bool { return isAlpha(c) || isDigit(c) }
