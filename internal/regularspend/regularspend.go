// SPDX-License-Identifier: MIT

// Package regularspend calculates an explainable household regular-spend
// baseline from an immutable snapshot. It contains no persistence or UI code.
package regularspend

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/debtpay"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberallocation"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Rate expresses one major unit of a currency as a rational number of base
// currency major units. Rational rates keep conversion deterministic.
type Rate struct{ Numerator, Denominator int64 }

// Rates is the FX table used for this calculation. Base currency implicitly
// has a rate of one.
type Rates struct {
	Base       string
	ByCurrency map[string]Rate
}

// Schedule is a recurring expense that the user has explicitly confirmed.
// TransactionIDs link posted history to this obligation for deduplication.
type Schedule struct {
	Recurring        domain.Recurring
	Confirmed        bool
	EndedAt          time.Time
	DebtAccountID    string
	TransactionIDs   []string
	Owners           []string
	DetectedSeriesID string
}

// DetectedSeries is a repeated charge pattern awaiting explicit confirmation.
// Amount must be positive and expresses one occurrence in its source currency.
type DetectedSeries struct {
	ID             string
	Name           string
	Amount         money.Money
	Cadence        domain.RecurringCadence
	PaymentCount   int
	Confirmed      bool
	Paused         bool
	EndedAt        time.Time
	DueDate        time.Time
	CategoryID     string
	AccountID      string
	ScheduleID     string
	TransactionIDs []string
	Owners         []string
}

// Overrides contains baseline-only user choices. Essential category roots
// include their descendants; one-off IDs never change reports or budgets.
type Overrides struct {
	EssentialCategoryIDs []string
	OneOffTransactionIDs []string
	TreatMerchantRegular []string
}

// Input is one household snapshot and the explicit date against which activity
// and schedules are evaluated.
type Input struct {
	Transactions   []domain.Transaction
	Categories     []domain.Category
	Schedules      []Schedule
	DetectedSeries []DetectedSeries
	Accounts       []domain.Account
	// Ownership optionally supplies an authoritative transaction-ID to owner-ID
	// mapping. When absent, domain.Transaction.Owners is used.
	Ownership       map[string][]string
	DebtPaymentData debtpay.Dataset
	Rates           Rates
	Overrides       Overrides
	AsOf            time.Time
	HistoryMonths   int
}

// Amounts holds base currency and unconverted source-currency totals. A missing
// FX rate never causes unlike minor units to be added.
type Amounts struct {
	Base       money.Money
	ByCurrency map[string]money.Money
	Incomplete bool
}

// Contributor explains one fixed obligation or one essential category estimate.
type Contributor struct {
	ID                  string
	Label               string
	SourceIDs           []string
	Evidence            string
	Cadence             domain.RecurringCadence
	MonthlyEquivalent   money.Money
	OriginalAmount      money.Money
	DueMonths           []time.Month
	Owners              []string
	SampleMonths        []string
	SampleSize          int
	SeasonalUnavailable bool
	Incomplete          bool
	ConversionRate      Rate
	HasConversionRate   bool
	Unestimated         bool
}

// MemberAmounts apportions an amount across named members, with distinct
// shared and unassigned buckets.
type MemberAmounts struct {
	ByMember   map[string]money.Money
	Shared     money.Money
	Unassigned money.Money
}

// Result is the two-number baseline and its deterministic supporting detail.
type Result struct {
	Fixed               Amounts
	TypicalEssential    Amounts
	Contributors        []Contributor
	SuggestedEssentials []string
	DataQualityNotes    []string
	AsOf                time.Time
}

