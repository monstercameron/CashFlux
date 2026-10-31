// Package accountedit turns the plain-text fields of the account forms into a domain account. The
// add form and the edit form both collect these fields and both call Apply, so the class, scope,
// liability and asset rules and their validation are written once and can be tested without a
// browser. Apply reports problems as typed issues; the caller decides how to show them.
package accountedit

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Field names one form field that can carry an issue.
type Field string

// The fields an issue can name.
const (
	FieldName         Field = "name"
	FieldOpening      Field = "opening"
	FieldCreditLimit  Field = "credit-limit"
	FieldAPR          Field = "apr"
	FieldAPY          Field = "apy"
	FieldMinPayment   Field = "min-payment"
	FieldDueDay       Field = "due-day"
	FieldStatementDay Field = "statement-day"
	FieldExpectedRet  Field = "expected-return"
	FieldLiquidity    Field = "liquidity"
	FieldStability    Field = "stability"
	FieldLockUntil    Field = "lock-until"
	FieldSnoozeUntil  Field = "snooze-until"
	FieldRevalueDays  Field = "revalue-days"
	FieldOwnerShares  Field = "owner-shares"
)

// Reason says why a field has an issue.
type Reason string

// The reasons an issue can carry.
const (
	Required Reason = "required"
	Invalid  Reason = "invalid"
	Range    Reason = "range"
	Total    Reason = "total"
)

// Issue is one problem with the entered fields. Sum is set for a share total that is not 100.
type Issue struct {
	Field  Field
	Reason Reason
	Sum    int
}

// Fields is everything the account forms collect, as plain text, before it is checked.
type Fields struct {
	Name            string
	Type            string
	Owner           string
	Currency        string
	Opening         string
	AsLiability     bool
	CreditLimit     string
	APR             string
	APY             string
	MinPayment      string
	DueDay          string
	StatementDay    string
	Lender          string
	ExpectedReturn  string
	Liquidity       string
	Stability       string
	LockUntil       string
	Institution     string
	InstitutionID   string
	Notes           string
	BeneficiaryNote string
	RevalueDays     string
	FreshnessExempt bool
	SnoozeUntil     string
	ExcludeNetWorth bool
	VarName         string
	// Shares is the split-ownership map, or nil when the account is not split.
	Shares map[string]int
}

// Apply writes the entered fields onto base and returns the result. base supplies the fields the form
// does not own (the ID, the custom values and the as-of date). When there are issues, the returned
// account is not meant to be saved.
func Apply(base domain.Account, f Fields) (domain.Account, []Issue) {
	var issues []Issue
	add := func(field Field, reason Reason) { issues = append(issues, Issue{Field: field, Reason: reason}) }

	name := strings.TrimSpace(f.Name)
	if name == "" {
		add(FieldName, Required)
	}
	cur := strings.ToUpper(strings.TrimSpace(f.Currency))
	opening, ok := parseMoney(f.Opening, cur)
	if !ok {
		add(FieldOpening, Invalid)
	}

	a := base
	a.Name = name
	a.VarName = strings.TrimSpace(f.VarName)
	a.Currency = cur
	a.OpeningBalance = opening
	a.OwnerID = f.Owner
	a.Scope = domain.ScopeIndividual
	if f.Owner == domain.GroupOwnerID {
		a.Scope = domain.ScopeShared
	}
	typ := domain.AccountType(f.Type)
	a.Type = typ
	a.Class = typ.Class()
	if typ == domain.TypeOther && f.AsLiability {
		a.Class = domain.ClassLiability
	}
	a.ExcludeFromNetWorth = f.ExcludeNetWorth
	a.Institution = titleCase(strings.TrimSpace(f.Institution))
	if a.Institution == "" && strings.TrimSpace(f.Lender) != "" {
		a.Institution = titleCase(strings.TrimSpace(f.Lender))
	}
	a.InstitutionID = f.InstitutionID
	a.Notes = strings.TrimSpace(f.Notes)
	a.BeneficiaryNote = strings.TrimSpace(f.BeneficiaryNote)
	a.FreshnessExempt = f.FreshnessExempt
	a.FreshnessSnoozeUntil = time.Time{}
	if snooze, ok := parseDate(f.SnoozeUntil); !ok {
		add(FieldSnoozeUntil, Invalid)
	} else {
		a.FreshnessSnoozeUntil = snooze
	}
	revalue, ok := parseInt(f.RevalueDays)
	if !ok || revalue < 0 {
		add(FieldRevalueDays, Invalid)
	}
	a.RevalueDays = revalue

	if a.Class == domain.ClassLiability {
		applyLiability(&a, f, cur, add)
	} else {
		applyAsset(&a, f, add)
	}

	a.OwnershipShares = nil
	if f.Shares != nil {
		sum := 0
		for _, v := range f.Shares {
			sum += v
		}
		if sum != 100 {
			issues = append(issues, Issue{Field: FieldOwnerShares, Reason: Total, Sum: sum})
		}
		a.OwnershipShares = cloneShares(f.Shares)
	}
	return a, issues
}

