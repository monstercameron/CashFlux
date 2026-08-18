// SPDX-License-Identifier: MIT

// Package worksheet shows an account's period as the arithmetic it actually is
// (C690):
//
//	opening + money in − money out + transfers + checkpoints = closing
//
// # Why a worksheet rather than another balance
//
// The app is full of figures that are each individually right and never shown
// beside the thing they should add up to. An account card gives a balance;
// reports give income and spending; the reconcile dialog gives a difference. When
// they disagree — and after a statement import they will — there is nowhere to
// look that says WHICH term is wrong. A worksheet is the one artefact that puts
// every term on one line and lets the reader find the odd one themselves.
//
// # The terms, and why each is separate
//
// Transfers are their own column because they are not income or spending, and
// folding them into either is the mistake that started this whole line of work.
// Checkpoints are their own column because they are a stand-in for movement
// rather than movement — reports leave them out, so a worksheet that silently
// folded them in would not reconcile against anything else in the app.
//
// Residual is the honest term: what the statement says minus what the ledger
// computes. Zero means the account is explained. Anything else is the number to
// go looking for, stated once rather than implied by two figures that disagree.
package worksheet

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reconcile"
)

// Line is one account's arithmetic for the period.
//
// Every amount is in the account's own currency and in the STATED convention —
// positive is money you have, negative is money you owe — so a debt's line reads
// the way the account card and the reconcile dialog read it, rather than in
// whichever sign the account happens to store (C683).
type Line struct {
	// Account is the account this line is about.
	Account domain.Account
	// Opening is the balance the account carried into the period.
	Opening int64
	// In and Out are money that arrived and left during the period, excluding
	// transfers and checkpoints. Out is a magnitude.
	In, Out int64
	// Transfers is the net of movement between the household's own accounts —
	// not income, not spending, and the term most often folded into one of them
	// by mistake.
	Transfers int64
	// Checkpoints is the net of provisional balance adjustments in the period.
	// Real for the balance, absent from every report, and therefore the usual
	// answer to "why does my balance not match my spending".
	Checkpoints int64
	// Computed is opening + in − out + transfers + checkpoints: what the ledger
	// says the account closed at.
	Computed int64
	// Statement is what the account's own reconciliation history says it closed
	// at, when one covers this period.
	Statement int64
	// HasStatement is false when no reconciliation covers the period, in which
	// case Residual means nothing and is left at zero.
	HasStatement bool
	// Residual is Statement − Computed. Zero means explained.
	Residual int64
}

// Explained reports whether this line needs no further investigation: either a
// statement confirms it exactly, or there is no statement to disagree with yet.
func (l Line) Explained() bool { return !l.HasStatement || l.Residual == 0 }

// Report is the worksheet for one period.
type Report struct {
	// Start and End bound the period, half-open as everywhere else in the app.
	Start, End time.Time
	// Lines is one per account, in the order the accounts were given.
	Lines []Line
}

// Unexplained returns the lines whose statement and ledger disagree — the whole
// point of running a worksheet.
func (r Report) Unexplained() []Line {
	var out []Line
	for _, l := range r.Lines {
		if !l.Explained() {
			out = append(out, l)
		}
	}
	return out
}

// Build computes the worksheet for every account over [start, end).
func Build(accounts []domain.Account, txns []domain.Transaction, start, end time.Time) Report {
	byAccount := map[string][]domain.Transaction{}
	for _, t := range txns {
		byAccount[t.AccountID] = append(byAccount[t.AccountID], t)
	}

	r := Report{Start: start, End: end}
	for _, acc := range accounts {
		r.Lines = append(r.Lines, buildLine(acc, byAccount[acc.ID], start, end))
	}
	return r
}

func buildLine(acc domain.Account, rows []domain.Transaction, start, end time.Time) Line {
	conv, _ := reconcile.ConventionOf(acc, bookedMinor(acc, rows))
	l := Line{Account: acc, Opening: reconcile.Stated(acc.OpeningBalance.Amount, conv)}

	for _, t := range rows {
		amt := reconcile.Stated(t.Amount.Amount, conv)
		switch {
		case t.Date.Before(start):
			// Everything before the period is opening balance, whatever it was.
			l.Opening += amt
		case !t.Date.Before(end):
			// After the period: not this worksheet's business.
			continue
		case t.IsBalanceCheckpoint():
			l.Checkpoints += amt
		case t.IsTransfer():
			l.Transfers += amt
		case amt > 0:
			l.In += amt
		default:
			l.Out += -amt
		}
	}
	l.Computed = l.Opening + l.In - l.Out + l.Transfers + l.Checkpoints

	if stmt, ok := statementFor(acc, start, end); ok {
		l.Statement = reconcile.Stated(stmt, conv)
		l.HasStatement = true
		l.Residual = l.Statement - l.Computed
	}
	return l
}

// bookedMinor is the account's full booked balance, used only to infer a
// liability's storage convention when its opening balance cannot.
func bookedMinor(acc domain.Account, rows []domain.Transaction) int64 {
	sum := acc.OpeningBalance.Amount
	for _, t := range rows {
		sum += t.Amount.Amount
	}
	return sum
}

// statementFor returns the closing balance of the LAST reconciliation whose
// through-date falls inside the period.
//
// The last, because a period can hold more than one statement — a card closing
// mid-month and again at month end — and the one that closes the period is the
// one the worksheet's closing figure should be checked against.
func statementFor(acc domain.Account, start, end time.Time) (int64, bool) {
	type ev struct {
		at     time.Time
		amount int64
	}
	var in []ev
	for _, rec := range acc.Reconciliations {
		through := rec.Through()
		if through.Before(start) || !through.Before(end) {
			continue
		}
		in = append(in, ev{at: through, amount: rec.StatementBalance.Amount})
	}
	if len(in) == 0 {
		return 0, false
	}
	sort.SliceStable(in, func(i, j int) bool { return in[i].at.Before(in[j].at) })
	return in[len(in)-1].amount, true
}
