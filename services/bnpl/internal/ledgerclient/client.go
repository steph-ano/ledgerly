package ledgerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

var (
	ErrLedgerUnavailable     = errors.New("ledger service is currently unavailable")
	ErrLedgerPolicyViolation = errors.New("ledger financial policy violation (e.g. insufficient funds)")
	ErrIdempotencyMismatch    = errors.New("ledger idempotency key conflict with mismatched payload")
)

type Direction string

const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

type EntryDTO struct {
	AccountID uuid.UUID `json:"account_id"`
	Amount    int64     `json:"amount"` // in cents
	Direction Direction `json:"direction"`
	Currency  string    `json:"currency"`
}

type RecordTxRequest struct {
	Description string     `json:"description"`
	Entries     []EntryDTO `json:"entries"`
}

type TransactionResponse struct {
	ID             uuid.UUID  `json:"id"`
	ClientID       string     `json:"client_id"`
	IdempotencyKey string     `json:"idempotency_key"`
	Description    string     `json:"description"`
	ReversalOf     *uuid.UUID `json:"reversal_of,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Client communicates with the Ledger microservice via HTTP.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// RecordTransaction sends a balanced double-entry transaction to the Ledger service with an idempotency key.
func (c *Client) RecordTransaction(
	ctx context.Context,
	clientID string,
	idempotencyKey string,
	description string,
	entries []EntryDTO,
) (*TransactionResponse, error) {
	reqBody := RecordTxRequest{
		Description: description,
		Entries:     entries,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ledger request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/transactions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	req.Header.Set("X-Client-ID", clientID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLedgerUnavailable, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var txResp TransactionResponse
		if err := json.Unmarshal(respBody, &txResp); err != nil {
			return nil, fmt.Errorf("failed to parse ledger response: %w", err)
		}
		return &txResp, nil

	case http.StatusConflict:
		return nil, ErrIdempotencyMismatch

	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("%w: %s", ErrLedgerPolicyViolation, string(respBody))

	default:
		return nil, fmt.Errorf("ledger returned unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
}
