package postgres_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/scheduler"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/webhook"
)

func TestIntegration_OrderCreationAndInstallments(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	orderRepo := postgres.NewOrderRepository(db)
	ctx := context.Background()

	orderID := uuid.New()
	custID := uuid.New()
	merchID := uuid.New()
	now := time.Now().UTC()

	order, err := domain.NewOrder(orderID, "merchant_app", custID, merchID, 10001, "USD", "https://merchant.example.com/webhook", now)
	require.NoError(t, err)

	err = orderRepo.Create(ctx, order)
	require.NoError(t, err)

	retrieved, err := orderRepo.GetByID(ctx, orderID)
	require.NoError(t, err)
	assert.Equal(t, orderID, retrieved.ID)
	assert.Equal(t, int64(10001), retrieved.TotalAmount)
	require.Len(t, retrieved.Installments, 4)

	// Verify Cuota 1 remainder assignment: 2501, 2500, 2500, 2500
	assert.Equal(t, int64(2501), retrieved.Installments[0].Amount)
	assert.Equal(t, int64(2500), retrieved.Installments[1].Amount)
	assert.Equal(t, int64(2500), retrieved.Installments[2].Amount)
	assert.Equal(t, int64(2500), retrieved.Installments[3].Amount)
}

func TestIntegration_LeaseDueInstallmentsSkipLocked(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	orderRepo := postgres.NewOrderRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()

	// Create 5 orders, each having installments
	var orderIDs []uuid.UUID
	for i := 0; i < 5; i++ {
		o, err := domain.NewOrder(uuid.New(), "client_skip", uuid.New(), uuid.New(), 10000, "USD", "", now)
		require.NoError(t, err)
		require.NoError(t, orderRepo.Create(ctx, o))
		orderIDs = append(orderIDs, o.ID)
	}

	// 2 parallel workers leasing installments concurrently
	var wg sync.WaitGroup
	var worker1Leased, worker2Leased []domain.Installment
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		worker1Leased, err1 = orderRepo.LeaseDueInstallments(ctx, 10, now.Add(50*24*time.Hour))
	}()

	go func() {
		defer wg.Done()
		worker2Leased, err2 = orderRepo.LeaseDueInstallments(ctx, 10, now.Add(50*24*time.Hour))
	}()

	wg.Wait()
	require.NoError(t, err1)
	require.NoError(t, err2)

	// Verify that workers leased completely mutually exclusive installments (zero overlap!)
	leasedSet := make(map[uuid.UUID]bool)
	for _, inst := range worker1Leased {
		assert.False(t, leasedSet[inst.ID], "worker1 received duplicated installment")
		leasedSet[inst.ID] = true
	}
	for _, inst := range worker2Leased {
		assert.False(t, leasedSet[inst.ID], "worker2 received installment already leased by worker1")
		leasedSet[inst.ID] = true
	}

	totalLeased := len(worker1Leased) + len(worker2Leased)
	assert.True(t, totalLeased > 0)
}

func TestIntegration_SchedulerAutomatedPayment(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	orderRepo := postgres.NewOrderRepository(db)
	attemptRepo := postgres.NewPaymentAttemptRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)
	gw := gateway.NewSimulator()
	ctx := context.Background()

	now := time.Now().UTC()
	order, err := domain.NewOrder(uuid.New(), "client_sched", uuid.New(), uuid.New(), 10000, "USD", "https://mock.com/hook", now)
	require.NoError(t, err)

	// Cuota 1 was paid at checkout
	order.Installments[0].MarkPaid(now)
	order.Status = domain.OrderStatusActive
	require.NoError(t, orderRepo.Create(ctx, order))

	// Setup scheduler
	sched := scheduler.NewInstallmentScheduler(
		scheduler.Config{
			BatchSize:          10,
			DefaultPaymentCard: "pm_card_visa",
		},
		orderRepo,
		attemptRepo,
		outboxRepo,
		gw,
		nil, // no external ledger in this unit integration test
	)

	// Advance time to 15 days (Cuota 2 is due at day 14)
	day15 := now.Add(15 * 24 * time.Hour)
	count, err := sched.ProcessDueInstallments(ctx, day15)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should process exactly 1 due installment (Cuota 2)")

	// Verify Cuota 2 is now paid
	inst2, err := orderRepo.GetInstallmentByID(ctx, order.Installments[1].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusPaid, inst2.Status)
	assert.NotNil(t, inst2.PaidAt)

	// Verify payment attempt was recorded
	attempts, err := attemptRepo.ListByInstallment(ctx, inst2.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	assert.Equal(t, domain.PaymentResultSuccess, attempts[0].Result)

	// Verify outbox event was recorded
	outboxEvents, err := outboxRepo.LeasePendingEvents(ctx, 10, day15)
	require.NoError(t, err)
	require.NotEmpty(t, outboxEvents)
	assert.Equal(t, "installment.paid", outboxEvents[0].EventType)
}

