// Package accountedit turns the plain-text fields of the account forms into a domain account. The
// add form and the edit form both collect these fields and both call Apply, so the class, scope,
// liability and asset rules and their validation are written once and can be tested without a
// browser. Apply reports problems as typed issues; the caller decides how to show them.
package accountedit

import (
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Field names one form field that can carry an issue.
type Field string

// The fields an issue can name.
const (
	FieldName         Field = "name"
	FieldType         Field = "type"
	FieldOwner        Field = "owner"
	FieldCurrency     Field = "currency"
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
	FieldVarName      Field = "var-name"
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

// The limits Apply holds the figures to. A rate is a percentage a year: an APR or an APY is never
// negative, and an expected return cannot lose more than everything.
const (
	maxRatePercent       = 1000
	minReturnPercent     = -100
	maxRevalueDays       = 3650
	maxScore             = 100
	maxDueDay            = 28
	maxStatementDay      = 31
	ownershipTotalShares = 100
)

// RangeOf returns the whole-number bounds a field is held to, for a field whose range issue the form
// states in its message. ok is false for a field with no stated range.
func RangeOf(f Field) (lo, hi int, ok bool) {
	switch f {
	case FieldDueDay:
		return 1, maxDueDay, true
	case FieldStatementDay:
		return 1, maxStatementDay, true
	case FieldLiquidity, FieldStability, FieldOwnerShares:
		return 0, maxScore, true
	case FieldRevalueDays:
		return 0, maxRevalueDays, true
	case FieldAPR, FieldAPY:
		return 0, maxRatePercent, true
	case FieldExpectedRet:
		return minReturnPercent, maxRatePercent, true
	}
	return 0, 0, false
}

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
	// MemberIDs lists the members that exist. When it is not nil, an owner outside it (other than the
	// group) is refused, and shares for a member outside it are ignored. A nil list skips both checks.
	MemberIDs []string
}

// ClassOf is the class of an account of the given type. An "Other" account has no natural class, so
// it is a liability only when asLiability says so; every other type has the class its type gives.
func ClassOf(t domain.AccountType, asLiability bool) domain.AccountClass {
	if t == domain.TypeOther && asLiability {
		return domain.ClassLiability
	}
	return t.Class()
}

// CountsAsDebt reports whether an existing account is an "Other" account that is counted as a debt,
// which is what the forms' "counts as a debt" switch starts from. It is the inverse of ClassOf.
func CountsAsDebt(t domain.AccountType, c domain.AccountClass) bool {
	return t == domain.TypeOther && c == domain.ClassLiability
}

// Apply writes the entered fields onto base and returns the result. base supplies the fields the form
// does not own (the ID, the custom values and the as-of date). When there are issues, the returned
// account is not meant to be saved. A type, currency or owner that the person left as it already was
// on base is not checked again, so an account saved before a rule existed can still be edited.
func Apply(base domain.Account, f Fields) (domain.Account, []Issue) {
	var issues []Issue
	add := func(field Field, reason Reason) { issues = append(issues, Issue{Field: field, Reason: reason}) }

	name := strings.TrimSpace(f.Name)
	if name == "" {
		add(FieldName, Required)
	}
	cur := strings.ToUpper(strings.TrimSpace(f.Currency))
	switch {
	case cur == "":
		add(FieldCurrency, Required)
	case cur != strings.ToUpper(strings.TrimSpace(base.Currency)) && !isCurrencyCode(cur):
		add(FieldCurrency, Invalid)
	}
	opening, ok := parseMoney(f.Opening, cur)
	if !ok {
		add(FieldOpening, Invalid)
	}

	a := base
	a.Name = name
	a.VarName = strings.TrimSpace(f.VarName)
	if a.VarName != "" && engineenv.AccountVarSlug(a.VarName) == "" {
		add(FieldVarName, Invalid)
	}
	a.Currency = cur
	a.OpeningBalance = opening
	if f.MemberIDs != nil && f.Owner != base.OwnerID && f.Owner != domain.GroupOwnerID && !contains(f.MemberIDs, f.Owner) {
		add(FieldOwner, Invalid)
	}
	a.OwnerID = f.Owner
	a.Scope = domain.ScopeIndividual
	if f.Owner == domain.GroupOwnerID {
		a.Scope = domain.ScopeShared
	}
	typ := domain.AccountType(f.Type)
	if !typ.Valid() && (typ == "" || typ != base.Type) {
		add(FieldType, Invalid)
	}
	a.Type = typ
	a.Class = ClassOf(typ, f.AsLiability)
	a.ExcludeFromNetWorth = f.ExcludeNetWorth
	a.InstitutionID = strings.TrimSpace(f.InstitutionID)
	a.Institution = strings.TrimSpace(f.Institution)
	if a.InstitutionID == "" {
		a.Institution = titleCase(a.Institution)
	}
	if lender := strings.TrimSpace(f.Lender); a.Institution == "" && a.InstitutionID == "" && a.Class == domain.ClassLiability && lender != "" {
		a.Institution = titleCase(lender)
	}
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
	switch {
	case !ok:
		add(FieldRevalueDays, Invalid)
	case revalue < 0 || revalue > maxRevalueDays:
		add(FieldRevalueDays, Range)
	}
	a.RevalueDays = revalue

	if a.Class == domain.ClassLiability {
		applyLiability(&a, f, cur, add)
	} else {
		applyAsset(&a, f, add)
	}

	a.OwnershipShares = nil
	if f.Shares != nil {
		shares := sharesOf(f.Shares, f.MemberIDs)
		if sharesOutOfRange(shares) {
			add(FieldOwnerShares, Range)
		} else if sum := sumShares(shares); sum != ownershipTotalShares {
			issues = append(issues, Issue{Field: FieldOwnerShares, Reason: Total, Sum: sum})
		}
		a.OwnershipShares = shares
	}
	return a, issues
}

// applyLiability writes the debt-only fields and clears the asset-only ones.
func applyLiability(a *domain.Account, f Fields, cur string, add func(Field, Reason)) {
	a.CreditLimit = money.Money{}
	if limit, ok := parseMoney(f.CreditLimit, cur); !ok {
		add(FieldCreditLimit, Invalid)
	} else if limit.Amount < 0 {
		add(FieldCreditLimit, Range)
	} else if limit.Amount > 0 {
		a.CreditLimit = limit
	}
	switch rate, state := parseRate(f.APR); {
	case state == rateInvalid:
		add(FieldAPR, Invalid)
		*a = a.WithoutRateAPR()
	case state == rateBlank:
		*a = a.WithoutRateAPR()
	case rate < 0 || rate > maxRatePercent:
		add(FieldAPR, Range)
		*a = a.WithoutRateAPR()
	default:
		*a = a.WithRateAPR(rate)
	}
	a.MinPayment = money.Money{}
	if minPay, ok := parseMoney(f.MinPayment, cur); !ok {
		add(FieldMinPayment, Invalid)
	} else if minPay.Amount < 0 {
		add(FieldMinPayment, Range)
	} else if minPay.Amount > 0 {
		a.MinPayment = minPay
	}
	dueDay, ok := parseInt(f.DueDay)
	if !ok || (dueDay != 0 && (dueDay < 1 || dueDay > maxDueDay)) {
		add(FieldDueDay, Range)
	}
	a.DueDayOfMonth = dueDay
	statementDay, ok := parseInt(f.StatementDay)
	if !ok || statementDay < 0 || statementDay > maxStatementDay {
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
	switch {
	case !ok:
		add(FieldExpectedRet, Invalid)
	case expected < minReturnPercent || expected > maxRatePercent:
		add(FieldExpectedRet, Range)
	}
	a.ExpectedReturnAPR = expected
	apy, ok := parseFloat(f.APY)
	switch {
	case !ok:
		add(FieldAPY, Invalid)
	case apy < 0 || apy > maxRatePercent:
		add(FieldAPY, Range)
	}
	a.APY = apy
	liquidity, ok := parseInt(f.Liquidity)
	if !ok || liquidity < 0 || liquidity > maxScore {
		add(FieldLiquidity, Range)
	}
	a.LiquidityScore = liquidity
	stability, ok := parseInt(f.Stability)
	if !ok || stability < 0 || stability > maxScore {
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
	v, ok := parseDecimal(s)
	if !ok {
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
	return parseDecimal(s)
}

// parseDecimal reads a plain decimal number, with an optional sign and exponent. Words such as NaN
// and Inf, hexadecimal and digit separators are refused, and so is a value too large to hold.
func parseDecimal(s string) (float64, bool) {
	for _, r := range s {
		if !strings.ContainsRune("0123456789+-.eE", r) {
			return 0, false
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
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

// titleCase capitalises the first letter of each word of an institution name. A word that already
// has a capital in it ("eToro", "BNP") is left as written.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if strings.IndexFunc(w, unicode.IsUpper) >= 0 {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// isCurrencyCode reports whether code has the shape of a currency code: three capital letters. The
// registry knows only the common currencies and a household may add rates for others, so the shape
// is what is checked, not membership.
func isCurrencyCode(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// contains reports whether list has s.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sharesOf copies the shares that count: all of them when memberIDs is nil, otherwise only those of
// members that exist. The copy keeps the account from aliasing the form's map.
func sharesOf(src map[string]int, memberIDs []string) map[string]int {
	out := make(map[string]int, len(src))
	for k, v := range src {
		if memberIDs == nil || contains(memberIDs, k) {
			out[k] = v
		}
	}
	return out
}

// sumShares adds up the shares.
func sumShares(shares map[string]int) int {
	sum := 0
	for _, v := range shares {
		sum += v
	}
	return sum
}

// sharesOutOfRange reports whether any share is below 0 or above 100.
func sharesOutOfRange(shares map[string]int) bool {
	for _, v := range shares {
		if v < 0 || v > ownershipTotalShares {
			return true
		}
	}
	return false
}

// minSplitMembers is the fewest members a split needs: ownership is divided between at least two.
const minSplitMembers = 2

// SplitAvailable reports whether there are enough members for the split-ownership editor to be
// offered. Both account forms use it to decide whether to show the editor and whether to hold the
// account to a share total of 100.
func SplitAvailable(memberCount int) bool { return memberCount >= minSplitMembers }

// ShareTotal is the sum of the ownership shares that count: all of them when memberIDs is nil,
// otherwise only those of members that exist. The forms show it, and Apply checks the same figure.
func ShareTotal(shares map[string]int, memberIDs []string) int {
	return sumShares(sharesOf(shares, memberIDs))
}

// SharesValid reports whether the shares that count are each from 0 to 100 and total 100, which is
// what Apply requires of a split.
func SharesValid(shares map[string]int, memberIDs []string) bool {
	counted := sharesOf(shares, memberIDs)
	return !sharesOutOfRange(counted) && sumShares(counted) == ownershipTotalShares
}

// DefaultBaseCurrency is the currency the forms use while the household has not chosen a base.
const DefaultBaseCurrency = "USD"

// BaseCurrencyOrDefault returns the household's base currency, or DefaultBaseCurrency when none is
// set. The account and service add forms both read their starting currency through it.
func BaseCurrencyOrDefault(base string) string {
	if strings.TrimSpace(base) == "" {
		return DefaultBaseCurrency
	}
	return base
}

// MoneyStep is the step of a money box in the currency: "0.01" for two decimals, "0.001" for three
// and "1" for none, so a box accepts exactly the figures the currency can hold.
func MoneyStep(cur string) string { return stepOfDecimals(currency.Decimals(cur)) }

// stepOfDecimals is the smallest step of a box that holds the given number of decimal places.
func stepOfDecimals(decimals int) string {
	if decimals <= 0 {
		return "1"
	}
	return "0." + strings.Repeat("0", decimals-1) + "1"
}

// IsLockable reports whether a lock-until date is offered at creation time for the account type: the
// asset types that can be locked or held to a maturity date (savings, investment, retirement, crypto,
// property, vehicle and the catch-all other). The liquid everyday types (checking, debit, cash) can
// still reach the field through Advanced.
func IsLockable(t domain.AccountType) bool {
	switch t {
	case domain.TypeSavings, domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto,
		domain.TypeProperty, domain.TypeVehicle, domain.TypeOther:
		return true
	}
	return false
}

// ParseShare reads one member's ownership percentage from a form box. A blank box is zero. Anything
// that is not a whole number from 0 to 100 is rejected, so the form keeps the previous figure
// instead of turning mistyped input into zero.
func ParseShare(raw string) (int, bool) {
	n, ok := parseInt(raw)
	if !ok || n < 0 || n > ownershipTotalShares {
		return 0, false
	}
	return n, true
}

// WithShare returns a copy of shares with one member's share set from the text typed in its box. A
// figure that is not a whole percentage leaves the share as it was.
func WithShare(shares map[string]int, memberID, raw string) map[string]int {
	out := make(map[string]int, len(shares)+1)
	for k, v := range shares {
		out[k] = v
	}
	if n, ok := ParseShare(raw); ok {
		out[memberID] = n
	}
	return out
}
