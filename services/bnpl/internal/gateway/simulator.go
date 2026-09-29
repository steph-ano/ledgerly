package gateway

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

var (
	ErrGatewayTimeout     = errors.New("gateway connection timeout")
	ErrInvalidChargeAmount = errors.New("charge amount must be strictly positive")
)

type ChargeRequest struct {
	IdempotencyKey     string `json:"idempotency_key"`
	Amount             int64  `json:"amount"` // in cents
	Currency           string `json:"currency"`
	PaymentMethodToken string `json:"payment_method_token"`
	SimulateAction     string `json:"simulate_action,omitempty"`
}

type ChargeResult struct {
	Success          bool   `json:"success"`
	GatewayReference string `json:"gateway_reference"`
	ErrorCode        string `json:"error_code,omitempty"`
	ErrorMessage     string `json:"error_message,omitempty"`
}

// PaymentGateway abstracts the payment provider.
type PaymentGateway interface {
	Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
}

// Simulator implements an in-memory, deterministic, and idempotent payment gateway simulator.
type Simulator struct {
	mu           sync.RWMutex
	chargesByKey map[string]ChargeResult
}

func NewSimulator() *Simulator {
	return &Simulator{
		chargesByKey: make(map[string]ChargeResult),
	}
}

func (s *Simulator) Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidChargeAmount
	}

	// Idempotency check: if already processed with this key, return original result
	s.mu.RLock()
	if cached, ok := s.chargesByKey[req.IdempotencyKey]; ok {
		s.mu.RUnlock()
		return &cached, nil
	}
	s.mu.RUnlock()

	// Check deterministic simulation triggers
	action := req.SimulateAction
	if action == "" {
		action = req.PaymentMethodToken
	}

	var result ChargeResult

	switch action {
	case "pm_card_timeout", "timeout":
		return nil, ErrGatewayTimeout

	case "pm_card_insufficient_funds", "decline_funds":
		result = ChargeResult{
			Success:          false,
			GatewayReference: fmt.Sprintf("ch_declined_%s", uuid.NewString()[:8]),
			ErrorCode:        "insufficient_funds",
			ErrorMessage:     "The payment card has insufficient funds.",
		}

	case "pm_card_expired", "decline_expired":
		result = ChargeResult{
			Success:          false,
			GatewayReference: fmt.Sprintf("ch_declined_%s", uuid.NewString()[:8]),
			ErrorCode:        "card_expired",
			ErrorMessage:     "The payment card has expired.",
		}

	default:
		// Successful charge
		result = ChargeResult{
			Success:          true,
			GatewayReference: fmt.Sprintf("ch_sim_%s", uuid.NewString()[:8]),
		}
	}

	// Save to idempotency store if key is provided
	if req.IdempotencyKey != "" {
		s.mu.Lock()
		s.chargesByKey[req.IdempotencyKey] = result
		s.mu.Unlock()
	}

	return &result, nil
}
