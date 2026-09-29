package domain

import (
	"regexp"
	"time"

	"github.com/google/uuid"
)

var currencyRegex = regexp.MustCompile(`^[A-Z]{3}$`)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusActive    OrderStatus = "active"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusDefaulted OrderStatus = "defaulted"
	OrderStatusCanceled  OrderStatus = "canceled"
)

func (s OrderStatus) Valid() bool {
	switch s {
	case OrderStatusPending, OrderStatusActive, OrderStatusCompleted, OrderStatusDefaulted, OrderStatusCanceled:
		return true
	default:
		return false
	}
}

// Order represents a BNPL purchase order split across 4 bi-weekly installments.
type Order struct {
	ID                 uuid.UUID     `json:"id"`
	ClientID           string        `json:"client_id"`
	CustomerAccountID  uuid.UUID     `json:"customer_account_id"`
	MerchantAccountID  uuid.UUID     `json:"merchant_account_id"`
	TotalAmount        int64         `json:"total_amount"` // in cents
	Currency           string        `json:"currency"`
	Status             OrderStatus   `json:"status"`
	MerchantWebhookURL string        `json:"merchant_webhook_url,omitempty"`
	Installments       []Installment `json:"installments"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// NewOrder validates and creates a new Order with 4 bi-weekly installments.
// Any non-divisible cent remainder is allocated to Installment 1 (down payment).
func NewOrder(
	id uuid.UUID,
	clientID string,
	customerAccountID uuid.UUID,
	merchantAccountID uuid.UUID,
	totalAmount int64,
	currency string,
	merchantWebhookURL string,
	createdAt time.Time,
) (*Order, error) {
	if id == uuid.Nil {
		id = uuid.New()
	}
	if clientID == "" {
		return nil, ErrEmptyClientID
	}
	if customerAccountID == uuid.Nil || merchantAccountID == uuid.Nil {
		return nil, ErrInvalidAccountID
	}
	if totalAmount <= 0 {
		return nil, ErrInvalidAmount
	}
	if !currencyRegex.MatchString(currency) {
		return nil, ErrInvalidCurrency
	}
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	// Calculate 4 bi-weekly installments
	baseInstallment := totalAmount / 4
	remainder := totalAmount % 4

	installments := make([]Installment, 4)
	for i := 0; i < 4; i++ {
		number := i + 1
		amount := baseInstallment
		if number == 1 {
			// First installment (down payment) absorbs remainder
			amount += remainder
		}

		// Bi-weekly due dates: 0 days, 14 days, 28 days, 42 days
		dueDate := createdAt.Add(time.Duration(i*14*24) * time.Hour)

		installments[i] = Installment{
			ID:           uuid.New(),
			OrderID:      id,
			Number:       number,
			Amount:       amount,
			Currency:     currency,
			DueDate:      dueDate,
			Status:       InstallmentStatusPending,
			AttemptCount: 0,
			CreatedAt:    createdAt,
		}
	}

	return &Order{
		ID:                 id,
		ClientID:           clientID,
		CustomerAccountID:  customerAccountID,
		MerchantAccountID:  merchantAccountID,
		TotalAmount:        totalAmount,
		Currency:           currency,
		Status:             OrderStatusPending,
		MerchantWebhookURL: merchantWebhookURL,
		Installments:       installments,
		CreatedAt:          createdAt,
		UpdatedAt:          createdAt,
	}, nil
}

// MarkActive transitions an order to active when the down payment (Cuota 1) is confirmed.
func (o *Order) MarkActive(now time.Time) error {
	if o.Status != OrderStatusPending {
		return ErrInvalidOrderStatusTransition
	}
	o.Status = OrderStatusActive
	o.UpdatedAt = now
	return nil
}

// CheckCompletion evaluates if all 4 installments are paid, transitioning the order to completed.
func (o *Order) CheckCompletion(now time.Time) bool {
	if o.Status != OrderStatusActive {
		return false
	}

	for _, inst := range o.Installments {
		if inst.Status != InstallmentStatusPaid {
			return false
		}
	}

	o.Status = OrderStatusCompleted
	o.UpdatedAt = now
	return true
}

// MarkDefaulted transitions an order to defaulted if an installment fails permanently.
func (o *Order) MarkDefaulted(now time.Time) {
	o.Status = OrderStatusDefaulted
	o.UpdatedAt = now
}
