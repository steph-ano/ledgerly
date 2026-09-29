package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

type CreateAccountRequest struct {
	ClientID string             `json:"client_id"`
	Type     domain.AccountType `json:"type"`
	Currency string             `json:"currency"`
}

type EntryRequest struct {
	AccountID uuid.UUID        `json:"account_id"`
	Amount    int64            `json:"amount"`
	Direction domain.Direction `json:"direction"`
	Currency  string           `json:"currency"`
}

type RecordTransactionRequest struct {
	ClientID       string         `json:"client_id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Description    string         `json:"description"`
	Entries        []EntryRequest `json:"entries"`
}

type ReverseTransactionRequest struct {
	ClientID              string    `json:"client_id"`
	IdempotencyKey        string    `json:"idempotency_key"`
	OriginalTransactionID uuid.UUID `json:"original_transaction_id"`
	Description           string    `json:"description"`
}

// CanonicalPayload forms the normalized body used to compute the request_hash
type CanonicalPayload struct {
	Description string         `json:"description"`
	Entries     []EntryRequest `json:"entries"`
}

type LedgerService struct {
	accountRepo            domain.AccountRepository
	txRepo                 domain.TransactionRepository
	platformOverdraftLimit int64
}

func NewLedgerService(
	accountRepo domain.AccountRepository,
	txRepo domain.TransactionRepository,
	platformOverdraftLimit int64,
) *LedgerService {
	return &LedgerService{
		accountRepo:            accountRepo,
		txRepo:                 txRepo,
		platformOverdraftLimit: platformOverdraftLimit,
	}
}

// CreateAccount validates and creates a new account.
func (s *LedgerService) CreateAccount(ctx context.Context, req CreateAccountRequest) (*domain.Account, error) {
	acc, err := domain.NewAccount(uuid.New(), req.ClientID, req.Type, req.Currency, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	if err := s.accountRepo.Create(ctx, acc); err != nil {
		return nil, fmt.Errorf("failed to persist account: %w", err)
	}

	return acc, nil
}

// GetAccount retrieves an account by ID.
func (s *LedgerService) GetAccount(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	return s.accountRepo.GetByID(ctx, id)
}

// GetAccountBalance calculates and returns the current balance of an account.
func (s *LedgerService) GetAccountBalance(ctx context.Context, id uuid.UUID) (*domain.AccountBalance, error) {
	return s.accountRepo.GetBalance(ctx, id)
}

// RecordTransaction validates and atomically records a transaction with idempotency and balance policies.
func (s *LedgerService) RecordTransaction(ctx context.Context, req RecordTransactionRequest) (*domain.Transaction, bool, error) {
	if req.ClientID == "" {
		return nil, false, domain.ErrEmptyClientID
	}
	if req.IdempotencyKey == "" {
		return nil, false, domain.ErrEmptyIdempotencyKey
	}
	if req.Description == "" {
		return nil, false, domain.ErrEmptyDescription
	}

	// Compute canonical hash over description + entries
	canonical := CanonicalPayload{
		Description: req.Description,
		Entries:     req.Entries,
	}
	reqHash, err := CanonicalHash(canonical)
	if err != nil {
		return nil, false, fmt.Errorf("failed to compute canonical request hash: %w", err)
	}

	txID := uuid.New()
	entries := make([]domain.Entry, len(req.Entries))
	now := time.Now().UTC()

	for i, e := range req.Entries {
		entry, err := domain.NewEntry(
			uuid.New(),
			txID,
			e.AccountID,
			e.Amount,
			e.Direction,
			e.Currency,
			now,
		)
		if err != nil {
			return nil, false, err
		}
		entries[i] = *entry
	}

	txEntity, err := domain.NewTransaction(
		txID,
		req.ClientID,
		req.IdempotencyKey,
		reqHash,
		req.Description,
		entries,
		now,
	)
	if err != nil {
		return nil, false, err
	}

	return s.txRepo.RecordTransaction(ctx, txEntity, s.platformOverdraftLimit)
}

// ReverseTransaction reverses an existing transaction atomically.
func (s *LedgerService) ReverseTransaction(ctx context.Context, req ReverseTransactionRequest) (*domain.Transaction, bool, error) {
	if req.ClientID == "" {
		return nil, false, domain.ErrEmptyClientID
	}
	if req.IdempotencyKey == "" {
		return nil, false, domain.ErrEmptyIdempotencyKey
	}
	if req.OriginalTransactionID == uuid.Nil {
		return nil, false, domain.ErrInvalidTransactionID
	}
	if req.Description == "" {
		req.Description = fmt.Sprintf("Reversal of transaction %s", req.OriginalTransactionID)
	}

	origTx, err := s.txRepo.GetByID(ctx, req.OriginalTransactionID)
	if err != nil {
		return nil, false, err
	}

	canonical := map[string]any{
		"action":                  "reversal",
		"original_transaction_id": req.OriginalTransactionID.String(),
		"description":             req.Description,
	}
	reqHash, err := CanonicalHash(canonical)
	if err != nil {
		return nil, false, fmt.Errorf("failed to compute reversal canonical hash: %w", err)
	}

	reversalID := uuid.New()
	reversalTx, err := domain.NewReversalTransaction(
		origTx,
		reversalID,
		req.ClientID,
		req.IdempotencyKey,
		reqHash,
		req.Description,
		time.Now().UTC(),
	)
	if err != nil {
		return nil, false, err
	}

	return s.txRepo.RecordTransaction(ctx, reversalTx, s.platformOverdraftLimit)
}

// GetTransaction retrieves a transaction by ID.
func (s *LedgerService) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	return s.txRepo.GetByID(ctx, id)
}

// ListEntries returns ledger entries for an account.
func (s *LedgerService) ListEntries(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]domain.Entry, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.txRepo.ListEntriesByAccount(ctx, accountID, limit, offset)
}
