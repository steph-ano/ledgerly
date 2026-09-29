package domain

import "errors"

var (
	ErrInvalidAmount                 = errors.New("order total amount must be strictly positive")
	ErrInvalidCurrency               = errors.New("invalid currency code: must be 3 uppercase letters (ISO 4217)")
	ErrEmptyClientID                 = errors.New("client id cannot be empty")
	ErrInvalidAccountID              = errors.New("account id cannot be nil")
	ErrOrderNotFound                 = errors.New("order not found")
	ErrInstallmentNotFound           = errors.New("installment not found")
	ErrOrderAlreadyCompleted         = errors.New("order is already completed")
	ErrInstallmentAlreadyPaid         = errors.New("installment is already paid")
	ErrInstallmentPaymentInProgress   = errors.New("payment for this installment is currently in progress")
	ErrMaxRetriesExceeded             = errors.New("maximum payment retry attempts exceeded")
	ErrInvalidOrderStatusTransition  = errors.New("invalid order status transition")
)
