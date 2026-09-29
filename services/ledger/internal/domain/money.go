package domain

import (
	"fmt"
	"math"
	"regexp"
)

var currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

// Money represents a monetary value with integer cents and an ISO 4217 currency code.
// Floating-point representations are strictly prohibited.
type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// NewMoney creates a new Money value with currency validation.
func NewMoney(amount int64, currency string) (Money, error) {
	if !currencyRegex.MatchString(currency) {
		return Money{}, ErrInvalidCurrency
	}
	return Money{
		Amount:   amount,
		Currency: currency,
	}, nil
}

// MustMoney creates a new Money value, panicking on invalid currency (useful in tests).
func MustMoney(amount int64, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// Add adds two Money values of the same currency with overflow check.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}

	a := m.Amount
	b := other.Amount

	// Overflow checking for 64-bit signed integers
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return Money{}, ErrAmountOverflow
	}

	return Money{
		Amount:   a + b,
		Currency: m.Currency,
	}, nil
}

// Sub subtracts other Money from m with overflow check.
func (m Money) Sub(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}

	a := m.Amount
	b := other.Amount

	// Subtraction overflow: a - b is equivalent to a + (-b)
	if b == math.MinInt64 {
		if a >= 0 {
			return Money{}, ErrAmountOverflow
		}
	} else if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
		return Money{}, ErrAmountOverflow
	}

	return Money{
		Amount:   a - b,
		Currency: m.Currency,
	}, nil
}

// Mul multiplies Money by an integer factor with overflow check.
func (m Money) Mul(factor int64) (Money, error) {
	if factor == 0 || m.Amount == 0 {
		return Money{Amount: 0, Currency: m.Currency}, nil
	}

	a := m.Amount
	b := factor

	// Special case: math.MinInt64 * -1 overflows 64-bit signed integer
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return Money{}, ErrAmountOverflow
	}

	c := a * b
	if c/b != a {
		return Money{}, ErrAmountOverflow
	}

	return Money{
		Amount:   c,
		Currency: m.Currency,
	}, nil
}

// IsZero returns true if amount is 0.
func (m Money) IsZero() bool {
	return m.Amount == 0
}

// IsPositive returns true if amount is strictly greater than 0.
func (m Money) IsPositive() bool {
	return m.Amount > 0
}

// IsNegative returns true if amount is strictly less than 0.
func (m Money) IsNegative() bool {
	return m.Amount < 0
}

// String prints standard representation e.g. "1050 USD"
func (m Money) String() string {
	return fmt.Sprintf("%d %s", m.Amount, m.Currency)
}
