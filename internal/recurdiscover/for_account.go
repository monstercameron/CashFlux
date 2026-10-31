// SPDX-License-Identifier: MIT

package recurdiscover

// ForAccount returns the candidates detected on one account, in discovery order.
// A limit above zero caps the list; zero or less returns every match.
func (r Result) ForAccount(accountID string, limit int) []Candidate {
	var out []Candidate
	for _, c := range r.Candidates {
		if c.AccountID != accountID {
			continue
		}
		out = append(out, c)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
