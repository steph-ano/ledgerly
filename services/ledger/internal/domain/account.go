package domain

import (
	"time"

	"github.com/google/uuid"
)

type AccountType string

const (
	AccountTypeCustomer AccountType = "customer"
	AccountTypeMerchant AccountType = "merchant"
	AccountTypePlatform AccountType = "platform"
	AccountTypeFees     AccountType = "fees"
)

// Valid returns true if AccountType is one of the recognized types.
func (t AccountType) Valid() bool {
	switch t {
	case AccountTypeCustomer, AccountTypeMerchant, AccountTypePlatform, AccountTypeFees:
		return true
	default:
		return false
	}
}

// Account represents a financial ledger account.
type Account struct {
	ID        uuid.UUID   `json:"id"`
	ClientID  string      `json:"client_id"`
	Type      AccountType `json:"type"`
	Currency  string      `json:"currency"`
	CreatedAt time.Time   `json:"created_at"`
}

// NewAccount creates and validates a new Account entity.
func NewAccount(id uuid.UUID, clientID string, accType AccountType, currency string, createdAt time.Time) (*Account, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidAccountID
	}
	if clientID == "" {
		return nil, ErrEmptyClientID
	}
	if !accType.Valid() {
		return nil, ErrInvalidAccountType
	}
	if !currencyRegex.MatchString(currency) {
		return nil, ErrInvalidCurrency
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	return &Account{
		ID:        id,
		ClientID:  clientID,
		Type:      accType,
		Currency:  currency,
		CreatedAt: createdAt,
	}, nil
}

// AccountBalance captures the cumulative debits and credits of an account.
type AccountBalance struct {
	AccountID uuid.UUID `json:"account_id"`
	Debits    int64     `json:"debits"`
	Credits   int64     `json:"credits"`
	Currency  string    `json:"currency"`
}

// NetDebit returns Debits - Credits (normal balance for Asset accounts like customer and platform).
func (b AccountBalance) NetDebit() (Money, error) {
	debitMoney, err := NewMoney(b.Debits, b.Currency)
	if err != nil {
		return Money{}, err
	}
	creditMoney, err := NewMoney(b.Credits, b.Currency)
	if err != nil {
		return Money{}, err
	}
	return debitMoney.Sub(creditMoney)
}

// NetCredit returns Credits - Debits (normal balance for Liability & Revenue accounts like merchant and fees).
func (b AccountBalance) NetCredit() (Money, error) {
	creditMoney, err := NewMoney(b.Credits, b.Currency)
	if err != nil {
		return Money{}, err
	}
	debitMoney, err := NewMoney(b.Debits, b.Currency)
	if err != nil {
		return Money{}, err
	}
	return creditMoney.Sub(debitMoney)
}

// ValidateBalancePolicy enforces the negative balance policy specified in ADR-0001.
// platformOverdraftLimit must be >= 0 (cents allowed in overdraft for platform operational cash).
func (a *Account) ValidateBalancePolicy(balance AccountBalance, platformOverdraftLimit int64) error {
	if balance.Currency != a.Currency {
		return ErrCurrencyMismatch
	}

	switch a.Type {
	case AccountTypeCustomer:
		// Normal balance: DEBIT (receivables). Debit balance must be >= 0 (credits cannot exceed debits).
		netDebit, err := balance.NetDebit()
		if err != nil {
			return err
		}
		if netDebit.Amount < 0 {
			return ErrNegativeBalanceNotAllowed
		}

	case AccountTypeMerchant:
		// Normal balance: CREDIT (payables). Credit balance must be >= 0 (platform cannot overpay merchant).
		netCredit, err := balance.NetCredit()
		if err != nil {
			return err
		}
		if netCredit.Amount < 0 {
			return ErrInsufficientFunds
		}

	case AccountTypePlatform:
		// Normal balance: DEBIT (cash/operational reserves).
		// Must not breach the operational overdraft limit floor: netDebit >= -platformOverdraftLimit.
		netDebit, err := balance.NetDebit()
		if err != nil {
			return err
		}
		if platformOverdraftLimit < 0 {
			platformOverdraftLimit = 0
		}
		if netDebit.Amount < -platformOverdraftLimit {
			return ErrPlatformLiquidityBreached
		}

	case AccountTypeFees:
		// Normal balance: CREDIT (earned revenues). Credit balance must be >= 0.
		netCredit, err := balance.NetCredit()
		if err != nil {
			return err
		}
		if netCredit.Amount < 0 {
			return ErrNegativeBalanceNotAllowed
		}

	default:
		return ErrInvalidAccountType
	}

	return nil
}
