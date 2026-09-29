# ADR-0006: Transactional Outbox Pattern for Merchant Webhook Delivery

## Status
Accepted

## Context
When an order is created, an installment is paid, or an account defaults, merchants must be notified promptly via HTTP webhooks.

In distributed systems, a naive implementation attempts to send the webhook directly during the HTTP request or database transaction:
1. **Dual-Write Problem**: If the database transaction commits but the HTTP request to the merchant fails or times out, the event is permanently lost.
2. **Reverse Dual-Write Problem**: If the HTTP request succeeds but the database rolls back, the merchant is falsely notified of a payment that never happened.
3. **Latency & Cascade Failures**: Slow merchant endpoints block user checkout requests.

## Decision
We implement the **Transactional Outbox Pattern** to decouple state changes from webhook delivery.

### 1. Schema & Atomic Persistence
An `outbox_events` table is created in PostgreSQL:
```sql
CREATE TABLE outbox_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    payload      JSONB NOT NULL,
    status       VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending, delivered, failed
    attempt_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NULL,
    published_at TIMESTAMPTZ NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```
Whenever an order or installment status changes, the corresponding event (e.g. `order.created`, `installment.paid`, `order.defaulted`) is inserted into `outbox_events` **within the exact same database transaction**.

### 2. Reliable Delivery Worker
An asynchronous outbox worker polls for `pending` events using `SELECT ... FOR UPDATE SKIP LOCKED`:
1. Constructs the HTTP request containing the canonical JSON event payload.
2. Signs the payload using HMAC-SHA256 with the merchant's secret key:
   ```text
   X-Ledgerly-Signature: sha256=<hex-digest>
   X-Ledgerly-Timestamp: <unix-epoch>
   ```
3. Dispatches the HTTP POST request to the merchant's registered webhook URL with a 5-second timeout.
4. On HTTP 2xx response: Marks the event as `delivered` with `published_at = NOW()`.
5. On HTTP 4xx/5xx or network timeout: Increments `attempt_count`, calculates exponential backoff (`next_retry_at = NOW() + 2^attempts * minutes`), and leaves the event for subsequent retry up to a maximum of 5 attempts.

## Consequences
- **Positive**: Guaranteed at-least-once delivery; zero coupling between merchant endpoint latency and core transaction checkout throughput; cryptographic integrity and non-repudiation via HMAC signatures.
- **Negative**: Merchants must handle at-least-once delivery idempotently using the unique `event_id` in the webhook payload.
