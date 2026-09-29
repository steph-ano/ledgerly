-- Enable UUID extension if not present
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- 1. ORDERS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS orders (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id             VARCHAR(64)  NOT NULL,
    customer_account_id   UUID         NOT NULL,
    merchant_account_id   UUID         NOT NULL,
    total_amount          BIGINT       NOT NULL,
    currency              CHAR(3)      NOT NULL,
    status                VARCHAR(32)  NOT NULL,
    merchant_webhook_url  TEXT         NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_orders_total_amount CHECK (total_amount > 0),
    CONSTRAINT chk_orders_currency CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_orders_status CHECK (status IN ('pending', 'active', 'completed', 'defaulted', 'canceled'))
);

CREATE INDEX IF NOT EXISTS idx_orders_client_id ON orders (client_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);

-- ============================================================================
-- 2. INSTALLMENTS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS installments (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       UUID         NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    number         INT          NOT NULL,
    amount         BIGINT       NOT NULL,
    currency       CHAR(3)      NOT NULL,
    due_date       TIMESTAMPTZ  NOT NULL,
    status         VARCHAR(32)  NOT NULL,
    attempt_count  INT          NOT NULL DEFAULT 0,
    next_retry_at  TIMESTAMPTZ  NULL,
    locked_at      TIMESTAMPTZ  NULL,
    paid_at        TIMESTAMPTZ  NULL,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_order_installment_number UNIQUE (order_id, number),
    CONSTRAINT chk_installment_number CHECK (number BETWEEN 1 AND 4),
    CONSTRAINT chk_installment_amount CHECK (amount > 0),
    CONSTRAINT chk_installment_currency CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT chk_installment_status CHECK (status IN ('pending', 'processing', 'retrying', 'paid', 'failed'))
);

-- Index for the scheduler using SKIP LOCKED
CREATE INDEX IF NOT EXISTS idx_installments_scheduler 
ON installments (due_date ASC, next_retry_at ASC) 
WHERE status IN ('pending', 'retrying');

-- ============================================================================
-- 3. PAYMENT ATTEMPTS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS payment_attempts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    installment_id    UUID         NOT NULL REFERENCES installments(id) ON DELETE CASCADE,
    gateway_reference VARCHAR(255) NOT NULL,
    result            VARCHAR(32)  NOT NULL,
    error_code        VARCHAR(64)  NULL,
    error_message     TEXT         NULL,
    attempt_number    INT          NOT NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_payment_attempt_result CHECK (result IN ('success', 'declined', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_payment_attempts_installment ON payment_attempts (installment_id);

-- ============================================================================
-- 4. OUTBOX EVENTS TABLE (Transactional Outbox Pattern)
-- ============================================================================
CREATE TABLE IF NOT EXISTS outbox_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type      VARCHAR(64)  NOT NULL,
    aggregate_id    UUID         NOT NULL,
    destination_url TEXT         NOT NULL,
    payload         JSONB        NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    attempt_count   INT          NOT NULL DEFAULT 0,
    next_retry_at   TIMESTAMPTZ  NULL,
    published_at    TIMESTAMPTZ  NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_outbox_status CHECK (status IN ('pending', 'delivered', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_pending 
ON outbox_events (created_at ASC) 
WHERE status = 'pending';
