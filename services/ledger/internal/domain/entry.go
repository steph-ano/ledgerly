package domain

import (
	"time"

	"github.com/google/uuid"
)

type Direction string

const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

// Valid returns true if direction is either DEBIT or CREDIT.
func (d Direction) Valid() bool {
	return d == DirectionDebit || d == DirectionCredit
}

// Reverse returns the opposite entry direction.
func (d Direction) Reverse() Direction {
	if d == DirectionDebit {
		return DirectionCredit
	}
	return DirectionDebit
}

// Entry represents an immutable atomic double-entry line in the ledger.
type Entry struct {
	ID            uuid.UUID `json:"id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	AccountID     uuid.UUID `json:"account_id"`
	Amount        int64     `json:"amount"`
	Direction     Direction `json:"direction"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}

// NewEntry creates and validates a single ledger Entry.
func NewEntry(
	id uuid.UUID,
	transactionID uuid.UUID,
	accountID uuid.UUID,
	amount int64,
	direction Direction,
	currency string,
	createdAt time.Time,
) (*Entry, error) {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if transactionID == uuid.Nil {
		return nil, ErrInvalidTransactionID
	}
	if accountID == uuid.Nil {
		return nil, ErrInvalidAccountID
	}
	if amount <= 0 {
		return nil, ErrAmountNonPositive
	}
	if !direction.Valid() {
		return nil, ErrInvalidEntryDirection
	}
	if !currencyRegex.MatchString(currency) {
		return nil, ErrInvalidCurrency
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return &Entry{
		ID:            id,
		TransactionID: transactionID,
		AccountID:     accountID,
		Amount:        amount,
		Direction:     direction,
		Currency:      currency,
		CreatedAt:     createdAt,
	}, nil
}
