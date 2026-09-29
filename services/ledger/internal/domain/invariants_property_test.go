package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

// TestProperty_MoneyAdditionCommutativity tests that A + B == B + A for all non-overflowing values.
func TestProperty_MoneyAdditionCommutativity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Bound to safe range to prevent overflow in random generation
		a := rapid.Int64Range(-math.MaxInt64/4, math.MaxInt64/4).Draw(t, "amountA")
		b := rapid.Int64Range(-math.MaxInt64/4, math.MaxInt64/4).Draw(t, "amountB")

		mA := domain.MustMoney(a, "USD")
		mB := domain.MustMoney(b, "USD")

		resAB, errAB := mA.Add(mB)
		resBA, errBA := mB.Add(mA)

		require.NoError(t, errAB)
		require.NoError(t, errBA)
		require.Equal(t, resAB.Amount, resBA.Amount)
	})
}

// TestProperty_MoneyIdentityAndInverse tests A + 0 == A and A - A == 0.
func TestProperty_MoneyIdentityAndInverse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.Int64Range(-math.MaxInt64/2, math.MaxInt64/2).Draw(t, "amountA")
		mA := domain.MustMoney(a, "USD")
		zero := domain.MustMoney(0, "USD")

		resAddZero, err := mA.Add(zero)
		require.NoError(t, err)
		require.Equal(t, mA.Amount, resAddZero.Amount)

		resSubSelf, err := mA.Sub(mA)
		require.NoError(t, err)
		require.True(t, resSubSelf.IsZero())
	})
}

// TestProperty_TransactionBalanceInvariant verifies that any constructed transaction enforces sum(debits) == sum(credits).
func TestProperty_TransactionBalanceInvariant(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 1 to 5 debit amounts (each > 0)
		numDebits := rapid.IntRange(1, 5).Draw(t, "numDebits")
		var totalDebit int64

		debitEntries := make([]domain.Entry, numDebits)
		for i := 0; i < numDebits; i++ {
			// keep amounts reasonable to avoid sum overflow
			amt := rapid.Int64Range(1, 100_000_000).Draw(t, "debitAmt")
			totalDebit += amt
			debitEntries[i] = domain.Entry{
				AccountID: uuid.New(),
				Amount:    amt,
				Direction: domain.DirectionDebit,
				Currency:  "USD",
			}
		}

		// Bound numCredits so that each entry can receive at least 1 cent
		maxCredits := 5
		if int64(maxCredits) > totalDebit {
			maxCredits = int(totalDebit)
		}
		numCredits := rapid.IntRange(1, maxCredits).Draw(t, "numCredits")
		creditEntries := make([]domain.Entry, numCredits)
		remaining := totalDebit

		for i := 0; i < numCredits-1; i++ {
			neededForOthers := int64(numCredits - 1 - i)
			maxForThis := remaining - neededForOthers
			amt := rapid.Int64Range(1, maxForThis).Draw(t, "creditAmt")
			remaining -= amt
			creditEntries[i] = domain.Entry{
				AccountID: uuid.New(),
				Amount:    amt,
				Direction: domain.DirectionCredit,
				Currency:  "USD",
			}
		}
		// Final credit takes the exact remainder
		creditEntries[len(creditEntries)-1] = domain.Entry{
			AccountID: uuid.New(),
			Amount:    remaining,
			Direction: domain.DirectionCredit,
			Currency:  "USD",
		}

		allEntries := append(debitEntries, creditEntries...)
		if len(allEntries) < 2 {
			return
		}

		hash := sha256.Sum256([]byte("payload"))
		hexHash := hex.EncodeToString(hash[:])

		tx, err := domain.NewTransaction(
			uuid.New(),
			"client_prop",
			uuid.NewString(),
			hexHash,
			"Property Test Balanced Transaction",
			allEntries,
			time.Now().UTC(),
		)
		require.NoError(t, err)
		require.NotNil(t, tx)

		// Calculate net sum across all entries
		var sumDebit, sumCredit int64
		for _, e := range tx.Entries {
			if e.Direction == domain.DirectionDebit {
				sumDebit += e.Amount
			} else {
				sumCredit += e.Amount
			}
		}
		require.Equal(t, sumDebit, sumCredit, "central ledger invariant violated: debits != credits")
	})
}

// TestProperty_ReversalNetZeroEffect verifies that combining an original transaction with its reversal
// yields exactly zero net balance change across every involved account.
func TestProperty_ReversalNetZeroEffect(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		accA := uuid.New()
		accB := uuid.New()
		amount := rapid.Int64Range(1, 1_000_000_000).Draw(t, "transferAmount")

		entries := []domain.Entry{
			{AccountID: accA, Amount: amount, Direction: domain.DirectionDebit, Currency: "USD"},
			{AccountID: accB, Amount: amount, Direction: domain.DirectionCredit, Currency: "USD"},
		}

		hash := sha256.Sum256([]byte("orig"))
		origTx, err := domain.NewTransaction(
			uuid.New(),
			"client_prop",
			uuid.NewString(),
			hex.EncodeToString(hash[:]),
			"Original",
			entries,
			time.Now().UTC(),
		)
		require.NoError(t, err)

		revHash := sha256.Sum256([]byte("rev"))
		revTx, err := domain.NewReversalTransaction(
			origTx,
			uuid.New(),
			"client_prop",
			uuid.NewString(),
			hex.EncodeToString(revHash[:]),
			"Reversal",
			time.Now().UTC(),
		)
		require.NoError(t, err)
		require.Equal(t, &origTx.ID, revTx.ReversalOf)

		// Map net effect by account across both transactions
		netChange := make(map[uuid.UUID]int64)
		for _, e := range append(origTx.Entries, revTx.Entries...) {
			if e.Direction == domain.DirectionDebit {
				netChange[e.AccountID] += e.Amount
			} else {
				netChange[e.AccountID] -= e.Amount
			}
		}

		// Every account must have net effect = 0
		for acc, net := range netChange {
			require.Equal(t, int64(0), net, "account %s did not net to zero after reversal", acc)
		}

		// A reversal of revTx must be rejected
		_, err = domain.NewReversalTransaction(
			revTx,
			uuid.New(),
			"client_prop",
			uuid.NewString(),
			hex.EncodeToString(revHash[:]),
			"Reversal of reversal",
			time.Now().UTC(),
		)
		require.ErrorIs(t, err, domain.ErrCannotReverseReversal)
	})
}
