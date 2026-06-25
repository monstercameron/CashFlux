// SPDX-License-Identifier: MIT

// FX-AI helpers: build prompts that ask an AI for today's exchange rates and
// leniently parse the structured JSON reply. No build tags — compiles natively
// for unit tests.
package currency

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BuildFXPrompt returns the user-turn prompt that instructs an AI to fetch
// today's mid-market exchange rates via web search and return them in a
// specific JSON format.
//
// Rate orientation: each value is how many base units one unit of the target
// currency is worth (e.g. base "USD", rate for "EUR" = 1.08 means 1 EUR = 1.08 USD).
// This matches the Rates struct convention used throughout CashFlux.
func BuildFXPrompt(base string, codes []string) string {
	codeList := strings.Join(codes, ", ")
	return fmt.Sprintf(`Use web search to find today's mid-market exchange rates.

Return ONLY a JSON object — no markdown fences, no prose, no extra keys — in exactly this shape:
{"base":"%s","asOf":"YYYY-MM-DD","rates":{"EUR":1.08,...}}

Rules:
- "base" must be "%s"
- "asOf" is today's date in YYYY-MM-DD format
- "rates" contains only these currency codes: %s
- Each rate value is a positive decimal expressing how many %s units equal ONE unit of that currency
  (example: if 1 EUR = 1.08 USD and base is USD, then "EUR": 1.08)
- Use mid-market (interbank) rates, not retail buy/sell spreads
- Omit any currency you cannot find a reliable rate for`, base, base, codeList, base)
}

// fxRatesPayload is the JSON structure the AI is asked to return.
type fxRatesPayload struct {
	Base  string             `json:"base"`
	AsOf  string             `json:"asOf"`
	Rates map[string]float64 `json:"rates"`
}

// ParseFXReply leniently extracts the first balanced JSON object from the
// model's reply (which may include prose or markdown fences) and returns
// the clean, validated rate map and the asOf date.
//
// Validation rules applied:
//   - Only codes returned by Codes() (the registered currency set) are kept.
//   - The base currency code itself is dropped (it always converts at 1.0).
//   - Non-positive rates are dropped.
//   - Unknown or extra keys are silently discarded.
//
// Returns an error when no JSON object is found, the JSON cannot be parsed,
// or no valid rates remain after filtering.
func ParseFXReply(reply, base string) (rates map[string]float64, asOf string, err error) {
	obj, err := extractFirstJSON(reply)
	if err != nil {
		return nil, "", fmt.Errorf("currency: could not find JSON in AI reply: %w", err)
	}

	var payload fxRatesPayload
	if err := json.Unmarshal([]byte(obj), &payload); err != nil {
		return nil, "", fmt.Errorf("currency: could not parse FX JSON: %w", err)
	}

	known := make(map[string]bool, len(Codes()))
	for _, c := range Codes() {
		known[c] = true
	}

	baseNorm := strings.ToUpper(strings.TrimSpace(base))
	clean := make(map[string]float64, len(payload.Rates))
	for code, rate := range payload.Rates {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == baseNorm {
			continue // base always at 1.0 — never store it
		}
		if !known[code] {
			continue // not a registered currency
		}
		if rate <= 0 {
			continue // non-positive rates are invalid
		}
		clean[code] = rate
	}

	if len(clean) == 0 {
		return nil, "", errors.New("currency: AI reply contained no valid exchange rates")
	}

	return clean, payload.AsOf, nil
}

// extractFirstJSON finds and returns the first balanced {...} JSON object
// in s. It handles the common model behaviour of wrapping the JSON in prose
// or triple-backtick markdown fences.
func extractFirstJSON(s string) (string, error) {
	start := strings.Index(s, "{")
	if start < 0 {
		return "", errors.New("no JSON object found")
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && inString {
			escape = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}
	return "", errors.New("JSON object is not balanced")
}