// Build calculates fixed commitments and typical essential month. The median
// uses up to the six completed calendar months before AsOf, with at least three
// observed months required for each selected category.
func Build(in Input) (Result, error) {
	if in.AsOf.IsZero() {
		return Result{}, fmt.Errorf("regularspend: AsOf is required")
	}
	base := strings.ToUpper(strings.TrimSpace(in.Rates.Base))
	if base == "" {
		return Result{}, fmt.Errorf("regularspend: base currency is required")
	}
	in.Rates.Base = base
	out := Result{Fixed: Amounts{Base: money.Zero(base), ByCurrency: map[string]money.Money{}}, TypicalEssential: Amounts{Base: money.Zero(base), ByCurrency: map[string]money.Money{}}, AsOf: in.AsOf}
	covered := make(map[string]bool)
	merchantOverrides := stringSet(in.Overrides.TreatMerchantRegular)
	for _, s := range in.Schedules {
		for _, id := range s.TransactionIDs {
			covered[id] = true
		}
	}
	for _, s := range in.DetectedSeries {
		for _, id := range s.TransactionIDs {
			covered[id] = true
		}
	}
	lines := make([]Contributor, 0)
	for _, s := range in.Schedules {
		r := s.Recurring
		if !s.Confirmed || r.Paused || (!s.EndedAt.IsZero() && !s.EndedAt.After(in.AsOf)) {
			continue
		}
		if r.Amount.Amount == 0 {
			out.DataQualityNotes = append(out.DataQualityNotes, fmt.Sprintf("%s: amount is unknown and was left unestimated.", r.Label))
			out.Fixed.Incomplete = true
			continue
		}
		if r.Amount.Amount > 0 {
			continue
		}
		line, err := fixedLine(r.ID, r.Label, "scheduled", r.Amount.Abs(), r.Cadence, nil, s.Owners, in.Rates)
		if err != nil {
			out.DataQualityNotes = append(out.DataQualityNotes, err.Error())
			if line.ID == "" {
				continue
			}
		}
		line.SourceIDs = []string{r.ID}
		if !r.NextDue.IsZero() {
			line.DueMonths = dueMonths(r.Cadence, r.NextDue.Month())
		}
		lines = append(lines, line)
	}
	series := append([]DetectedSeries(nil), in.DetectedSeries...)
	sort.Slice(series, func(i, j int) bool { return series[i].ID < series[j].ID })
	for _, s := range series {
		if (!s.Confirmed && !merchantOverrides[merchantKey(s.Name, "")]) || s.Paused || (!s.EndedAt.IsZero() && !s.EndedAt.After(in.AsOf)) || s.ScheduleID != "" {
			continue
		}
		if s.Amount.Amount == 0 || s.Cadence == "" {
			out.DataQualityNotes = append(out.DataQualityNotes, "Unestimated detected series: amount or cadence is unknown.")
			out.Fixed.Incomplete = true
			continue
		}
		line, err := fixedLine(s.ID, s.Name, fmt.Sprintf("detected from %d payments", s.PaymentCount), s.Amount.Abs(), s.Cadence, nil, s.Owners, in.Rates)
		if err != nil {
			out.DataQualityNotes = append(out.DataQualityNotes, err.Error())
			if line.ID == "" {
				continue
			}
		}
		line.SourceIDs = []string{s.ID}
		if !s.DueDate.IsZero() {
			line.DueMonths = dueMonths(s.Cadence, s.DueDate.Month())
		}
		lines = append(lines, line)
	}
	// Active liability minimums are included once; an explicitly linked schedule
	// represents that same debt obligation.
	for _, a := range in.Accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.MinPayment.Amount <= 0 {
			continue
		}
		linked := false
		for _, s := range in.Schedules {
			if s.Confirmed && !s.Recurring.Paused && (s.EndedAt.IsZero() || s.EndedAt.After(in.AsOf)) && s.DebtAccountID == a.ID {
				linked = true
				break
			}
		}
		if linked {
			continue
		}
		owners := []string{}
		if a.OwnerID != "" && a.OwnerID != domain.GroupOwnerID {
			owners = []string{a.OwnerID}
		}
		line, err := fixedLine("minimum:"+a.ID, a.Name, "minimum payment entered", a.MinPayment.Abs(), domain.CadenceMonthly, nil, owners, in.Rates)
		if err != nil {
			out.DataQualityNotes = append(out.DataQualityNotes, err.Error())
			if line.ID == "" {
				continue
			}
		}
		line.SourceIDs = []string{a.ID}
		lines = append(lines, line)
	}
	// Classification is suggestion-only: a supported lender payment is not a
	// second purchase expense, and ambiguous rows remain visible for review.
	debtCandidates := make(map[string]bool)
	for _, txn := range in.Transactions {
		candidate := debtpay.Classify(debtTransaction(txn), in.DebtPaymentData)
		if candidate.Confidence != debtpay.ConfidenceNone {
			debtCandidates[txn.ID] = true
			out.DataQualityNotes = append(out.DataQualityNotes, fmt.Sprintf("%s may be a debt payment; review its classification before treating it as essential spending.", txn.ID))
		}
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Label != lines[j].Label {
			return lines[i].Label < lines[j].Label
		}
		return lines[i].ID < lines[j].ID
	})
	for _, line := range lines {
		out.Contributors = append(out.Contributors, line)
		if line.Unestimated || line.Incomplete {
			out.Fixed.Incomplete = true
		}
		addAmount(&out.Fixed, line.MonthlyEquivalent)
	}
	if out.Fixed.Incomplete {
		out.DataQualityNotes = append(out.DataQualityNotes, "Some fixed commitments are unestimated or lack an exchange rate; the base total is incomplete.")
	}
	essentialRoots := append([]string(nil), in.Overrides.EssentialCategoryIDs...)
	essentialCats := categorytree.DescendantsOfAll(in.Categories, essentialRoots)
	out.SuggestedEssentials = suggestEssentials(in.Categories)
	if len(essentialRoots) == 0 {
		out.DataQualityNotes = append(out.DataQualityNotes, "Choose essential categories to estimate typical variable essentials.")
		out.TypicalEssential.Incomplete = true
	}
	maxMonths := in.HistoryMonths
	if maxMonths <= 0 || maxMonths > 12 {
		maxMonths = 6
	}
	end := monthStart(in.AsOf)
	start := end.AddDate(0, 0, 0).AddDate(0, -maxMonths, 0)
	monthly := map[string]map[string]int64{}
	categoryOwners := map[string]map[string]bool{}
	for i := 0; i < maxMonths; i++ {
		month := start.AddDate(0, i, 0)
		monthly[monthKey(month)] = map[string]int64{}
	}
	ones := stringSet(in.Overrides.OneOffTransactionIDs)
	for _, t := range in.Transactions {
		if t.ID == "" || t.Date.IsZero() || t.Date.After(in.AsOf) || t.TransferAccountID != "" || t.TransferGroupID != "" || t.ExcludeFromReports || !t.BalanceCheckpointAt.IsZero() || ones[t.ID] || debtCandidates[t.ID] || !t.CountsInReports() {
			continue
		}
		if covered[t.ID] {
			continue
		}
		key := monthKey(t.Date)
		bucket, ok := monthly[key]
		if !ok {
			continue
		}
		postings := transactionPostings(t)
		for _, p := range postings {
			if !essentialCats[p.category] {
				continue
			}
			key := p.category + "\x00" + strings.ToUpper(p.currency)
			bucket[key] -= p.amount
			if categoryOwners[key] == nil {
				categoryOwners[key] = map[string]bool{}
			}
			owners, hasOwnership := in.Ownership[t.ID]
			if !hasOwnership {
				owners = t.Owners()
			}
			for _, owner := range owners {
				if owner != "" {
					categoryOwners[key][owner] = true
				}
			}
		}
	}
	for _, categoryID := range sortedKeys(essentialCats) {
		currencies := map[string]bool{}
		for _, bucket := range monthly {
			for key := range bucket {
				if strings.HasPrefix(key, categoryID+"\x00") {
					currencies[strings.TrimPrefix(key, categoryID+"\x00")] = true
				}
			}
		}
		if len(currencies) == 0 {
			currencies[base] = true
		}
		for _, code := range sortedKeys(currencies) {
			values := make([]int64, 0, maxMonths)
			sampleMonths := make([]string, 0, maxMonths)
			for i := 0; i < maxMonths; i++ {
				m := start.AddDate(0, i, 0)
				if value, ok := monthly[monthKey(m)][categoryID+"\x00"+code]; ok {
					values = append(values, value)
					sampleMonths = append(sampleMonths, monthKey(m))
				}
			}
			if len(values) < 3 {
				out.DataQualityNotes = append(out.DataQualityNotes, fmt.Sprintf("%s (%s): needs at least three observed complete months; found %d.", categoryName(in.Categories, categoryID), code, len(values)))
				out.TypicalEssential.Incomplete = true
				continue
			}
			med := median(values)
			line := Contributor{ID: "essential:" + categoryID + ":" + code, Label: categoryName(in.Categories, categoryID), SourceIDs: []string{categoryID}, Evidence: fmt.Sprintf("median of %d complete months", len(values)), MonthlyEquivalent: money.New(med, code), OriginalAmount: money.New(med, code), SampleMonths: sampleMonths, SampleSize: len(values), SeasonalUnavailable: maxMonths >= 12 && !hasTwoComparableYears(sampleMonths)}
			for owner := range categoryOwners[categoryID+"\x00"+code] {
				line.Owners = append(line.Owners, owner)
			}
			sort.Strings(line.Owners)
			if code != base {
				converted, conversionErr := convertMinor(med, code, in.Rates)
				if conversionErr != nil {
					line.Incomplete = true
					out.DataQualityNotes = append(out.DataQualityNotes, fmt.Sprintf("%s: %v", line.Label, conversionErr))
				} else {
					line.MonthlyEquivalent = money.New(converted, base)
					line.ConversionRate = in.Rates.ByCurrency[code]
					line.HasConversionRate = true
				}
			}
			if line.SeasonalUnavailable {
				out.DataQualityNotes = append(out.DataQualityNotes, line.Label+": seasonal comparison is unavailable without two comparable years.")
			}
			out.Contributors = append(out.Contributors, line)
			addAmount(&out.TypicalEssential, line.MonthlyEquivalent)
		}
	}
	// The essential-month headline includes the fixed baseline plus variable essentials.
	addAmounts(&out.TypicalEssential, out.Fixed)
	if out.TypicalEssential.Incomplete {
		out.DataQualityNotes = append(out.DataQualityNotes, "The typical essential month has incomplete history or missing exchange rates.")
	}
	sort.Strings(out.DataQualityNotes)
	return out, nil
}

