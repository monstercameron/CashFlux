// SPDX-License-Identifier: MIT

// Package holdingimport turns a pasted or uploaded table of positions into
// holdings, previewed before anything is written (C376).
//
// Adding an investment account meant typing every position by hand, one row at a
// time, while every brokerage in existence exports exactly this table. The
// transaction importer already solved the shape of the problem — map columns,
// preview, then commit — but not the content: a holdings row is a POSITION, not
// an event, so importing the same file twice must update what you hold rather
// than double it.
//
// Three rules follow from that and drive everything here:
//
//   - Match on account + ticker, and fall back to account + name when a position
//     has no ticker. A position is identified by what it IS, not by when it was
//     imported.
//   - A match UPDATES; it never adds. Importing Monday's export and then
//     Friday's should leave you holding what Friday said, not both.
//   - A blank cell means "leave it alone", not zero. Brokerage exports routinely
//     omit cost basis, and treating that as $0 would silently report every
//     position as pure gain.
//
// Pure Go: parsing, matching and planning only. The caller writes.
package holdingimport

import (
	"errors"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Field names a column's meaning.
type Field string

const (
	// FieldNone leaves a column unmapped.
	FieldNone Field = ""
	// FieldTicker is the exchange symbol.
	FieldTicker Field = "ticker"
	// FieldName is the security or fund name.
	FieldName Field = "name"
	// FieldShares is the number of units held.
	FieldShares Field = "shares"
	// FieldCostBasis is the TOTAL acquisition cost for the position.
	FieldCostBasis Field = "costBasis"
	// FieldPrice is the current price PER SHARE.
	FieldPrice Field = "price"
	// FieldAssetClass is the broad class label ("Stocks", "Bonds", …).
	FieldAssetClass Field = "assetClass"
	// FieldSector and FieldRegion are the optional allocation dimensions (C377).
	FieldSector Field = "sector"
	FieldRegion Field = "region"
)

// Profile maps each column index of the source table to a Field. Columns beyond
// the profile, or mapped to FieldNone, are ignored.
type Profile struct {
	// Columns is per-index; its length need not match a row's.
	Columns []Field
	// HasHeader skips the first row.
	HasHeader bool
	// Decimals is the currency's minor-unit exponent (2 for dollars), used to
	// turn "1,234.56" into 123456.
	Decimals int
}

// Field returns the mapping for a column index, or FieldNone when unmapped.
func (p Profile) Field(i int) Field {
	if i < 0 || i >= len(p.Columns) {
		return FieldNone
	}
	return p.Columns[i]
}

// Mapped reports whether a field is mapped to any column.
func (p Profile) Mapped(f Field) bool {
	for _, c := range p.Columns {
		if c == f {
			return true
		}
	}
	return false
}

// ErrNoIdentity is returned when a profile maps neither ticker nor name. Without
// one of them a row cannot be matched against what is already held, so every
// import would duplicate the whole account.
var ErrNoIdentity = errors.New("map a ticker or a name column so positions can be matched")

// ErrNoShares is returned when a profile maps no shares column. A position with
// no quantity is not a holding.
var ErrNoShares = errors.New("map a shares column")

// Validate reports why a profile cannot be used, or nil.
func (p Profile) Validate() error {
	if !p.Mapped(FieldTicker) && !p.Mapped(FieldName) {
		return ErrNoIdentity
	}
	if !p.Mapped(FieldShares) {
		return ErrNoShares
	}
	return nil
}

// Parsed is one source row after mapping, with the fields that were actually
// PRESENT flagged. Presence matters as much as value: a blank cost-basis cell
// must leave an existing basis alone rather than zero it.
type Parsed struct {
	Ticker     string
	Name       string
	Shares     float64
	HasShares  bool
	CostMinor  int64
	HasCost    bool
	PriceMinor int64
	HasPrice   bool
	AssetClass string
	Sector     string
	Region     string
	// Line is the 1-based source row, so a problem can be pointed at.
	Line int
	// Err describes why the row cannot be used, or is empty.
	Err string
}

// Usable reports whether the row can become a holding.
func (p Parsed) Usable() bool { return p.Err == "" }

// Key is the row's match identity: its ticker if it has one, else its name,
// folded and trimmed.
func (p Parsed) Key() string {
	if k := strings.ToLower(strings.TrimSpace(p.Ticker)); k != "" {
		return k
	}
	return strings.ToLower(strings.TrimSpace(p.Name))
}

// Parse maps rows through a profile. Every row comes back, including bad ones,
// each carrying its own error: a preview that silently drops the rows it could
// not read is how a user commits an import believing it covered everything.
func Parse(p Profile, rows [][]string) []Parsed {
	if p.HasHeader && len(rows) > 0 {
		rows = rows[1:]
	}
	out := make([]Parsed, 0, len(rows))
	for i, cols := range rows {
		out = append(out, parseRow(p, cols, i+1))
	}
	return out
}

func parseRow(p Profile, cols []string, line int) Parsed {
	r := Parsed{Line: line}
	for i, raw := range cols {
		v := strings.TrimSpace(raw)
		switch p.Field(i) {
		case FieldTicker:
			r.Ticker = strings.ToUpper(v)
		case FieldName:
			r.Name = v
		case FieldShares:
			if v == "" {
				continue
			}
			n, err := parseFloat(v)
			if err != nil {
				r.Err = "shares is not a number: " + v
				continue
			}
			r.Shares, r.HasShares = n, true
		case FieldCostBasis:
			if v == "" {
				continue
			}
			n, err := parseMinor(v, p.Decimals)
			if err != nil {
				r.Err = "cost basis is not an amount: " + v
				continue
			}
			r.CostMinor, r.HasCost = n, true
		case FieldPrice:
			if v == "" {
				continue
			}
			n, err := parseMinor(v, p.Decimals)
			if err != nil {
				r.Err = "price is not an amount: " + v
				continue
			}
			r.PriceMinor, r.HasPrice = n, true
		case FieldAssetClass:
			r.AssetClass = v
		case FieldSector:
			r.Sector = v
		case FieldRegion:
			r.Region = v
		}
	}
	if r.Err == "" {
		switch {
		case r.Key() == "":
			r.Err = "no ticker or name"
		case !r.HasShares:
			r.Err = "no share count"
		case r.Shares < 0:
			r.Err = "negative share count"
		}
	}
	return r
}

// Action names what committing a row would do.
type Action string

const (
	// ActionAdd creates a new holding.
	ActionAdd Action = "add"
	// ActionUpdate changes an existing one.
	ActionUpdate Action = "update"
	// ActionSkip leaves everything alone — the row is unusable, or it would
	// change nothing.
	ActionSkip Action = "skip"
)

// Change is one planned write, with the before and after so a preview can show
// what actually moves rather than just "12 rows".
type Change struct {
	Action Action
	// Row is the parsed source row.
	Row Parsed
	// Before is the existing holding for an update (zero for an add).
	Before domain.Holding
	// After is the holding as it would be written (zero for a skip).
	After domain.Holding
	// Reason explains a skip.
	Reason string
}

// Plan works out what importing rows into an account would do, without writing.
//
// existing is the account's current holdings; anything belonging to another
// account is ignored rather than matched, so importing into the wrong account
// cannot silently rewrite the right one.
//
// newID supplies ids for added holdings. It is a parameter because id generation
// is not pure and this package is.
func Plan(accountID string, existing []domain.Holding, rows []Parsed, newID func() string) []Change {
	byKey := map[string]domain.Holding{}
	for _, h := range existing {
		if h.AccountID != accountID {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(h.Ticker))
		if k == "" {
			k = strings.ToLower(strings.TrimSpace(h.Name))
		}
		if k != "" {
			byKey[k] = h
		}
	}
	out := make([]Change, 0, len(rows))
	for _, r := range rows {
		if !r.Usable() {
			out = append(out, Change{Action: ActionSkip, Row: r, Reason: r.Err})
			continue
		}
		if cur, ok := byKey[r.Key()]; ok {
			next := applyTo(cur, r)
			if sameImportedFields(next, cur) {
				out = append(out, Change{Action: ActionSkip, Row: r, Before: cur,
					Reason: "already matches"})
				continue
			}
			out = append(out, Change{Action: ActionUpdate, Row: r, Before: cur, After: next})
			continue
		}
		h := applyTo(domain.Holding{AccountID: accountID}, r)
		if newID != nil {
			h.ID = newID()
		}
		out = append(out, Change{Action: ActionAdd, Row: r, After: h})
		// A file listing the same position twice updates the first result rather
		// than adding two rows for it.
		byKey[r.Key()] = h
	}
	return out
}

// applyTo folds a parsed row onto a holding, writing only what the row actually
// carried. A blank cell leaves the existing value alone — brokerage exports
// routinely omit cost basis, and treating that as zero would report every
// position as pure gain.
// sameImportedFields reports whether an import would leave a holding unchanged.
//
// It compares the fields applyTo can touch rather than the whole struct: a
// holding also carries tax lots and a price date that no import row sets, and
// `==` on the struct stopped compiling the moment lots arrived. Comparing only
// what the import writes is also the more honest test — "already matches" should
// mean "this row has nothing to add", not "every unrelated field happens to be
// equal too".
func sameImportedFields(a, b domain.Holding) bool {
	return a.Ticker == b.Ticker &&
		a.Name == b.Name &&
		a.Shares == b.Shares &&
		a.CostBasisMinor == b.CostBasisMinor &&
		a.CurrentPriceMinorPerShare == b.CurrentPriceMinorPerShare &&
		a.AssetClass == b.AssetClass &&
		a.Sector == b.Sector &&
		a.Region == b.Region
}

func applyTo(h domain.Holding, r Parsed) domain.Holding {
	if r.Ticker != "" {
		h.Ticker = r.Ticker
	}
	if r.Name != "" {
		h.Name = r.Name
	}
	if r.HasShares {
		h.Shares = r.Shares
	}
	if r.HasCost {
		h.CostBasisMinor = r.CostMinor
	}
	if r.HasPrice {
		h.CurrentPriceMinorPerShare = r.PriceMinor
	}
	if r.AssetClass != "" {
		h.AssetClass = r.AssetClass
	}
	if r.Sector != "" {
		h.Sector = r.Sector
	}
	if r.Region != "" {
		h.Region = r.Region
	}
	// A holding with a ticker and no name is unreadable in a list; fall back to
	// the ticker rather than showing a blank row.
	if strings.TrimSpace(h.Name) == "" {
		h.Name = h.Ticker
	}
	return h
}

// Summary counts a plan, for the sentence above a preview table.
type Summary struct {
	Add, Update, Skip int
}

// Total is how many rows the plan covers.
func (s Summary) Total() int { return s.Add + s.Update + s.Skip }

// Writes is how many rows would actually change something.
func (s Summary) Writes() int { return s.Add + s.Update }

// Summarize counts a plan by action.
func Summarize(changes []Change) Summary {
	var s Summary
	for _, c := range changes {
		switch c.Action {
		case ActionAdd:
			s.Add++
		case ActionUpdate:
			s.Update++
		default:
			s.Skip++
		}
	}
	return s
}

// GuessProfile maps a header row by column name, so the common brokerage export
// needs no manual mapping at all. Unrecognised headers map to FieldNone rather
// than to a guess: a column silently read as "price" when it was "day change"
// would corrupt every position in the file.
func GuessProfile(header []string, decimals int) Profile {
	p := Profile{Columns: make([]Field, len(header)), HasHeader: true, Decimals: decimals}
	for i, h := range header {
		p.Columns[i] = guessField(h)
	}
	return p
}

func guessField(h string) Field {
	s := strings.ToLower(strings.TrimSpace(h))
	s = strings.ReplaceAll(s, "_", " ")
	switch {
	case s == "":
		return FieldNone
	case contains(s, "symbol", "ticker"):
		return FieldTicker
	case contains(s, "description", "security", "fund name", "name"):
		return FieldName
	// "quantity"/"shares"/"units" — checked before price so "share price" does
	// not read as a share count.
	case contains(s, "quantity", "qty"), s == "shares", s == "units":
		return FieldShares
	case contains(s, "cost basis", "total cost", "book value"):
		return FieldCostBasis
	case contains(s, "last price", "price", "market price", "nav"):
		return FieldPrice
	case contains(s, "asset class", "class", "category"):
		return FieldAssetClass
	case contains(s, "sector", "industry"):
		return FieldSector
	case contains(s, "region", "geography", "country"):
		return FieldRegion
	}
	return FieldNone
}

func contains(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// parseFloat accepts thousands separators and a leading currency symbol, which
// brokerage exports use freely.
func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(cleanNumeric(s), 64)
}

// parseMinor turns a decimal amount into minor units, rounding half away from
// zero at the currency's precision.
func parseMinor(s string, decimals int) (int64, error) {
	if decimals < 0 {
		decimals = 0
	}
	f, err := strconv.ParseFloat(cleanNumeric(s), 64)
	if err != nil {
		return 0, err
	}
	mul := 1.0
	for range decimals {
		mul *= 10
	}
	v := f * mul
	if v < 0 {
		return int64(v - 0.5), nil
	}
	return int64(v + 0.5), nil
}

// cleanNumeric strips the decoration exports carry: currency symbols, thousands
// separators, spaces, and accounting-style parentheses for negatives.
func cleanNumeric(s string) string {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
	if neg {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r == '.', r == '-', r == '+':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if neg && out != "" && !strings.HasPrefix(out, "-") {
		out = "-" + out
	}
	return out
}
