package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// RecordTransaction atomically records a balanced transaction with idempotency and balance policy enforcement.
// Implements ADR-0002 (deterministic SELECT ... FOR UPDATE) and ADR-0003 (ON CONFLICT DO NOTHING with hash comparison).
func (r *TransactionRepository) RecordTransaction(
	ctx context.Context,
	txEntity *domain.Transaction,
	platformOverdraftLimit int64,
) (*domain.Transaction, bool, error) {
	// Execute inside READ COMMITTED transaction
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Idempotency Insert with ON CONFLICT DO NOTHING
	insertTxQuery := `
		INSERT INTO transactions (id, client_id, idempotency_key, request_hash, description, reversal_of, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (client_id, idempotency_key) DO NOTHING
		RETURNING id;
	`
	var insertedID uuid.UUID
	err = tx.QueryRowContext(ctx, insertTxQuery,
		txEntity.ID,
		txEntity.ClientID,
		txEntity.IdempotencyKey,
		txEntity.RequestHash,
		txEntity.Description,
		txEntity.ReversalOf,
		txEntity.CreatedAt,
	).Scan(&insertedID)

	// 2. Branch: Conflict Handling (Existing Key)
	if errors.Is(err, sql.ErrNoRows) {
		// No row inserted: a record with this (client_id, idempotency_key) already exists
		selectExistingQuery := `
			SELECT id, client_id, idempotency_key, request_hash, description, reversal_of, created_at
			FROM transactions
			WHERE client_id = $1 AND idempotency_key = $2
		`
		var existing domain.Transaction
		var existingReversalOf *uuid.UUID

		err = tx.QueryRowContext(ctx, selectExistingQuery, txEntity.ClientID, txEntity.IdempotencyKey).Scan(
			&existing.ID,
			&existing.ClientID,
			&existing.IdempotencyKey,
			&existing.RequestHash,
			&existing.Description,
			&existingReversalOf,
			&existing.CreatedAt,
		)
		if err != nil {
			return nil, false, fmt.Errorf("failed to read existing idempotency record: %w", err)
		}
		existing.ReversalOf = existingReversalOf

		// Verify canonical request hash
		if existing.RequestHash != txEntity.RequestHash {
			return nil, false, domain.ErrIdempotencyKeyPayloadMismatch
		}

		// Hashes match: retrieve original entries and return existing transaction
		entries, err := r.listEntriesByTransactionTx(ctx, tx, existing.ID)
		if err != nil {
			return nil, false, fmt.Errorf("failed to load existing transaction entries: %w", err)
		}
		existing.Entries = entries

		if err := tx.Commit(); err != nil {
			return nil, false, fmt.Errorf("failed to commit read transaction: %w", err)
		}

		return &existing, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("failed to execute idempotency insert: %w", err)
	}

	// 3. New Transaction: Collect and Sort Account IDs in Deterministic Ascending Order
	accountMap := make(map[uuid.UUID]bool)
	for _, entry := range txEntity.Entries {
		accountMap[entry.AccountID] = true
	}

	sortedAccountIDs := make([]uuid.UUID, 0, len(accountMap))
	for accID := range accountMap {
		sortedAccountIDs = append(sortedAccountIDs, accID)
	}
	sort.Slice(sortedAccountIDs, func(i, j int) bool {
		return sortedAccountIDs[i].String() < sortedAccountIDs[j].String()
	})

	// 4. Acquire Row Locks in Deterministic Order (SELECT ... FOR UPDATE)
	accountsByID := make(map[uuid.UUID]*domain.Account)
	for _, accID := range sortedAccountIDs {
		lockQuery := `
			SELECT id, client_id, type, currency, created_at
			FROM accounts
			WHERE id = $1
			FOR UPDATE;
		`
		var acc domain.Account
		var accType string
		err := tx.QueryRowContext(ctx, lockQuery, accID).Scan(
			&acc.ID,
			&acc.ClientID,
			&accType,
			&acc.Currency,
			&acc.CreatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, false, fmt.Errorf("%w: account id %s", domain.ErrAccountNotFound, accID)
			}
			return nil, false, fmt.Errorf("failed to lock account %s: %w", accID, err)
		}
		acc.Type = domain.AccountType(accType)
		accountsByID[accID] = &acc
	}

	// 5. Calculate Current and Projected Balances, and Enforce Negative Balance Policies
	for _, accID := range sortedAccountIDs {
		acc := accountsByID[accID]

		// Query current sum of debits and credits inside the locked transaction
		balanceQuery := `
			SELECT 
				COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) AS debits,
				COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) AS credits
			FROM entries
			WHERE account_id = $1
		`
		var currentDebits, currentCredits int64
		err := tx.QueryRowContext(ctx, balanceQuery, accID).Scan(&currentDebits, &currentCredits)
		if err != nil {
			return nil, false, fmt.Errorf("failed to query balance for account %s: %w", accID, err)
		}

		// Accumulate movements from this transaction
		projectedDebits := currentDebits
		projectedCredits := currentCredits
		for _, entry := range txEntity.Entries {
			if entry.AccountID == accID {
				if entry.Currency != acc.Currency {
					return nil, false, domain.ErrAccountCurrencyMismatch
				}
				if entry.Direction == domain.DirectionDebit {
					projectedDebits += entry.Amount
				} else {
					projectedCredits += entry.Amount
				}
			}
		}

		projectedBalance := domain.AccountBalance{
			AccountID: accID,
			Debits:    projectedDebits,
			Credits:   projectedCredits,
			Currency:  acc.Currency,
		}

		// Enforce account negative balance policy
		if err := acc.ValidateBalancePolicy(projectedBalance, platformOverdraftLimit); err != nil {
			if txEntity.ReversalOf != nil && errors.Is(err, domain.ErrInsufficientFunds) {
				return nil, false, domain.ErrInsufficientFundsForReversal
			}
			return nil, false, err
		}
	}

	// 6. Insert Entries
	insertEntryQuery := `
		INSERT INTO entries (id, transaction_id, account_id, amount, direction, currency, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	stmt, err := tx.PrepareContext(ctx, insertEntryQuery)
	if err != nil {
		return nil, false, fmt.Errorf("failed to prepare entry insert statement: %w", err)
	}
	defer stmt.Close()

	for _, entry := range txEntity.Entries {
		_, err := stmt.ExecContext(ctx,
			entry.ID,
			txEntity.ID,
			entry.AccountID,
			entry.Amount,
			string(entry.Direction),
			entry.Currency,
			entry.CreatedAt,
		)
		if err != nil {
			return nil, false, fmt.Errorf("failed to insert entry: %w", err)
		}
	}

	// 7. Commit Transaction
	// PostgreSQL deferred constraint triggers will automatically verify minimum 2 entries and zero-sum balance
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return txEntity, true, nil
}

// GetByID retrieves a transaction and its entries by ID.
func (r *TransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, client_id, idempotency_key, request_hash, description, reversal_of, created_at
		FROM transactions
		WHERE id = $1
	`
	var tx domain.Transaction
	var reversalOf *uuid.UUID

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID,
		&tx.ClientID,
		&tx.IdempotencyKey,
		&tx.RequestHash,
		&tx.Description,
		&reversalOf,
		&tx.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	tx.ReversalOf = reversalOf

	entries, err := r.ListEntriesByTransaction(ctx, tx.ID)
	if err != nil {
		return nil, err
	}
	tx.Entries = entries

	return &tx, nil
}

