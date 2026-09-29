package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrAccountNotFound               = errors.New("account not found")
	ErrTransactionNotFound           = errors.New("transaction not found")
	ErrIdempotencyKeyPayloadMismatch = errors.New("idempotency key already exists with a different payload")
)

// AccountRepository defines the persistence contract for ledger accounts.
type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	GetBalance(ctx context.Context, accountID uuid.UUID) (*AccountBalance, error)
}

// TransactionRepository defines the persistence contract for transactions and ledger entries.
type TransactionRepository interface {
	// RecordTransaction atomically records a balanced transaction with idempotency and balance policy enforcement.
	// Returns (transaction, isNewRecord, error).
	// If the transaction with the same (client_id, idempotency_key) already exists and payload matches, isNewRecord is false.
	RecordTransaction(ctx context.Context, tx *Transaction, platformOverdraftLimit int64) (*Transaction, bool, error)

	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	GetByClientAndIdempotencyKey(ctx context.Context, clientID, idempotencyKey string) (*Transaction, error)
	ListEntriesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]Entry, error)
}
