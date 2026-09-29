package domain

import (
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// Transaction represents an atomic group of balanced ledger entries.
type Transaction struct {
	ID             uuid.UUID  `json:"id"`
	ClientID       string     `json:"client_id"`
	IdempotencyKey string     `json:"idempotency_key"`
	RequestHash    string     `json:"request_hash"`
	Description    string     `json:"description"`
	ReversalOf     *uuid.UUID `json:"reversal_of,omitempty"`
	Entries        []Entry    `json:"entries"`
	CreatedAt      time.Time  `json:"created_at"`
}

// NewTransaction validates and builds a balanced Transaction entity.
func NewTransaction(
	id uuid.UUID,
	clientID string,
	idempotencyKey string,
	requestHash string,
	description string,
	entries []Entry,
	createdAt time.Time,
) (*Transaction, error) {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if clientID == "" {
		return nil, ErrEmptyClientID
	}
	if idempotencyKey == "" {
		return nil, ErrEmptyIdempotencyKey
	}
	if len(requestHash) != 64 {
		return nil, ErrInvalidRequestHash
	}
	if _, err := hex.DecodeString(requestHash); err != nil {
		return nil, ErrInvalidRequestHash
	}
	if description == "" {
		return nil, ErrEmptyDescription
	}
	if len(entries) < 2 {
		return nil, ErrInsufficientEntries
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	// Invariant validation:
	// 1. Single currency across all entries
	// 2. sum(Debits) - sum(Credits) == 0 (with overflow-safe addition)
	// 3. Positive amounts and valid directions
	currency := entries[0].Currency
	if !currencyRegex.MatchString(currency) {
		return nil, ErrInvalidCurrency
	}

	totalDebits, err := NewMoney(0, currency)
	if err != nil {
		return nil, err
	}
	totalCredits, err := NewMoney(0, currency)
	if err != nil {
		return nil, err
	}

	validatedEntries := make([]Entry, len(entries))

	for i, entry := range entries {
		if entry.Currency != currency {
			return nil, ErrMultipleCurrencies
		}
		if entry.Amount <= 0 {
			return nil, ErrAmountNonPositive
		}
		if !entry.Direction.Valid() {
			return nil, ErrInvalidEntryDirection
		}

		entryMoney, err := NewMoney(entry.Amount, currency)
		if err != nil {
			return nil, err
		}

		if entry.Direction == DirectionDebit {
			totalDebits, err = totalDebits.Add(entryMoney)
			if err != nil {
				return nil, err
			}
		} else {
			totalCredits, err = totalCredits.Add(entryMoney)
			if err != nil {
				return nil, err
			}
		}

		entry.TransactionID = id
		if entry.ID == uuid.Nil {
			entry.ID = uuid.New()
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = createdAt
		}
		validatedEntries[i] = entry
	}

	if totalDebits.Amount != totalCredits.Amount {
		return nil, ErrUnbalancedTransaction
	}

	return &Transaction{
		ID:             id,
		ClientID:       clientID,
		IdempotencyKey: idempotencyKey,
		RequestHash:    requestHash,
		Description:    description,
		ReversalOf:     nil,
		Entries:        validatedEntries,
		CreatedAt:      createdAt,
	}, nil
}

// NewReversalTransaction creates a balancing reversal transaction from an original transaction.
func NewReversalTransaction(
	original *Transaction,
	reversalID uuid.UUID,
	clientID string,
	idempotencyKey string,
	requestHash string,
	description string,
	createdAt time.Time,
) (*Transaction, error) {
	if original == nil {
		return nil, ErrInvalidTransactionID
	}
	if original.ReversalOf != nil {
		return nil, ErrCannotReverseReversal
	}
	if reversalID == uuid.Nil {
		reversalID = uuid.New()
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	// Invert directions of all entries
	reversedEntries := make([]Entry, len(original.Entries))
	for i, origEntry := range original.Entries {
		reversedEntries[i] = Entry{
			ID:            uuid.New(),
			TransactionID: reversalID,
			AccountID:     origEntry.AccountID,
			Amount:        origEntry.Amount,
			Direction:     origEntry.Direction.Reverse(),
			Currency:      origEntry.Currency,
			CreatedAt:     createdAt,
		}
	}

	tx, err := NewTransaction(
		reversalID,
		clientID,
		idempotencyKey,
		requestHash,
		description,
		reversedEntries,
		createdAt,
	)
	if err != nil {
		return nil, err
	}

	tx.ReversalOf = &original.ID
	return tx, nil
}
