package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new account into the database.
func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO accounts (id, client_id, type, currency, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.ExecContext(ctx, query,
		account.ID,
		account.ClientID,
		string(account.Type),
		account.Currency,
		account.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert account: %w", err)
	}
	return nil
}

// GetByID retrieves an account by its unique UUID.
func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	query := `
		SELECT id, client_id, type, currency, created_at
		FROM accounts
		WHERE id = $1
	`
	var acc domain.Account
	var accType string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&acc.ID,
		&acc.ClientID,
		&accType,
		&acc.Currency,
		&acc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, fmt.Errorf("failed to query account by id: %w", err)
	}

	acc.Type = domain.AccountType(accType)
	return &acc, nil
}

// GetBalance calculates the current balance of an account using SUM(entries).
func (r *AccountRepository) GetBalance(ctx context.Context, accountID uuid.UUID) (*domain.AccountBalance, error) {
	// First ensure account exists and get currency
	acc, err := r.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE 0 END), 0) AS debits,
			COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0) AS credits
		FROM entries
		WHERE account_id = $1
	`
	var bal domain.AccountBalance
	bal.AccountID = accountID
	bal.Currency = acc.Currency

	err = r.db.QueryRowContext(ctx, query, accountID).Scan(
		&bal.Debits,
		&bal.Credits,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate account balance: %w", err)
	}

	return &bal, nil
}
