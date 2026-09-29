package domain

import "errors"

var (
	// Money errors
	ErrAmountOverflow     = errors.New("monetary amount overflow")
	ErrAmountNonPositive  = errors.New("monetary amount must be strictly positive")
	ErrInvalidCurrency    = errors.New("invalid currency code: must be 3 uppercase letters (ISO 4217)")
	ErrCurrencyMismatch   = errors.New("currency mismatch between operations or entities")

	// Account errors
	ErrInvalidAccountType          = errors.New("invalid account type")
	ErrInvalidAccountID            = errors.New("invalid account id")
	ErrEmptyClientID               = errors.New("client id cannot be empty")
	ErrNegativeBalanceNotAllowed   = errors.New("account balance cannot be negative under account policy")
	ErrInsufficientFunds           = errors.New("insufficient funds to execute transaction")
	ErrInsufficientFundsForReversal = errors.New("insufficient funds to execute reversal transaction")
	ErrPlatformLiquidityBreached   = errors.New("platform operational liquidity floor breached")

	// Transaction & Entry errors
	ErrInvalidTransactionID   = errors.New("invalid transaction id")
	ErrEmptyIdempotencyKey     = errors.New("idempotency key cannot be empty")
	ErrInvalidRequestHash      = errors.New("request hash must be a 64-character hex string")
	ErrEmptyDescription        = errors.New("transaction description cannot be empty")
	ErrInsufficientEntries     = errors.New("transaction must contain at least 2 entries")
	ErrInvalidEntryDirection   = errors.New("invalid entry direction: must be DEBIT or CREDIT")
	ErrUnbalancedTransaction   = errors.New("transaction is unbalanced: sum of debits must equal sum of credits")
	ErrMultipleCurrencies      = errors.New("transaction entries must all have the same currency")
	ErrAccountCurrencyMismatch = errors.New("entry currency does not match account currency")
	ErrAlreadyReversed         = errors.New("transaction has already been reversed")
	ErrCannotReverseReversal   = errors.New("cannot reverse a transaction that is itself a reversal")
	ErrSameAccountTransfer     = errors.New("cannot create balanced transaction between the exact same account")
)
