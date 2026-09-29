package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

func validHash(payload string) string {
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

func TestNewTransaction(t *testing.T) {
	txID := uuid.New()
	accA := uuid.New()
	accB := uuid.New()
	hash := validHash("test-payload")
	now := time.Now().UTC()

	t.Run("valid balanced transaction", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: 1000, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: 1000, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		tx, err := domain.NewTransaction(txID, "client_1", "key_1", hash, "Purchase", entries, now)
		require.NoError(t, err)
		assert.Equal(t, txID, tx.ID)
		assert.Equal(t, "key_1", tx.IdempotencyKey)
		assert.Equal(t, hash, tx.RequestHash)
		assert.Len(t, tx.Entries, 2)
		assert.Equal(t, txID, tx.Entries[0].TransactionID)
		assert.Equal(t, txID, tx.Entries[1].TransactionID)
		assert.Nil(t, tx.ReversalOf)
	})

	t.Run("multi-entry balanced transaction", func(t *testing.T) {
		accFee := uuid.New()
		entries := []domain.Entry{
			{AccountID: accA, Amount: 1000, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: 950, Direction: domain.DirectionCredit, Currency: "USD"},
			{AccountID: accFee, Amount: 50, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		tx, err := domain.NewTransaction(txID, "client_1", "key_2", hash, "Purchase with Fee", entries, now)
		require.NoError(t, err)
		assert.Len(t, tx.Entries, 3)
	})

	t.Run("unbalanced transaction fails", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: 1000, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: 999, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		_, err := domain.NewTransaction(txID, "client_1", "key_3", hash, "Unbalanced", entries, now)
		assert.ErrorIs(t, err, domain.ErrUnbalancedTransaction)
	})

	t.Run("fewer than 2 entries fails", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: 1000, Direction: domain.DirectionDebit, Currency: "USD"},
		}

		_, err := domain.NewTransaction(txID, "client_1", "key_4", hash, "Single entry", entries, now)
		assert.ErrorIs(t, err, domain.ErrInsufficientEntries)
	})

	t.Run("mixed currencies fails", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: 1000, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: 1000, Direction: domain.DirectionCredit, Currency: "EUR"},
		}

		_, err := domain.NewTransaction(txID, "client_1", "key_5", hash, "Mixed currency", entries, now)
		assert.ErrorIs(t, err, domain.ErrMultipleCurrencies)
	})

	t.Run("zero or negative amount fails", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: 0, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: 0, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		_, err := domain.NewTransaction(txID, "client_1", "key_6", hash, "Zero amount", entries, now)
		assert.ErrorIs(t, err, domain.ErrAmountNonPositive)
	})

	t.Run("invalid request hash format", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: 1000, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: 1000, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		_, err := domain.NewTransaction(txID, "client_1", "key_7", "not-a-valid-hex-hash", "Invalid hash", entries, now)
		assert.ErrorIs(t, err, domain.ErrInvalidRequestHash)
	})

	t.Run("arithmetic overflow in total entries fails safe", func(t *testing.T) {
		entries := []domain.Entry{
			{AccountID: accA, Amount: math.MaxInt64 - 10, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accA, Amount: 20, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: math.MaxInt64 - 10, Direction: domain.DirectionCredit, Currency: "USD"},
			{AccountID: accB, Amount: 20, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		_, err := domain.NewTransaction(txID, "client_1", "key_8", hash, "Overflow entries", entries, now)
		assert.ErrorIs(t, err, domain.ErrAmountOverflow)
	})
}

func TestNewReversalTransaction(t *testing.T) {
	origTxID := uuid.New()
	reversalID := uuid.New()
	accA := uuid.New()
	accB := uuid.New()
	hash := validHash("orig-payload")
	revHash := validHash("reversal-payload")
	now := time.Now().UTC()

	entries := []domain.Entry{
		{AccountID: accA, Amount: 1500, Direction: domain.DirectionDebit, Currency: "USD"},
		{AccountID: accB, Amount: 1500, Direction: domain.DirectionCredit, Currency: "USD"},
	}

	origTx, err := domain.NewTransaction(origTxID, "client_1", "key_orig", hash, "Original Sale", entries, now)
	require.NoError(t, err)

	t.Run("successful reversal creation", func(t *testing.T) {
		revTx, err := domain.NewReversalTransaction(
			origTx,
			reversalID,
			"client_1",
			"key_rev_1",
			revHash,
			"Reversal of Original Sale",
			now,
		)
		require.NoError(t, err)
		assert.Equal(t, reversalID, revTx.ID)
		assert.Equal(t, &origTxID, revTx.ReversalOf)
		assert.Len(t, revTx.Entries, 2)

		// Directions must be inverted
		assert.Equal(t, accA, revTx.Entries[0].AccountID)
		assert.Equal(t, domain.DirectionCredit, revTx.Entries[0].Direction) // Debit became Credit
		assert.Equal(t, int64(1500), revTx.Entries[0].Amount)

		assert.Equal(t, accB, revTx.Entries[1].AccountID)
		assert.Equal(t, domain.DirectionDebit, revTx.Entries[1].Direction) // Credit became Debit
		assert.Equal(t, int64(1500), revTx.Entries[1].Amount)
	})

	t.Run("cannot reverse a transaction that is already a reversal", func(t *testing.T) {
		revTx, err := domain.NewReversalTransaction(
			origTx,
			reversalID,
			"client_1",
			"key_rev_2",
			revHash,
			"Reversal of Original Sale",
			now,
		)
		require.NoError(t, err)

		// Attempting to reverse revTx
		_, err = domain.NewReversalTransaction(
			revTx,
			uuid.New(),
			"client_1",
			"key_rev_3",
			validHash("rev-of-rev"),
			"Reversal of reversal",
			now,
		)
		assert.ErrorIs(t, err, domain.ErrCannotReverseReversal)
	})
}
