package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gitlab.com/steph-ano/ledgerly/services/bnpl/internal/storage/postgres"
)

type Config struct {
	PollInterval time.Duration
	BatchSize    int
	SigningKey   string
	Timeout      time.Duration
}

func DefaultConfig(signingKey string) Config {
	if signingKey == "" {
		signingKey = "ledgerly_webhook_default_secret_key"
	}
	return Config{
		PollInterval: 3 * time.Second,
		BatchSize:    25,
		SigningKey:   signingKey,
		Timeout:      5 * time.Second,
	}
}

type Dispatcher struct {
	cfg        Config
	outboxRepo *postgres.OutboxRepository
	httpClient *http.Client
}

func NewDispatcher(cfg Config, outboxRepo *postgres.OutboxRepository) *Dispatcher {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 25
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 3 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.SigningKey == "" {
		cfg.SigningKey = "ledgerly_webhook_default_secret_key"
	}

	return &Dispatcher{
		cfg:        cfg,
		outboxRepo: outboxRepo,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

// ComputeSignature calculates HMAC-SHA256 of payload.
func ComputeSignature(payload []byte, secret string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// ProcessPendingEvents executes one round of dispatching pending outbox events.
func (d *Dispatcher) ProcessPendingEvents(ctx context.Context, now time.Time) (int, error) {
	events, err := d.outboxRepo.LeasePendingEvents(ctx, d.cfg.BatchSize, now)
	if err != nil {
		return 0, fmt.Errorf("failed to lease outbox events: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	for _, event := range events {
		d.dispatchSingleEvent(ctx, event, now)
	}

	return len(events), nil
}

func (d *Dispatcher) dispatchSingleEvent(ctx context.Context, event postgres.OutboxEvent, now time.Time) {
	ts := now.Unix()
	sig := ComputeSignature(event.Payload, d.cfg.SigningKey, ts)

	req, err := http.NewRequestWithContext(ctx, "POST", event.DestinationURL, bytes.NewReader(event.Payload))
	if err != nil {
		slog.Error("failed to construct webhook request", "event_id", event.ID, "error", err)
		_ = d.outboxRepo.MarkFailed(ctx, event.ID, now, event.AttemptCount+1)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ledgerly-Event-Type", event.EventType)
	req.Header.Set("X-Ledgerly-Event-ID", event.ID.String())
	req.Header.Set("X-Ledgerly-Timestamp", fmt.Sprintf("%d", ts))
	req.Header.Set("X-Ledgerly-Signature", fmt.Sprintf("sha256=%s", sig))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		slog.Warn("webhook delivery network error", "event_id", event.ID, "url", event.DestinationURL, "error", err)
		_ = d.outboxRepo.MarkFailed(ctx, event.ID, now, event.AttemptCount+1)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = d.outboxRepo.MarkDelivered(ctx, event.ID, now)
		slog.Info("webhook successfully delivered", "event_id", event.ID, "status", resp.StatusCode)
	} else {
		slog.Warn("webhook destination returned non-2xx status", "event_id", event.ID, "status", resp.StatusCode)
		_ = d.outboxRepo.MarkFailed(ctx, event.ID, now, event.AttemptCount+1)
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			_, _ = d.ProcessPendingEvents(ctx, now)
		}
	}
}
