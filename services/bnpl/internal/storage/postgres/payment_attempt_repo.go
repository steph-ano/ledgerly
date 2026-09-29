package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/domain"
)

type PaymentAttemptRepository struct {
	db *sql.DB
}

func NewPaymentAttemptRepository(db *sql.DB) *PaymentAttemptRepository {
	return &PaymentAttemptRepository{db: db}
}

// Create logs a payment gateway attempt.
func (r *PaymentAttemptRepository) Create(ctx context.Context, attempt *domain.PaymentAttempt) error {
	query := `
		INSERT INTO payment_attempts (id, installment_id, gateway_reference, result, error_code, error_message, attempt_number, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		attempt.ID,
		attempt.InstallmentID,
		attempt.GatewayReference,
		string(attempt.Result),
		attempt.ErrorCode,
		attempt.ErrorMessage,
		attempt.AttemptNumber,
		attempt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record payment attempt: %w", err)
	}
	return nil
}

// ListByInstallment returns all attempts made for a given installment.
func (r *PaymentAttemptRepository) ListByInstallment(ctx context.Context, installmentID uuid.UUID) ([]domain.PaymentAttempt, error) {
	query := `
		SELECT id, installment_id, gateway_reference, result, error_code, error_message, attempt_number, created_at
		FROM payment_attempts
		WHERE installment_id = $1
		ORDER BY attempt_number ASC
	`
	rows, err := r.db.QueryContext(ctx, query, installmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment attempts: %w", err)
	}
	defer rows.Close()

	var attempts []domain.PaymentAttempt
	for rows.Next() {
		var a domain.PaymentAttempt
		var res string
		var errCode, errMsg sql.NullString
		if err := rows.Scan(
			&a.ID,
			&a.InstallmentID,
			&a.GatewayReference,
			&res,
			&errCode,
			&errMsg,
			&a.AttemptNumber,
			&a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payment attempt: %w", err)
		}
		a.Result = domain.PaymentResult(res)
		if errCode.Valid {
			a.ErrorCode = errCode.String
		}
		if errMsg.Valid {
			a.ErrorMessage = errMsg.String
		}
		attempts = append(attempts, a)
	}
	return attempts, nil
}
