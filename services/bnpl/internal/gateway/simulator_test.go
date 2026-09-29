package gateway_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
)

func TestSimulator_Charge(t *testing.T) {
	ctx := context.Background()
	sim := gateway.NewSimulator()

	t.Run("successful charge", func(t *testing.T) {
		req := gateway.ChargeRequest{
			IdempotencyKey:     "test_key_1",
			Amount:             2500,
			Currency:           "USD",
			PaymentMethodToken: "pm_card_visa",
		}
		res, err := sim.Charge(ctx, req)
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.NotEmpty(t, res.GatewayReference)
	})

	t.Run("idempotent replay returns identical reference", func(t *testing.T) {
		req := gateway.ChargeRequest{
			IdempotencyKey:     "test_key_idem",
			Amount:             5000,
			Currency:           "USD",
			PaymentMethodToken: "pm_card_visa",
		}
		res1, err := sim.Charge(ctx, req)
		require.NoError(t, err)

		// Second charge with same key
		res2, err := sim.Charge(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, res1.GatewayReference, res2.GatewayReference)
	})

	t.Run("declined insufficient funds", func(t *testing.T) {
		req := gateway.ChargeRequest{
			IdempotencyKey:     "test_key_decline",
			Amount:             2500,
			Currency:           "USD",
			PaymentMethodToken: "pm_card_insufficient_funds",
		}
		res, err := sim.Charge(ctx, req)
		require.NoError(t, err)
		assert.False(t, res.Success)
		assert.Equal(t, "insufficient_funds", res.ErrorCode)
	})

	t.Run("transient timeout simulation", func(t *testing.T) {
		req := gateway.ChargeRequest{
			IdempotencyKey:     "test_key_timeout",
			Amount:             2500,
			Currency:           "USD",
			PaymentMethodToken: "pm_card_timeout",
		}
		_, err := sim.Charge(ctx, req)
		assert.ErrorIs(t, err, gateway.ErrGatewayTimeout)
	})
}