// applyLiability writes the debt-only fields and clears the asset-only ones.
func applyLiability(a *domain.Account, f Fields, cur string, add func(Field, Reason)) {
	a.CreditLimit = money.Money{}
	if limit, ok := parseMoney(f.CreditLimit, cur); !ok {
		add(FieldCreditLimit, Invalid)
	} else if limit.Amount > 0 {
		a.CreditLimit = limit
	}
	switch rate, state := parseRate(f.APR); state {
	case rateInvalid:
		add(FieldAPR, Invalid)
		*a = a.WithoutRateAPR()
	case rateBlank:
		*a = a.WithoutRateAPR()
	default:
		*a = a.WithRateAPR(rate)
	}
	a.MinPayment = money.Money{}
	if minPay, ok := parseMoney(f.MinPayment, cur); !ok {
		add(FieldMinPayment, Invalid)
	} else if minPay.Amount > 0 {
		a.MinPayment = minPay
	}
	dueDay, ok := parseInt(f.DueDay)
	if !ok || (dueDay != 0 && (dueDay < 1 || dueDay > 28)) {
		add(FieldDueDay, Range)
	}
	a.DueDayOfMonth = dueDay
	statementDay, ok := parseInt(f.StatementDay)
	if !ok || statementDay < 0 || statementDay > 31 {
		add(FieldStatementDay, Range)
	}
	a.StatementDay = statementDay
	a.Lender = strings.TrimSpace(f.Lender)
	a.ExpectedReturnAPR = 0
	a.APY = 0
	a.LiquidityScore = 0
	a.StabilityScore = 0
	a.LockUntil = time.Time{}
}

// applyAsset writes the asset-only figures and clears the debt-only ones.
func applyAsset(a *domain.Account, f Fields, add func(Field, Reason)) {
	expected, ok := parseFloat(f.ExpectedReturn)
	if !ok {
		add(FieldExpectedRet, Invalid)
	}
	a.ExpectedReturnAPR = expected
	apy, ok := parseFloat(f.APY)
	if !ok {
		add(FieldAPY, Invalid)
	}
	a.APY = apy
	liquidity, ok := parseInt(f.Liquidity)
	if !ok || liquidity < 0 || liquidity > 100 {
		add(FieldLiquidity, Range)
	}
	a.LiquidityScore = liquidity
	stability, ok := parseInt(f.Stability)
	if !ok || stability < 0 || stability > 100 {
		add(FieldStability, Range)
	}
	a.StabilityScore = stability
	a.LockUntil = time.Time{}
	if lock, ok := parseDate(f.LockUntil); !ok {
		add(FieldLockUntil, Invalid)
	} else {
		a.LockUntil = lock
	}
	*a = a.WithoutRateAPR()
	a.CreditLimit = money.Money{}
	a.MinPayment = money.Money{}
	a.DueDayOfMonth = 0
	a.StatementDay = 0
	a.Lender = ""
}

// rateState is what a rate box holds: nothing, a number, or something that is not a number.
type rateState int

const (
	rateBlank rateState = iota
	rateSet
	rateInvalid
)

// parseRate reads an APR box. A blank box means no rate was recorded. Zero is a rate of 0%, which is
// different from blank.
func parseRate(raw string) (float64, rateState) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, rateBlank
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, rateInvalid
	}
	return v, rateSet
}

// parseMoney reads an amount in the currency's minor units. A blank box is zero.
func parseMoney(raw, cur string) (money.Money, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return money.New(0, cur), true
	}
	minor, err := money.ParseMinor(s, currency.Decimals(cur))
	if err != nil {
		return money.Money{Currency: cur}, false
	}
	return money.New(minor, cur), true
}

// parseInt reads a whole number. A blank box is zero.
func parseInt(raw string) (int, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// parseFloat reads a decimal number. A blank box is zero.
func parseFloat(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, true
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// parseDate reads a calendar date. A blank box is the zero time.
func parseDate(raw string) (time.Time, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, true
	}
	d, err := dateutil.ParseDate(s)
	return d, err == nil
}

// titleCase capitalises each word of an institution name.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// cloneShares copies a share map so the account does not alias the form's map.
func cloneShares(src map[string]int) map[string]int {
	out := make(map[string]int, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