func TestIntegration_SchedulerExponentialBackoffAndDefault(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	orderRepo := postgres.NewOrderRepository(db)
	attemptRepo := postgres.NewPaymentAttemptRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)
	gw := gateway.NewSimulator()
	ctx := context.Background()

	now := time.Now().UTC()
	order, err := domain.NewOrder(uuid.New(), "client_backoff", uuid.New(), uuid.New(), 10000, "USD", "https://merchant.com/hook", now)
	require.NoError(t, err)
	order.Installments[0].MarkPaid(now)
	order.Status = domain.OrderStatusActive
	require.NoError(t, orderRepo.Create(ctx, order))

	// Scheduler configured with failing card (insufficient funds)
	sched := scheduler.NewInstallmentScheduler(
		scheduler.Config{
			BatchSize:          10,
			DefaultPaymentCard: "pm_card_insufficient_funds",
		},
		orderRepo,
		attemptRepo,
		outboxRepo,
		gw,
		nil,
	)

	// 1st attempt at day 14
	currentTime := now.Add(14 * 24 * time.Hour)
	count, err := sched.ProcessDueInstallments(ctx, currentTime)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	inst, err := orderRepo.GetInstallmentByID(ctx, order.Installments[1].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusRetrying, inst.Status)
	assert.Equal(t, 1, inst.AttemptCount)
	require.NotNil(t, inst.NextRetryAt)
	assert.WithinDuration(t, currentTime.Add(2*time.Hour), *inst.NextRetryAt, time.Second)

	// 2nd attempt at currentTime + 3h (retry due at +2h)
	currentTime = currentTime.Add(3 * time.Hour)
	_, err = sched.ProcessDueInstallments(ctx, currentTime)
	require.NoError(t, err)

	inst, err = orderRepo.GetInstallmentByID(ctx, order.Installments[1].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusRetrying, inst.Status)
	assert.Equal(t, 2, inst.AttemptCount)
	require.NotNil(t, inst.NextRetryAt)
	assert.WithinDuration(t, currentTime.Add(12*time.Hour), *inst.NextRetryAt, time.Second)

	// 3rd attempt at currentTime + 13h
	currentTime = currentTime.Add(13 * time.Hour)
	_, err = sched.ProcessDueInstallments(ctx, currentTime)
	require.NoError(t, err)

	inst, err = orderRepo.GetInstallmentByID(ctx, order.Installments[1].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusRetrying, inst.Status)
	assert.Equal(t, 3, inst.AttemptCount)
	require.NotNil(t, inst.NextRetryAt)
	assert.WithinDuration(t, currentTime.Add(24*time.Hour), *inst.NextRetryAt, time.Second)

	// 4th attempt at currentTime + 25h: Max retries exhausted!
	currentTime = currentTime.Add(25 * time.Hour)
	_, err = sched.ProcessDueInstallments(ctx, currentTime)
	require.NoError(t, err)

	inst, err = orderRepo.GetInstallmentByID(ctx, order.Installments[1].ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusFailed, inst.Status, "must transition to failed on 4th attempt")
	assert.Equal(t, 4, inst.AttemptCount)
	assert.Nil(t, inst.NextRetryAt)

	// Verify order was transitioned to defaulted!
	retrievedOrder, err := orderRepo.GetByID(ctx, order.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderStatusDefaulted, retrievedOrder.Status)
}

func TestIntegration_WebhookDispatcherHMAC(t *testing.T) {
	db, teardown := setupTestDB(t)
	defer teardown()

	outboxRepo := postgres.NewOutboxRepository(db)
	ctx := context.Background()

	var receivedHeaders http.Header
	var receivedBody []byte

	// Mock merchant webhook server
	mockMerchantServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received": true}`))
	}))
	defer mockMerchantServer.Close()

	secretKey := "test_secret_key_123"
	dispatcher := webhook.NewDispatcher(webhook.Config{
		BatchSize:  10,
		SigningKey: secretKey,
	}, outboxRepo)

	eventID := uuid.New()
	eventPayload := map[string]any{
		"order_id": eventID.String(),
		"status":   "completed",
	}

	err := outboxRepo.SaveEvent(ctx, nil, "order.completed", eventID, mockMerchantServer.URL, eventPayload)
	require.NoError(t, err)

	now := time.Now().UTC()
	dispatchedCount, err := dispatcher.ProcessPendingEvents(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, 1, dispatchedCount)

	// Verify webhook headers
	assert.Equal(t, "order.completed", receivedHeaders.Get("X-Ledgerly-Event-Type"))
	assert.NotEmpty(t, receivedHeaders.Get("X-Ledgerly-Signature"))
	assert.NotEmpty(t, receivedHeaders.Get("X-Ledgerly-Timestamp"))
	assert.Contains(t, string(receivedBody), "order_id")

	// Verify event was marked delivered in outbox
	pending, err := outboxRepo.LeasePendingEvents(ctx, 10, now)
	require.NoError(t, err)
	assert.Empty(t, pending, "event must be marked delivered and no longer pending")
}
