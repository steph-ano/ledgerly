package domain

import (
	"time"

	"github.com/google/uuid"
)

type PaymentResult string

const (
	PaymentResultSuccess  PaymentResult = "success"
	PaymentResultDeclined PaymentResult = "declined"
	PaymentResultError    PaymentResult = "error"
)

// PaymentAttempt records an interaction with the payment gateway for an installment.
type PaymentAttempt struct {
	ID               uuid.UUID     `json:"id"`
	InstallmentID    uuid.UUID     `json:"installment_id"`
	GatewayReference string        `json:"gateway_reference"`
	Result           PaymentResult `json:"result"`
	ErrorCode        string        `json:"error_code,omitempty"`
	ErrorMessage     string        `json:"error_message,omitempty"`
	AttemptNumber    int           `json:"attempt_number"`
	CreatedAt        time.Time     `json:"created_at"`
}
