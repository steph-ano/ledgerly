package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/gateway"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/ledgerclient"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
)

type Config struct {
	PollInterval       time.Duration
	BatchSize          int
	PlatformAccountID  uuid.UUID // Platform cash account in the ledger
	DefaultPaymentCard string
}

func DefaultConfig() Config {
	return Config{
		PollInterval:       5 * time.Second,
		BatchSize:          25,
		DefaultPaymentCard: "pm_card_visa",
	}
}

type InstallmentScheduler struct {
	cfg         Config
	orderRepo   *postgres.OrderRepository
	attemptRepo *postgres.PaymentAttemptRepository
	outboxRepo  *postgres.OutboxRepository
	gateway     gateway.PaymentGateway
	ledger      *ledgerclient.Client
}

func NewInstallmentScheduler(
	cfg Config,
	orderRepo *postgres.OrderRepository,
	attemptRepo *postgres.PaymentAttemptRepository,
	outboxRepo *postgres.OutboxRepository,
	gw gateway.PaymentGateway,
	ledger *ledgerclient.Client,
) *InstallmentScheduler {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.DefaultPaymentCard == "" {
		cfg.DefaultPaymentCard = "pm_card_visa"
	}
	return &InstallmentScheduler{
		cfg:         cfg,
		orderRepo:   orderRepo,
		attemptRepo: attemptRepo,
		outboxRepo:  outboxRepo,
		gateway:     gw,
		ledger:      ledger,
	}
}

// ProcessDueInstallments executes one round of processing due installments.
func (s *InstallmentScheduler) ProcessDueInstallments(ctx context.Context, now time.Time) (int, error) {
	leased, err := s.orderRepo.LeaseDueInstallments(ctx, s.cfg.BatchSize, now)
	if err != nil {
		return 0, fmt.Errorf("failed to lease due installments: %w", err)
	}

	if len(leased) == 0 {
		return 0, nil
	}

	for _, inst := range leased {
		s.processSingleInstallment(ctx, inst, now)
	}

	return len(leased), nil
}

func (s *InstallmentScheduler) processSingleInstallment(ctx context.Context, inst domain.Installment, now time.Time) {
	attemptNum := inst.AttemptCount + 1
	idemKey := fmt.Sprintf("bnpl_sched_inst_%s_attempt_%d", inst.ID, attemptNum)

	order, err := s.orderRepo.GetByID(ctx, inst.OrderID)
	if err != nil {
		slog.Error("failed to load order for installment", "order_id", inst.OrderID, "error", err)
		return
	}

	// 1. Charge Gateway
	chargeReq := gateway.ChargeRequest{
		IdempotencyKey:     idemKey,
		Amount:             inst.Amount,
		Currency:           inst.Currency,
		PaymentMethodToken: s.cfg.DefaultPaymentCard,
	}

	chargeRes, err := s.gateway.Charge(ctx, chargeReq)

	// Gateway transient network error
	if err != nil {
		slog.Warn("gateway transient error during installment payment", "installment_id", inst.ID, "error", err)
		_ = s.attemptRepo.Create(ctx, &domain.PaymentAttempt{
			ID:               uuid.New(),
			InstallmentID:    inst.ID,
			GatewayReference: "net_err",
			Result:           domain.PaymentResultError,
			ErrorCode:        "transient_error",
			ErrorMessage:     err.Error(),
			AttemptNumber:    attemptNum,
			CreatedAt:        now,
		})

		inst.MarkFailed(now)
		_ = s.orderRepo.UpdateInstallment(ctx, &inst)
		return
	}

	// 2. Evaluate Gateway Response
	if chargeRes.Success {
		// Log success attempt
		_ = s.attemptRepo.Create(ctx, &domain.PaymentAttempt{
			ID:               uuid.New(),
			InstallmentID:    inst.ID,
			GatewayReference: chargeRes.GatewayReference,
			Result:           domain.PaymentResultSuccess,
			AttemptNumber:    attemptNum,
			CreatedAt:        now,
		})

		// Record in Ledger if ledger client is configured
		if s.ledger != nil && s.cfg.PlatformAccountID != uuid.Nil {
			ledgerEntries := []ledgerclient.EntryDTO{
				{AccountID: s.cfg.PlatformAccountID, Amount: inst.Amount, Direction: ledgerclient.DirectionDebit, Currency: inst.Currency},
				{AccountID: order.CustomerAccountID, Amount: inst.Amount, Direction: ledgerclient.DirectionCredit, Currency: inst.Currency},
			}
			ledgerIdemKey := fmt.Sprintf("ledger_tx_inst_%s_paid", inst.ID)
			_, err = s.ledger.RecordTransaction(
				ctx,
				order.ClientID,
				ledgerIdemKey,
				fmt.Sprintf("Payment for Installment %d of Order %s", inst.Number, order.ID),
				ledgerEntries,
			)
			if err != nil {
				slog.Error("failed to record installment payment in ledger", "installment_id", inst.ID, "error", err)
			}
		}

		// Update installment to paid
		inst.MarkPaid(now)
		_ = s.orderRepo.UpdateInstallment(ctx, &inst)

		// Check if entire order is completed
		order.Installments[inst.Number-1] = inst
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

		slog.Info("installment successfully paid", "installment_id", inst.ID, "number", inst.Number)
	} else {
		// Log decline attempt
		_ = s.attemptRepo.Create(ctx, &domain.PaymentAttempt{
			ID:               uuid.New(),
			InstallmentID:    inst.ID,
			GatewayReference: chargeRes.GatewayReference,
			Result:           domain.PaymentResultDeclined,
			ErrorCode:        chargeRes.ErrorCode,
			ErrorMessage:     chargeRes.ErrorMessage,
			AttemptNumber:    attemptNum,
			CreatedAt:        now,
		})

		inst.MarkFailed(now)
		_ = s.orderRepo.UpdateInstallment(ctx, &inst)

		if inst.Status == domain.InstallmentStatusFailed {
			_ = s.orderRepo.UpdateOrderStatus(ctx, order.ID, domain.OrderStatusDefaulted)
			_ = s.outboxRepo.SaveEvent(ctx, nil, "order.defaulted", order.ID, order.MerchantWebhookURL, map[string]any{
				"order_id":       order.ID,
				"status":         domain.OrderStatusDefaulted,
				"failed_inst_id": inst.ID,
				"defaulted_at":   now,
			})
		}

		_ = s.outboxRepo.SaveEvent(ctx, nil, "installment.failed", inst.ID, order.MerchantWebhookURL, map[string]any{
			"installment_id": inst.ID,
			"order_id":       order.ID,
			"number":         inst.Number,
			"attempt_count":  inst.AttemptCount,
			"error_code":     chargeRes.ErrorCode,
		})

		slog.Warn("installment payment declined",
			"installment_id", inst.ID,
			"number", inst.Number,
			"attempt_count", inst.AttemptCount,
			"status", inst.Status,
		)
	}
}

// Start runs the periodic background scheduler loop until ctx is cancelled.
func (s *InstallmentScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			count, err := s.ProcessDueInstallments(ctx, now)
			if err != nil {
				slog.Error("error during installment processing loop", "error", err)
			} else if count > 0 {
				slog.Info("processed due installments batch", "count", count)
			}
		}
	}
}
