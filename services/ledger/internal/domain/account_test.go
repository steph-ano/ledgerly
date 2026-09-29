package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/steph-ano/ledgerly/services/ledger/internal/domain"
)

func TestNewAccount(t *testing.T) {
	validID := uuid.New()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		id          uuid.UUID
		clientID    string
		accType     domain.AccountType
		currency    string
		createdAt   time.Time
		expectedErr error
	}{
		{"valid customer account", validID, "client_1", domain.AccountTypeCustomer, "USD", now, nil},
		{"valid merchant account", validID, "client_1", domain.AccountTypeMerchant, "EUR", now, nil},
		{"valid platform account", validID, "client_1", domain.AccountTypePlatform, "USD", now, nil},
		{"valid fees account", validID, "client_1", domain.AccountTypeFees, "USD", now, nil},
		{"nil account id", uuid.Nil, "client_1", domain.AccountTypeCustomer, "USD", now, domain.ErrInvalidAccountID},
		{"empty client id", validID, "", domain.AccountTypeCustomer, "USD", now, domain.ErrEmptyClientID},
		{"invalid account type", validID, "client_1", "invalid_type", "USD", now, domain.ErrInvalidAccountType},
		{"invalid currency", validID, "client_1", domain.AccountTypeCustomer, "US", now, domain.ErrInvalidCurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc, err := domain.NewAccount(tt.id, tt.clientID, tt.accType, tt.currency, tt.createdAt)
			if tt.expectedErr != nil {
				assert.ErrorIs(t, err, tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.id, acc.ID)
				assert.Equal(t, tt.accType, acc.Type)
				assert.Equal(t, tt.currency, acc.Currency)
			}
		})
	}
}

func TestAccount_ValidateBalancePolicy(t *testing.T) {
	accID := uuid.New()

	t.Run("customer account policy", func(t *testing.T) {
		acc, err := domain.NewAccount(accID, "client_1", domain.AccountTypeCustomer, "USD", time.Now())
		require.NoError(t, err)

		// Debit balance >= 0 (debits >= credits): valid
		balValid := domain.AccountBalance{AccountID: accID, Debits: 1000, Credits: 400, Currency: "USD"}
		assert.NoError(t, acc.ValidateBalancePolicy(balValid, 0))

		// Debit balance = 0: valid
		balZero := domain.AccountBalance{AccountID: accID, Debits: 500, Credits: 500, Currency: "USD"}
		assert.NoError(t, acc.ValidateBalancePolicy(balZero, 0))

		// Debit balance < 0 (credits > debits): invalid
		balNegative := domain.AccountBalance{AccountID: accID, Debits: 400, Credits: 1000, Currency: "USD"}
		assert.ErrorIs(t, acc.ValidateBalancePolicy(balNegative, 0), domain.ErrNegativeBalanceNotAllowed)
	})

	t.Run("merchant account policy", func(t *testing.T) {
		acc, err := domain.NewAccount(accID, "client_1", domain.AccountTypeMerchant, "USD", time.Now())
		require.NoError(t, err)

		// Credit balance >= 0 (credits >= debits): valid
		balValid := domain.AccountBalance{AccountID: accID, Debits: 200, Credits: 1000, Currency: "USD"}
		assert.NoError(t, acc.ValidateBalancePolicy(balValid, 0))

		// Credit balance < 0 (debits > credits, e.g. overpayment/overdraft): invalid
		balNegative := domain.AccountBalance{AccountID: accID, Debits: 1200, Credits: 1000, Currency: "USD"}
		assert.ErrorIs(t, acc.ValidateBalancePolicy(balNegative, 0), domain.ErrInsufficientFunds)
	})

	t.Run("platform account policy with liquidity floor", func(t *testing.T) {
		acc, err := domain.NewAccount(accID, "client_1", domain.AccountTypePlatform, "USD", time.Now())
		require.NoError(t, err)

		// Default floor = 0 (debits >= credits)
		balPositive := domain.AccountBalance{AccountID: accID, Debits: 5000, Credits: 1000, Currency: "USD"}
		assert.NoError(t, acc.ValidateBalancePolicy(balPositive, 0))

		balBreachedNoCredit := domain.AccountBalance{AccountID: accID, Debits: 1000, Credits: 2000, Currency: "USD"}
		assert.ErrorIs(t, acc.ValidateBalancePolicy(balBreachedNoCredit, 0), domain.ErrPlatformLiquidityBreached)

		// With overdraft limit floor of $500 (50,000 cents):
		// Net debit = 1000 - 1400 = -400. Floor is -500. -400 >= -500 => Valid.
		balWithinOverdraft := domain.AccountBalance{AccountID: accID, Debits: 1000, Credits: 1400, Currency: "USD"}
		assert.NoError(t, acc.ValidateBalancePolicy(balWithinOverdraft, 500))

		// Net debit = 1000 - 1600 = -600. Floor is -500. -600 < -500 => Breached.
		balBeyondOverdraft := domain.AccountBalance{AccountID: accID, Debits: 1000, Credits: 1600, Currency: "USD"}
		assert.ErrorIs(t, acc.ValidateBalancePolicy(balBeyondOverdraft, 500), domain.ErrPlatformLiquidityBreached)
	})

	t.Run("fees account policy", func(t *testing.T) {
		acc, err := domain.NewAccount(accID, "client_1", domain.AccountTypeFees, "USD", time.Now())
		require.NoError(t, err)

		balValid := domain.AccountBalance{AccountID: accID, Debits: 0, Credits: 300, Currency: "USD"}
		assert.NoError(t, acc.ValidateBalancePolicy(balValid, 0))

		// Debits exceed credits (negative fee balance): invalid
		balNegative := domain.AccountBalance{AccountID: accID, Debits: 400, Credits: 300, Currency: "USD"}
		assert.ErrorIs(t, acc.ValidateBalancePolicy(balNegative, 0), domain.ErrNegativeBalanceNotAllowed)
	})

	t.Run("currency mismatch between account and balance", func(t *testing.T) {
		acc, err := domain.NewAccount(accID, "client_1", domain.AccountTypeCustomer, "USD", time.Now())
		require.NoError(t, err)

		balEUR := domain.AccountBalance{AccountID: accID, Debits: 1000, Credits: 500, Currency: "EUR"}
		assert.ErrorIs(t, acc.ValidateBalancePolicy(balEUR, 0), domain.ErrCurrencyMismatch)
	})
}