// AllocateToMembers returns the member allocation of a contributor. Shared and
// unassigned costs remain separately visible; this helper does not alter totals.
func AllocateToMembers(amount int64, currencyCode string, owners []string) MemberAmounts {
	out := MemberAmounts{ByMember: map[string]money.Money{}, Shared: money.Zero(currencyCode), Unassigned: money.Zero(currencyCode)}
	if len(owners) == 0 {
		out.Unassigned = money.New(amount, currencyCode)
		return out
	}
	if len(owners) > 1 {
		out.Shared = money.New(amount, currencyCode)
	}
	for _, share := range memberallocation.SplitAmount(amount, owners) {
		out.ByMember[share.MemberID] = money.New(share.Amount, currencyCode)
	}
	return out
}

// AllocateContributor applies WHO1 largest-remainder ownership to a contributor
// while preserving its household amount exactly.
func AllocateContributor(contributor Contributor) MemberAmounts {
	return AllocateToMembers(contributor.MonthlyEquivalent.Amount, contributor.MonthlyEquivalent.Currency, contributor.Owners)
}

type posting struct {
	category, currency string
	amount             int64
}

func debtTransaction(t domain.Transaction) debtpay.Transaction {
	return debtpay.Transaction{ID: t.ID, AccountID: t.AccountID, Date: t.Date, Payee: t.Payee, Description: t.Desc, AmountMinor: t.Amount.Amount, Currency: t.Amount.Currency, TransferGroupID: t.TransferGroupID, TransferAccountID: t.TransferAccountID}
}

