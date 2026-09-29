package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/ledgerclient"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
)

var (
	ErrDownPaymentDeclined = errors.New("initial down payment was declined")
)

type CreateOrderRequest struct {
	ClientID           string    `json:"client_id"`
	CustomerAccountID  uuid.UUID `json:"customer_account_id"`
	MerchantAccountID  uuid.UUID `json:"merchant_account_id"`
	TotalAmount        int64     `json:"total_amount"`
	Currency           string    `json:"currency"`
	PaymentMethodToken string    `json:"payment_method_token"`
	MerchantWebhookURL string    `json:"merchant_webhook_url,omitempty"`
}

type OrderService struct {
	orderRepo          *postgres.OrderRepository
	attemptRepo        *postgres.PaymentAttemptRepository
	outboxRepo         *postgres.OutboxRepository
	gateway            gateway.PaymentGateway
	ledger             *ledgerclient.Client
	platformAccountID  uuid.UUID
	feesAccountID      uuid.UUID
	merchantFeePercent int64 // in basis points, e.g. 500 = 5%
}

func NewOrderService(
	orderRepo *postgres.OrderRepository,
	attemptRepo *postgres.PaymentAttemptRepository,
	outboxRepo *postgres.OutboxRepository,
	gw gateway.PaymentGateway,
	ledger *ledgerclient.Client,
	platformAccountID uuid.UUID,
	feesAccountID uuid.UUID,
	merchantFeePercent int64,
) *OrderService {
	if merchantFeePercent <= 0 {
		merchantFeePercent = 500 // 5% default
	}
	return &OrderService{
		orderRepo:          orderRepo,
		attemptRepo:        attemptRepo,
		outboxRepo:         outboxRepo,
		gateway:            gw,
		ledger:             ledger,
		platformAccountID:  platformAccountID,
		feesAccountID:      feesAccountID,
		merchantFeePercent: merchantFeePercent,
	}
}

// CreateOrder validates, splits into 4 installments, charges Cuota 1 down payment, and activates the order.
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*domain.Order, error) {
	now := time.Now().UTC()
	orderID := uuid.New()

	order, err := domain.NewOrder(
		orderID,
		req.ClientID,
		req.CustomerAccountID,
		req.MerchantAccountID,
		req.TotalAmount,
		req.Currency,
		req.MerchantWebhookURL,
		now,
	)
	if err != nil {
		return nil, err
	}

	downPayment := &order.Installments[0]

	// 1. Charge Cuota 1 via Payment Gateway
	chargeIdemKey := fmt.Sprintf("down_payment_order_%s", order.ID)
	chargeRes, err := s.gateway.Charge(ctx, gateway.ChargeRequest{
		IdempotencyKey:     chargeIdemKey,
		Amount:             downPayment.Amount,
		Currency:           downPayment.Currency,
		PaymentMethodToken: req.PaymentMethodToken,
	})
	if err != nil {
		return nil, fmt.Errorf("gateway error during down payment: %w", err)
	}

	if !chargeRes.Success {
		return nil, fmt.Errorf("%w: %s (%s)", ErrDownPaymentDeclined, chargeRes.ErrorMessage, chargeRes.ErrorCode)
	}

	// 2. Mark Down Payment Paid & Activate Order
	downPayment.MarkPaid(now)
	if err := order.MarkActive(now); err != nil {
		return nil, err
	}

	// 3. Persist Order and Installments to PostgreSQL
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	// 4. Record Payment Attempt
	_ = s.attemptRepo.Create(ctx, &domain.PaymentAttempt{
		ID:               uuid.New(),
		InstallmentID:    downPayment.ID,
		GatewayReference: chargeRes.GatewayReference,
		Result:           domain.PaymentResultSuccess,
		AttemptNumber:    1,
		CreatedAt:        now,
	})

	// 5. Record Ledger Transactions if client is configured
	if s.ledger != nil {
		// A. Loan origination: Debit Customer (Total), Credit Merchant (Net), Credit Fees (Fee)
		feeAmount := (order.TotalAmount * s.merchantFeePercent) / 10000
		merchantNet := order.TotalAmount - feeAmount

		originationEntries := []ledgerclient.EntryDTO{
			{AccountID: order.CustomerAccountID, Amount: order.TotalAmount, Direction: ledgerclient.DirectionDebit, Currency: order.Currency},
			{AccountID: order.MerchantAccountID, Amount: merchantNet, Direction: ledgerclient.DirectionCredit, Currency: order.Currency},
			{AccountID: s.feesAccountID, Amount: feeAmount, Direction: ledgerclient.DirectionCredit, Currency: order.Currency},
		}
		_, _ = s.ledger.RecordTransaction(ctx, req.ClientID, fmt.Sprintf("order_%s_origination", order.ID), "Loan Origination", originationEntries)

		// B. Down payment: Debit Platform Cash, Credit Customer Receivable
		if s.platformAccountID != uuid.Nil {
			downPaymentEntries := []ledgerclient.EntryDTO{
				{AccountID: s.platformAccountID, Amount: downPayment.Amount, Direction: ledgerclient.DirectionDebit, Currency: order.Currency},
				{AccountID: order.CustomerAccountID, Amount: downPayment.Amount, Direction: ledgerclient.DirectionCredit, Currency: order.Currency},
			}
			_, _ = s.ledger.RecordTransaction(ctx, req.ClientID, fmt.Sprintf("order_%s_down_payment", order.ID), "Down Payment", downPaymentEntries)
		}
	}

	// 6. Transactional Outbox Events
	_ = s.outboxRepo.SaveEvent(ctx, nil, "order.created", order.ID, order.MerchantWebhookURL, map[string]any{
		"order_id":     order.ID,
		"total_amount": order.TotalAmount,
		"currency":     order.Currency,
		"status":       order.Status,
		"created_at":   order.CreatedAt,
	})

	_ = s.outboxRepo.SaveEvent(ctx, nil, "installment.paid", downPayment.ID, order.MerchantWebhookURL, map[string]any{
		"installment_id": downPayment.ID,
		"order_id":       order.ID,
		"number":         1,
		"amount":         downPayment.Amount,
		"currency":       downPayment.Currency,
		"paid_at":        now,
	})

	return order, nil
}

