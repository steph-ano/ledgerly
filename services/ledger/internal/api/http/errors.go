package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL_SERVER_ERROR"

	switch {
	case errors.Is(err, domain.ErrIdempotencyKeyPayloadMismatch):
		status = http.StatusConflict
		code = "IDEMPOTENCY_PAYLOAD_MISMATCH"

	case errors.Is(err, domain.ErrInsufficientFunds):
		status = http.StatusUnprocessableEntity
		code = "INSUFFICIENT_FUNDS"

	case errors.Is(err, domain.ErrInsufficientFundsForReversal):
		status = http.StatusUnprocessableEntity
		code = "INSUFFICIENT_FUNDS_FOR_REVERSAL"

	case errors.Is(err, domain.ErrNegativeBalanceNotAllowed):
		status = http.StatusUnprocessableEntity
		code = "NEGATIVE_BALANCE_NOT_ALLOWED"

	case errors.Is(err, domain.ErrPlatformLiquidityBreached):
		status = http.StatusUnprocessableEntity
		code = "PLATFORM_LIQUIDITY_BREACHED"

	case errors.Is(err, domain.ErrAlreadyReversed):
		status = http.StatusConflict
		code = "TRANSACTION_ALREADY_REVERSED"

	case errors.Is(err, domain.ErrCannotReverseReversal):
		status = http.StatusBadRequest
		code = "CANNOT_REVERSE_REVERSAL"

	case errors.Is(err, domain.ErrAccountNotFound):
		status = http.StatusNotFound
		code = "ACCOUNT_NOT_FOUND"

	case errors.Is(err, domain.ErrTransactionNotFound):
		status = http.StatusNotFound
		code = "TRANSACTION_NOT_FOUND"

	case errors.Is(err, domain.ErrUnbalancedTransaction),
		errors.Is(err, domain.ErrInsufficientEntries),
		errors.Is(err, domain.ErrMultipleCurrencies),
		errors.Is(err, domain.ErrInvalidCurrency),
		errors.Is(err, domain.ErrCurrencyMismatch),
		errors.Is(err, domain.ErrAmountNonPositive),
		errors.Is(err, domain.ErrAmountOverflow),
		errors.Is(err, domain.ErrEmptyClientID),
		errors.Is(err, domain.ErrEmptyIdempotencyKey),
		errors.Is(err, domain.ErrEmptyDescription),
		errors.Is(err, domain.ErrInvalidAccountType),
		errors.Is(err, domain.ErrInvalidAccountID),
		errors.Is(err, domain.ErrInvalidTransactionID),
		errors.Is(err, domain.ErrInvalidEntryDirection):
		status = http.StatusBadRequest
		code = "BAD_REQUEST"
	}

	writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: err.Error(),
		},
	})
}