func transactionPostings(t domain.Transaction) []posting {
	if len(t.Splits) > 0 {
		out := make([]posting, 0, len(t.Splits))
		for _, s := range t.Splits {
			out = append(out, posting{s.CategoryID, s.Amount.Currency, s.Amount.Amount})
		}
		return out
	}
	return []posting{{t.CategoryID, t.Amount.Currency, t.Amount.Amount}}
}
func fixedLine(id, label, evidence string, amount money.Money, cadence domain.RecurringCadence, due []time.Month, owners []string, rates Rates) (Contributor, error) {
	monthly, err := normalizeCadence(amount.Amount, cadence)
	if err != nil {
		return Contributor{ID: id, Label: label, Evidence: evidence, Cadence: cadence, OriginalAmount: amount, Unestimated: true, Owners: append([]string(nil), owners...)}, fmt.Errorf("%s: %w", label, err)
	}
	line := Contributor{ID: id, Label: label, Evidence: evidence, Cadence: cadence, OriginalAmount: amount, MonthlyEquivalent: money.New(monthly, amount.Currency), Owners: append([]string(nil), owners...), DueMonths: due}
	if rates.Base != amount.Currency {
		converted, err := convertMinor(monthly, amount.Currency, rates)
		if err != nil {
			line.Incomplete = true
			return line, err
		}
		line.MonthlyEquivalent = money.New(converted, rates.Base)
		line.ConversionRate = rates.ByCurrency[strings.ToUpper(amount.Currency)]
		line.HasConversionRate = true
	}
	return line, nil
}
func normalizeCadence(amount int64, c domain.RecurringCadence) (int64, error) {
	n, d := int64(1), int64(1)
	switch c {
	case domain.CadenceWeekly:
		n, d = 52, 12
	case domain.CadenceBiweekly:
		n, d = 26, 12
	case domain.CadenceSemimonthly:
		n = 2
	case domain.CadenceMonthly:
		n = 1
	case domain.CadenceQuarterly:
		d = 3
	case domain.CadenceYearly:
		d = 12
	default:
		return 0, fmt.Errorf("unknown cadence %q; amount left unestimated", c)
	}
	numerator := new(big.Int).Mul(big.NewInt(amount), big.NewInt(n))
	return roundRatio(numerator, big.NewInt(d)), nil
}
func convertMinor(amount int64, code string, rates Rates) (int64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == rates.Base {
		return amount, nil
	}
	r, ok := rates.ByCurrency[code]
	if !ok || r.Denominator <= 0 || r.Numerator <= 0 {
		return 0, fmt.Errorf("missing valid FX rate for %s to %s", code, rates.Base)
	}
	fromScale := pow10Minor(code)
	toScale := pow10Minor(rates.Base)
	n := new(big.Int).Mul(big.NewInt(amount), big.NewInt(r.Numerator))
	n.Mul(n, big.NewInt(toScale))
	d := new(big.Int).Mul(big.NewInt(r.Denominator), big.NewInt(fromScale))
	return roundRatio(n, d), nil
}
func roundRatio(n, d *big.Int) int64 {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(n, d, r)
	twice := new(big.Int).Lsh(new(big.Int).Abs(r), 1)
	if twice.Cmp(d) >= 0 {
		if n.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return q.Int64()
}
func addAmount(total *Amounts, m money.Money) {
	code := strings.ToUpper(m.Currency)
	if code == "" {
		return
	}
	if m.Currency == total.Base.Currency {
		total.Base.Amount += m.Amount
		return
	}
	old, ok := total.ByCurrency[code]
	if !ok {
		old = money.Zero(code)
	}
	old.Amount += m.Amount
	total.ByCurrency[code] = old
	total.Incomplete = true
}
func addAmounts(a *Amounts, b Amounts) {
	a.Base.Amount += b.Base.Amount
	for c, m := range b.ByCurrency {
		x := a.ByCurrency[c]
		x.Amount += m.Amount
		a.ByCurrency[c] = x
	}
	a.Incomplete = a.Incomplete || b.Incomplete
}
func dueMonths(c domain.RecurringCadence, month time.Month) []time.Month {
	switch c {
	case domain.CadenceYearly:
		return []time.Month{month}
	case domain.CadenceQuarterly:
		return []time.Month{month, time.Month((int(month)+2)%12 + 1), time.Month((int(month)+5)%12 + 1), time.Month((int(month)+8)%12 + 1)}
	default:
		return nil
	}
}
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
func monthKey(t time.Time) string { return t.Format("2006-01") }
func median(v []int64) int64 {
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	sum := new(big.Int).Add(big.NewInt(v[n/2-1]), big.NewInt(v[n/2]))
	return roundRatio(sum, big.NewInt(2))
}
func hasTwoComparableYears(months []string) bool {
	seen := map[string]map[string]bool{}
	for _, m := range months {
		t, err := time.Parse("2006-01", m)
		if err != nil {
			continue
		}
		k := t.Format("01")
		if seen[k] == nil {
			seen[k] = map[string]bool{}
		}
		seen[k][t.Format("2006")] = true
	}
	for _, years := range seen {
		if len(years) >= 2 {
			return true
		}
	}
	return false
}
func categoryName(cats []domain.Category, id string) string {
	for _, c := range cats {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}
func suggestEssentials(cats []domain.Category) []string {
	targets := map[string]bool{"groceries": true, "grocery": true, "utilities": true, "fuel": true, "gas": true, "insurance": true, "housing": true, "rent": true, "mortgage": true}
	out := []string{}
	for _, c := range cats {
		if c.Kind != domain.KindExpense {
			continue
		}
		n := strings.ToLower(strings.TrimSpace(c.Name))
		if targets[n] {
			out = append(out, c.ID)
		}
	}
	sort.Strings(out)
	return out
}
func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[strings.ToLower(strings.TrimSpace(v))] = true
	}
	return out
}
func merchantKey(payee, desc string) string {
	v := payee
	if strings.TrimSpace(v) == "" {
		v = desc
	}
	return strings.ToLower(strings.Join(strings.Fields(v), " "))
}
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func pow10Minor(code string) int64 {
	decimals := currency.Decimals(code)
	if decimals < 0 || decimals > 6 {
		return 100
	}
	result := int64(1)
	for i := 0; i < decimals; i++ {
		result *= 10
	}
	return result
}