// GetOrder retrieves an order by ID with all installments.
func (s *OrderService) GetOrder(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return s.orderRepo.GetByID(ctx, id)
}

// PayInstallment manually pays an installment.
func (s *OrderService) PayInstallment(ctx context.Context, installmentID uuid.UUID, cardToken string) (*domain.Installment, error) {
	inst, err := s.orderRepo.GetInstallmentByID(ctx, installmentID)
	if err != nil {
		return nil, err
	}

	if inst.Status == domain.InstallmentStatusPaid {
		return nil, domain.ErrInstallmentAlreadyPaid
	}
	if inst.Status == domain.InstallmentStatusProcessing {
		return nil, domain.ErrInstallmentPaymentInProgress
	}

	now := time.Now().UTC()
	attemptNum := inst.AttemptCount + 1
	idemKey := fmt.Sprintf("manual_inst_pay_%s_attempt_%d", inst.ID, attemptNum)

	chargeRes, err := s.gateway.Charge(ctx, gateway.ChargeRequest{
		IdempotencyKey:     idemKey,
		Amount:             inst.Amount,
		Currency:           inst.Currency,
		PaymentMethodToken: cardToken,
	})
	if err != nil {
		return nil, fmt.Errorf("gateway error: %w", err)
	}

	if !chargeRes.Success {
		inst.MarkFailed(now)
		_ = s.orderRepo.UpdateInstallment(ctx, inst)
		return nil, fmt.Errorf("payment declined: %s", chargeRes.ErrorMessage)
	}

	inst.MarkPaid(now)
	if err := s.orderRepo.UpdateInstallment(ctx, inst); err != nil {
		return nil, err
	}

	// Record payment attempt
	_ = s.attemptRepo.Create(ctx, &domain.PaymentAttempt{
		ID:               uuid.New(),
		InstallmentID:    inst.ID,
		GatewayReference: chargeRes.GatewayReference,
		Result:           domain.PaymentResultSuccess,
		AttemptNumber:    attemptNum,
		CreatedAt:        now,
	})

	order, err := s.orderRepo.GetByID(ctx, inst.OrderID)
	if err == nil {
		if order.CheckCompletion(now) {
			_ = s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusCompleted)
			_ = s.outboxRepo.SaveEvent(ctx, nil, "order.completed", order.ID, order.MerchantWebhookURL, map[string]any{
				"order_id": order.ID,
				"status":   domain.OrderStatusCompleted,
				"paid_at":  now,
			})
		}
		_ = s.outboxRepo.SaveEvent(ctx, nil, "installment.paid", inst.ID, order.MerchantWebhookURL, map[string]any{
			"installment_id": inst.ID,
			"order_id":       order.ID,
			"number":         inst.Number,
			"amount":         inst.Amount,
			"currency":       inst.Currency,
			"paid_at":        now,
		})
	}

	return inst, nil
}
