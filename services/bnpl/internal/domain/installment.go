package domain

import (
	"time"

	"github.com/google/uuid"
)

type InstallmentStatus string

const (
	InstallmentStatusPending    InstallmentStatus = "pending"
	InstallmentStatusProcessing InstallmentStatus = "processing"
	InstallmentStatusRetrying   InstallmentStatus = "retrying"
	InstallmentStatusPaid       InstallmentStatus = "paid"
	InstallmentStatusFailed     InstallmentStatus = "failed"
)

func (s InstallmentStatus) Valid() bool {
	switch s {
	case InstallmentStatusPending, InstallmentStatusProcessing, InstallmentStatusRetrying, InstallmentStatusPaid, InstallmentStatusFailed:
		return true
	default:
		return false
	}
}

// Installment represents one of the 4 bi-weekly payments of an order.
type Installment struct {
	ID           uuid.UUID         `json:"id"`
	OrderID      uuid.UUID         `json:"order_id"`
	Number       int               `json:"number"` // 1, 2, 3, or 4
	Amount       int64             `json:"amount"` // in cents
	Currency     string            `json:"currency"`
	DueDate      time.Time         `json:"due_date"`
	Status       InstallmentStatus `json:"status"`
	AttemptCount int               `json:"attempt_count"`
	NextRetryAt  *time.Time        `json:"next_retry_at,omitempty"`
	LockedAt     *time.Time        `json:"locked_at,omitempty"`
	PaidAt       *time.Time        `json:"paid_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// CanPay returns true if the installment is eligible for a payment attempt.
func (i *Installment) CanPay() bool {
	return i.Status == InstallmentStatusPending || i.Status == InstallmentStatusRetrying
}

// MarkPaid transitions the installment status to paid.
func (i *Installment) MarkPaid(now time.Time) {
	i.Status = InstallmentStatusPaid
	i.PaidAt = &now
	i.NextRetryAt = nil
	i.LockedAt = nil
}

// MarkFailed transitions the installment to retrying or failed with exponential backoff.
// Max attempts = 4 (1 initial + 3 retries).
func (i *Installment) MarkFailed(now time.Time) {
	i.AttemptCount++
	i.LockedAt = nil

	if i.AttemptCount >= 4 {
		i.Status = InstallmentStatusFailed
		i.NextRetryAt = nil
		return
	}

	i.Status = InstallmentStatusRetrying

	// Exponential backoff intervals:
	// Attempt 1 -> +2 hours
	// Attempt 2 -> +12 hours
	// Attempt 3 -> +24 hours
	var backoff time.Duration
	switch i.AttemptCount {
	case 1:
		backoff = 2 * time.Hour
	case 2:
		backoff = 12 * time.Hour
	case 3:
		backoff = 24 * time.Hour
	default:
		backoff = 24 * time.Hour
	}

	retryAt := now.Add(backoff)
	i.NextRetryAt = &retryAt
}
