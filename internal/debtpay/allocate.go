package debtpay

import (
	"errors"
	"fmt"
	"math/big"
)

var (
	// ErrInvalidPayment means the cash payment amount is not a positive minor-unit amount.
	ErrInvalidPayment = errors.New("payment amount must be positive")
	// ErrInvalidLoan means the loan balance, rate, or kind cannot be used for allocation.
	ErrInvalidLoan = errors.New("invalid loan details")
	// ErrInvalidSplit means statement components are negative or do not add to payment.
	ErrInvalidSplit = errors.New("statement split does not equal payment")
)

// Allocate splits a payment using a supplied statement breakdown or a monthly
// amortization estimate. Estimated interest uses APR basis points divided by
// 12 and rounds to the nearest minor unit, ties to even (banker's rounding).
func Allocate(payment Payment, loan Loan, statementSplit *Split) (Allocation, error) {
	if payment.AmountMinor <= 0 {
		return Allocation{}, ErrInvalidPayment
	}
	if loan.BalanceMinor < 0 {
		return Allocation{}, ErrInvalidLoan
	}
	if statementSplit != nil {
		if !validSplit(payment.AmountMinor, *statementSplit) {
			return Allocation{}, ErrInvalidSplit
		}
		return makeAllocation(payment.AmountMinor, loan.BalanceMinor, *statementSplit, false, AllocationStatement), nil
	}
	if loan.Kind == AccountCard {
		return makeAllocation(payment.AmountMinor, loan.BalanceMinor, Split{PrincipalMinor: payment.AmountMinor}, false, AllocationCard), nil
	}
	if loan.Kind != AccountLoan || !loan.HasAPR || loan.APRBasisPoints < 0 {
		return Allocation{}, ErrInvalidLoan
	}
	interest := roundEven(new(big.Int).Mul(big.NewInt(loan.BalanceMinor), big.NewInt(loan.APRBasisPoints)), big.NewInt(120000))
	if !interest.IsInt64() || interest.Sign() < 0 {
		return Allocation{}, fmt.Errorf("calculate monthly interest: %w", ErrInvalidLoan)
	}
	if interest.Int64() > payment.AmountMinor {
		interest.SetInt64(payment.AmountMinor)
	}
	split := Split{PrincipalMinor: payment.AmountMinor - interest.Int64(), InterestMinor: interest.Int64()}
	return makeAllocation(payment.AmountMinor, loan.BalanceMinor, split, true, AllocationAmortized), nil
}

func validSplit(payment int64, split Split) bool {
	values := []int64{split.PrincipalMinor, split.InterestMinor, split.FeesMinor, split.EscrowMinor}
	total := new(big.Int)
	for _, value := range values {
		if value < 0 {
			return false
		}
		total.Add(total, big.NewInt(value))
	}
	return total.Cmp(big.NewInt(payment)) == 0
}

func makeAllocation(payment, balance int64, split Split, estimated bool, method AllocationMethod) Allocation {
	overpayment := int64(0)
	if split.PrincipalMinor > balance {
		overpayment = split.PrincipalMinor - balance
	}
	return Allocation{
		PaymentMinor:     payment,
		PrincipalMinor:   split.PrincipalMinor,
		InterestMinor:    split.InterestMinor,
		FeesMinor:        split.FeesMinor,
		EscrowMinor:      split.EscrowMinor,
		Estimated:        estimated,
		Method:           method,
		Overpayment:      overpayment > 0,
		OverpaymentMinor: overpayment,
	}
}

func roundEven(numerator, denominator *big.Int) *big.Int {
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	twiceRemainder := new(big.Int).Lsh(remainder, 1)
	comparison := twiceRemainder.Cmp(denominator)
	if comparison > 0 || (comparison == 0 && quotient.Bit(0) == 1) {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}