// GetByClientAndIdempotencyKey retrieves a transaction by client_id and idempotency_key.
func (r *TransactionRepository) GetByClientAndIdempotencyKey(ctx context.Context, clientID, idempotencyKey string) (*domain.Transaction, error) {
	query := `
		SELECT id, client_id, idempotency_key, request_hash, description, reversal_of, created_at
		FROM transactions
		WHERE client_id = $1 AND idempotency_key = $2
	`
	var tx domain.Transaction
	var reversalOf *uuid.UUID

	err := r.db.QueryRowContext(ctx, query, clientID, idempotencyKey).Scan(
		&tx.ID,
		&tx.ClientID,
		&tx.IdempotencyKey,
		&tx.RequestHash,
		&tx.Description,
		&reversalOf,
		&tx.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to query transaction by idempotency key: %w", err)
	}
	tx.ReversalOf = reversalOf

	entries, err := r.ListEntriesByTransaction(ctx, tx.ID)
	if err != nil {
		return nil, err
	}
	tx.Entries = entries

	return &tx, nil
}

// ListEntriesByTransaction retrieves all entries associated with a transaction.
func (r *TransactionRepository) ListEntriesByTransaction(ctx context.Context, txID uuid.UUID) ([]domain.Entry, error) {
	query := `
		SELECT id, transaction_id, account_id, amount, direction, currency, created_at
		FROM entries
		WHERE transaction_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entries []domain.Entry
	for rows.Next() {
		var e domain.Entry
		var dir string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &dir, &e.Currency, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		e.Direction = domain.Direction(dir)
		entries = append(entries, e)
	}
	return entries, nil
}

func (r *TransactionRepository) listEntriesByTransactionTx(ctx context.Context, tx *sql.Tx, txID uuid.UUID) ([]domain.Entry, error) {
	query := `
		SELECT id, transaction_id, account_id, amount, direction, currency, created_at
		FROM entries
		WHERE transaction_id = $1
		ORDER BY created_at ASC
	`
	rows, err := tx.QueryContext(ctx, query, txID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []domain.Entry
	for rows.Next() {
		var e domain.Entry
		var dir string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &dir, &e.Currency, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.Direction = domain.Direction(dir)
		entries = append(entries, e)
	}
	return entries, nil
}

// ListEntriesByAccount retrieves paginated ledger entries for a given account.
func (r *TransactionRepository) ListEntriesByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.Entry, error) {
	query := `
		SELECT id, transaction_id, account_id, amount, direction, currency, created_at
		FROM entries
		WHERE account_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries by account: %w", err)
	}
	defer rows.Close()

	var entries []domain.Entry
	for rows.Next() {
		var e domain.Entry
		var dir string
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &dir, &e.Currency, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		e.Direction = domain.Direction(dir)
		entries = append(entries, e)
	}
	return entries, nil
}

// ReverseTransaction reverses an existing transaction atomically.
func (r *TransactionRepository) ReverseTransaction(
	ctx context.Context,
	origTxID uuid.UUID,
	reversalID uuid.UUID,
	clientID string,
	idempotencyKey string,
	requestHash string,
	description string,
	platformOverdraftLimit int64,
) (*domain.Transaction, bool, error) {
	origTx, err := r.GetByID(ctx, origTxID)
	if err != nil {
		return nil, false, err
	}

	reversalTx, err := domain.NewReversalTransaction(
		origTx,
		reversalID,
		clientID,
		idempotencyKey,
		requestHash,
		description,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, false, err
	}

	return r.RecordTransaction(ctx, reversalTx, platformOverdraftLimit)
}
