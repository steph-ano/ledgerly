package domain_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name        string
		amount      int64
		currency    string
		expectedErr error
	}{
		{"valid USD", 1000, "USD", nil},
		{"valid EUR zero", 0, "EUR", nil},
		{"valid negative amount", -500, "GBP", nil},
		{"invalid lowercase currency", 1000, "usd", domain.ErrInvalidCurrency},
		{"invalid 2-letter currency", 1000, "US", domain.ErrInvalidCurrency},
		{"invalid 4-letter currency", 1000, "USDT", domain.ErrInvalidCurrency},
		{"invalid symbols in currency", 1000, "U$D", domain.ErrInvalidCurrency},
		{"empty currency", 1000, "", domain.ErrInvalidCurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := domain.NewMoney(tt.amount, tt.currency)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.amount, m.Amount)
				assert.Equal(t, tt.currency, m.Currency)
			}
		})
	}
}

func TestMoney_Add(t *testing.T) {
	tests := []struct {
		name        string
		m1          domain.Money
		m2          domain.Money
		expected    domain.Money
		expectedErr error
	}{
		{
			name:     "regular addition",
			m1:       domain.MustMoney(1000, "USD"),
			m2:       domain.MustMoney(2500, "USD"),
			expected: domain.MustMoney(3500, "USD"),
		},
		{
			name:     "addition with negative",
			m1:       domain.MustMoney(1000, "USD"),
			m2:       domain.MustMoney(-400, "USD"),
			expected: domain.MustMoney(600, "USD"),
		},
		{
			name:        "currency mismatch",
			m1:          domain.MustMoney(1000, "USD"),
			m2:          domain.MustMoney(1000, "EUR"),
			expectedErr: domain.ErrCurrencyMismatch,
		},
		{
			name:        "positive overflow",
			m1:          domain.MustMoney(math.MaxInt64-10, "USD"),
			m2:          domain.MustMoney(20, "USD"),
			expectedErr: domain.ErrAmountOverflow,
		},
		{
			name:        "negative underflow",
			m1:          domain.MustMoney(math.MinInt64+10, "USD"),
			m2:          domain.MustMoney(-20, "USD"),
			expectedErr: domain.ErrAmountOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.m1.Add(tt.m2)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, res)
			}
		})
	}
}

func TestMoney_Sub(t *testing.T) {
	tests := []struct {
		name        string
		m1          domain.Money
		m2          domain.Money
		expected    domain.Money
		expectedErr error
	}{
		{
			name:     "regular subtraction",
			m1:       domain.MustMoney(3000, "USD"),
			m2:       domain.MustMoney(1200, "USD"),
			expected: domain.MustMoney(1800, "USD"),
		},
		{
			name:     "subtracting negative",
			m1:       domain.MustMoney(1000, "USD"),
			m2:       domain.MustMoney(-500, "USD"),
			expected: domain.MustMoney(1500, "USD"),
		},
		{
			name:        "currency mismatch",
			m1:          domain.MustMoney(1000, "USD"),
			m2:          domain.MustMoney(1000, "EUR"),
			expectedErr: domain.ErrCurrencyMismatch,
		},
		{
			name:        "positive overflow subtracting negative",
			m1:          domain.MustMoney(math.MaxInt64-10, "USD"),
			m2:          domain.MustMoney(-20, "USD"),
			expectedErr: domain.ErrAmountOverflow,
		},
		{
			name:        "underflow subtracting from min int",
			m1:          domain.MustMoney(math.MinInt64+5, "USD"),
			m2:          domain.MustMoney(10, "USD"),
			expectedErr: domain.ErrAmountOverflow,
		},
		{
			name:        "subtracting math.MinInt64",
			m1:          domain.MustMoney(0, "USD"),
			m2:          domain.MustMoney(math.MinInt64, "USD"),
			expectedErr: domain.ErrAmountOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.m1.Sub(tt.m2)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, res)
			}
		})
	}
}

func TestMoney_Mul(t *testing.T) {
	tests := []struct {
		name        string
		m           domain.Money
		factor      int64
		expected    domain.Money
		expectedErr error
	}{
		{
			name:     "multiply by positive",
			m:        domain.MustMoney(250, "USD"),
			factor:   4,
			expected: domain.MustMoney(1000, "USD"),
		},
		{
			name:     "multiply by zero",
			m:        domain.MustMoney(250, "USD"),
			factor:   0,
			expected: domain.MustMoney(0, "USD"),
		},
		{
			name:     "zero multiplied",
			m:        domain.MustMoney(0, "USD"),
			factor:   100,
			expected: domain.MustMoney(0, "USD"),
		},
		{
			name:        "overflow positive",
			m:           domain.MustMoney(math.MaxInt64/2+1, "USD"),
			factor:      2,
			expectedErr: domain.ErrAmountOverflow,
		},
		{
			name:        "overflow negative factor",
			m:           domain.MustMoney(math.MaxInt64/2+2, "USD"),
			factor:      -2,
			expectedErr: domain.ErrAmountOverflow,
		},
		{
			name:        "min int times negative one overflow",
			m:           domain.MustMoney(math.MinInt64, "USD"),
			factor:      -1,
			expectedErr: domain.ErrAmountOverflow,
		},
		{
			name:     "min int exact boundary fits",
			m:        domain.MustMoney(math.MaxInt64/2+1, "USD"),
			factor:   -2,
			expected: domain.MustMoney(math.MinInt64, "USD"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tt.m.Mul(tt.factor)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, res)
			}
		})
	}
}

func TestMoney_PredicatesAndString(t *testing.T) {
	zero := domain.MustMoney(0, "USD")
	pos := domain.MustMoney(100, "USD")
	neg := domain.MustMoney(-50, "USD")

	assert.True(t, zero.IsZero())
	assert.False(t, zero.IsPositive())
	assert.False(t, zero.IsNegative())

	assert.False(t, pos.IsZero())
	assert.True(t, pos.IsPositive())
	assert.False(t, pos.IsNegative())

	assert.False(t, neg.IsZero())
	assert.False(t, neg.IsPositive())
	assert.True(t, neg.IsNegative())

	assert.Equal(t, "100 USD", pos.String())
}
