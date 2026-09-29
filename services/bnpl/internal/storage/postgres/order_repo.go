package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create persists an Order and all its 4 Installments in a single atomic transaction.
func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin order transaction: %w", err)
	}
	defer tx.Rollback()

	insertOrderQuery := `
		INSERT INTO orders (id, client_id, customer_account_id, merchant_account_id, total_amount, currency, status, merchant_webhook_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.ExecContext(ctx, insertOrderQuery,
		order.ID,
		order.ClientID,
		order.CustomerAccountID,
		order.MerchantAccountID,
		order.TotalAmount,
		order.Currency,
		string(order.Status),
		order.MerchantWebhookURL,
		order.CreatedAt,
		order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	insertInstQuery := `
		INSERT INTO installments (id, order_id, number, amount, currency, due_date, status, attempt_count, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	stmt, err := tx.PrepareContext(ctx, insertInstQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare installment statement: %w", err)
	}
	defer stmt.Close()

	for _, inst := range order.Installments {
		_, err := stmt.ExecContext(ctx,
			inst.ID,
			order.ID,
			inst.Number,
			inst.Amount,
			inst.Currency,
			inst.DueDate,
			string(inst.Status),
			inst.AttemptCount,
			inst.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert installment: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit order creation: %w", err)
	}

	return nil
}

// GetByID retrieves an Order and its 4 Installments by Order ID.
func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	queryOrder := `
		SELECT id, client_id, customer_account_id, merchant_account_id, total_amount, currency, status, merchant_webhook_url, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	var o domain.Order
	var status string
	var webhookURL sql.NullString

	err := r.db.QueryRowContext(ctx, queryOrder, id).Scan(
		&o.ID,
		&o.ClientID,
		&o.CustomerAccountID,
		&o.MerchantAccountID,
		&o.TotalAmount,
		&o.Currency,
		&status,
		&webhookURL,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}
	o.Status = domain.OrderStatus(status)
	if webhookURL.Valid {
		o.MerchantWebhookURL = webhookURL.String
	}

	// Fetch installments
	queryInst := `
		SELECT id, order_id, number, amount, currency, due_date, status, attempt_count, next_retry_at, locked_at, paid_at, created_at
		FROM installments
		WHERE order_id = $1
		ORDER BY number ASC
	`
	rows, err := r.db.QueryContext(ctx, queryInst, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query installments: %w", err)
	}
	defer rows.Close()

	var installments []domain.Installment
	for rows.Next() {
		var inst domain.Installment
		var instStatus string
		if err := rows.Scan(
			&inst.ID,
			&inst.OrderID,
			&inst.Number,
			&inst.Amount,
			&inst.Currency,
			&inst.DueDate,
			&instStatus,
			&inst.AttemptCount,
			&inst.NextRetryAt,
			&inst.LockedAt,
			&inst.PaidAt,
			&inst.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan installment: %w", err)
		}
		inst.Status = domain.InstallmentStatus(instStatus)
		installments = append(installments, inst)
	}
	o.Installments = installments

	return &o, nil
}

// UpdateOrderStatus updates an order's status and updated_at.
func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	query := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	res, err := r.db.ExecContext(ctx, query, string(status), id)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domain.ErrOrderNotFound
	}
	return nil
}

// GetInstallmentByID retrieves an installment by its ID.
func (r *OrderRepository) GetInstallmentByID(ctx context.Context, id uuid.UUID) (*domain.Installment, error) {
	query := `
		SELECT id, order_id, number, amount, currency, due_date, status, attempt_count, next_retry_at, locked_at, paid_at, created_at
		FROM installments
		WHERE id = $1
	`
	var inst domain.Installment
	var status string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&inst.ID,
		&inst.OrderID,
		&inst.Number,
		&inst.Amount,
		&inst.Currency,
		&inst.DueDate,
		&status,
		&inst.AttemptCount,
		&inst.NextRetryAt,
		&inst.LockedAt,
		&inst.PaidAt,
		&inst.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrInstallmentNotFound
		}
		return nil, fmt.Errorf("failed to query installment: %w", err)
	}
	inst.Status = domain.InstallmentStatus(status)
	return &inst, nil
}

// UpdateInstallment updates mutable fields of an installment.
func (r *OrderRepository) UpdateInstallment(ctx context.Context, inst *domain.Installment) error {
	query := `
		UPDATE installments
		SET status = $1, attempt_count = $2, next_retry_at = $3, locked_at = $4, paid_at = $5
		WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query,
		string(inst.Status),
		inst.AttemptCount,
		inst.NextRetryAt,
		inst.LockedAt,
		inst.PaidAt,
		inst.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update installment: %w", err)
	}
	return nil
}

// LeaseDueInstallments locks and leases due installments using SELECT ... FOR UPDATE SKIP LOCKED.
// Immediately transitions their status to 'processing' to prevent concurrent worker picking.
func (r *OrderRepository) LeaseDueInstallments(ctx context.Context, limit int, now time.Time) ([]domain.Installment, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("failed to begin lease transaction: %w", err)
	}
	defer tx.Rollback()

	querySelect := `
		SELECT id, order_id, number, amount, currency, due_date, status, attempt_count, next_retry_at, locked_at, paid_at, created_at
		FROM installments
		WHERE status IN ('pending', 'retrying')
		  AND due_date <= $1
		  AND (next_retry_at IS NULL OR next_retry_at <= $1)
		ORDER BY due_date ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED;
	`
	rows, err := tx.QueryContext(ctx, querySelect, now, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to select due installments: %w", err)
	}
	defer rows.Close()

	var leased []domain.Installment
	for rows.Next() {
		var inst domain.Installment
		var status string
		if err := rows.Scan(
			&inst.ID,
			&inst.OrderID,
			&inst.Number,
			&inst.Amount,
			&inst.Currency,
			&inst.DueDate,
			&status,
			&inst.AttemptCount,
			&inst.NextRetryAt,
			&inst.LockedAt,
			&inst.PaidAt,
			&inst.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan leased installment: %w", err)
		}
		inst.Status = domain.InstallmentStatus(status)
		leased = append(leased, inst)
	}

	if len(leased) == 0 {
		return nil, nil
	}

	// Update leased rows to processing
	queryUpdate := `
		UPDATE installments
		SET status = 'processing', locked_at = $1
		WHERE id = ANY($2);
	`
	leasedIDs := make([]uuid.UUID, len(leased))
	for i, inst := range leased {
		leasedIDs[i] = inst.ID
		leased[i].Status = domain.InstallmentStatusProcessing
		leased[i].LockedAt = &now
	}

	if _, err := tx.ExecContext(ctx, queryUpdate, now, leasedIDs); err != nil {
		return nil, fmt.Errorf("failed to mark leased installments as processing: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit lease transaction: %w", err)
	}

	return leased, nil
}
