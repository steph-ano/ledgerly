package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
)

func TestNewOrder_InstallmentDivision(t *testing.T) {
	now := time.Now().UTC()
	custID := uuid.New()
	merchID := uuid.New()

	t.Run("evenly divisible amount: $100.00 (10000 cents)", func(t *testing.T) {
		order, err := domain.NewOrder(uuid.New(), "client_1", custID, merchID, 10000, "USD", "https://merchant.com/webhook", now)
		require.NoError(t, err)
		assert.Equal(t, domain.OrderStatusPending, order.Status)
		require.Len(t, order.Installments, 4)

		for i, inst := range order.Installments {
			assert.Equal(t, i+1, inst.Number)
			assert.Equal(t, int64(2500), inst.Amount, "each installment must be 2500 cents")
			assert.Equal(t, domain.InstallmentStatusPending, inst.Status)
		}

		// Check due dates: 0d, 14d, 28d, 42d
		assert.True(t, order.Installments[0].DueDate.Equal(now))
		assert.True(t, order.Installments[1].DueDate.Equal(now.Add(14*24*time.Hour)))
		assert.True(t, order.Installments[2].DueDate.Equal(now.Add(28*24*time.Hour)))
		assert.True(t, order.Installments[3].DueDate.Equal(now.Add(42*24*time.Hour)))
	})

	t.Run("non-divisible amount: $100.01 (10001 cents)", func(t *testing.T) {
		order, err := domain.NewOrder(uuid.New(), "client_1", custID, merchID, 10001, "USD", "", now)
		require.NoError(t, err)

		// 10001 / 4 = 2500 remainder 1 -> Cuota 1 gets 2501, rest get 2500
		assert.Equal(t, int64(2501), order.Installments[0].Amount)
		assert.Equal(t, int64(2500), order.Installments[1].Amount)
		assert.Equal(t, int64(2500), order.Installments[2].Amount)
		assert.Equal(t, int64(2500), order.Installments[3].Amount)

		var total int64
		for _, inst := range order.Installments {
			total += inst.Amount
		}
		assert.Equal(t, int64(10001), total)
	})

	t.Run("non-divisible amount: $100.03 (10003 cents)", func(t *testing.T) {
		order, err := domain.NewOrder(uuid.New(), "client_1", custID, merchID, 10003, "USD", "", now)
		require.NoError(t, err)

		// 10003 / 4 = 2500 remainder 3 -> Cuota 1 gets 2503, rest get 2500
		assert.Equal(t, int64(2503), order.Installments[0].Amount)
		assert.Equal(t, int64(2500), order.Installments[1].Amount)
		assert.Equal(t, int64(2500), order.Installments[2].Amount)
		assert.Equal(t, int64(2500), order.Installments[3].Amount)
	})
}

// TestProperty_InstallmentSumEqualsTotalAmount formally proves with rapid that for ANY arbitrary amount
// the sum of the 4 installments strictly equals the total amount.
func TestProperty_InstallmentSumEqualsTotalAmount(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		amount := rapid.Int64Range(1, 100_000_000_00).Draw(t, "totalAmountCents")
		order, err := domain.NewOrder(
			uuid.New(),
			"client_prop",
			uuid.New(),
			uuid.New(),
			amount,
			"USD",
			"",
			time.Now().UTC(),
		)
		require.NoError(t, err)
		require.Len(t, order.Installments, 4)

		var sum int64
		for _, inst := range order.Installments {
			require.True(t, inst.Amount >= 0)
			sum += inst.Amount
		}
		require.Equal(t, amount, sum, "installment sum does not equal total amount")
	})
}

func TestInstallment_RetryBackoffLifecycle(t *testing.T) {
	now := time.Now().UTC()
	inst := domain.Installment{
		ID:           uuid.New(),
		OrderID:      uuid.New(),
		Number:       2,
		Amount:       2500,
		Currency:     "USD",
		DueDate:      now,
		Status:       domain.InstallmentStatusPending,
		AttemptCount: 0,
		CreatedAt:    now,
	}

	assert.True(t, inst.CanPay())

	// 1st failure: attempt 1, retrying at +2h
	inst.MarkFailed(now)
	assert.Equal(t, domain.InstallmentStatusRetrying, inst.Status)
	assert.Equal(t, 1, inst.AttemptCount)
	require.NotNil(t, inst.NextRetryAt)
	assert.Equal(t, now.Add(2*time.Hour), *inst.NextRetryAt)
	assert.True(t, inst.CanPay())

	// 2nd failure: attempt 2, retrying at +12h
	inst.MarkFailed(now)
	assert.Equal(t, 2, inst.AttemptCount)
	assert.Equal(t, now.Add(12*time.Hour), *inst.NextRetryAt)

	// 3rd failure: attempt 3, retrying at +24h
	inst.MarkFailed(now)
	assert.Equal(t, 3, inst.AttemptCount)
	assert.Equal(t, now.Add(24*time.Hour), *inst.NextRetryAt)

	// 4th failure: max retries reached -> failed
	inst.MarkFailed(now)
	assert.Equal(t, domain.InstallmentStatusFailed, inst.Status)
	assert.Equal(t, 4, inst.AttemptCount)
	assert.Nil(t, inst.NextRetryAt)
	assert.False(t, inst.CanPay())
}

func TestOrder_StatusTransitions(t *testing.T) {
	now := time.Now().UTC()
	order, err := domain.NewOrder(uuid.New(), "client_1", uuid.New(), uuid.New(), 10000, "USD", "", now)
	require.NoError(t, err)

	assert.Equal(t, domain.OrderStatusPending, order.Status)

	// Mark active upon Cuota 1 payment
	require.NoError(t, order.MarkActive(now))
	assert.Equal(t, domain.OrderStatusActive, order.Status)

	// Not completed while installments pending
	assert.False(t, order.CheckCompletion(now))

	// Mark all installments paid
	for i := range order.Installments {
		order.Installments[i].MarkPaid(now)
	}

	assert.True(t, order.CheckCompletion(now))
	assert.Equal(t, domain.OrderStatusCompleted, order.Status)
}
