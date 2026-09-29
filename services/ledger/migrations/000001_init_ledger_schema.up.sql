-- ============================================================================
-- 1. ACCOUNTS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS accounts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id   VARCHAR(64)  NOT NULL,
    type        VARCHAR(32)  NOT NULL,
    currency    CHAR(3)      NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_account_type CHECK (type IN ('customer', 'merchant', 'platform', 'fees')),
    CONSTRAINT chk_account_currency CHECK (currency ~ '^[A-Z]{3}$')
);

CREATE INDEX IF NOT EXISTS idx_accounts_client_id ON accounts (client_id);
CREATE INDEX IF NOT EXISTS idx_accounts_client_currency ON accounts (client_id, currency);

-- ============================================================================
-- 2. TRANSACTIONS TABLE (Append-Only)
-- ============================================================================
CREATE TABLE IF NOT EXISTS transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       VARCHAR(64)  NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    request_hash    CHAR(64)     NOT NULL,
    description     TEXT         NOT NULL,
    reversal_of     UUID         NULL REFERENCES transactions(id),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_transactions_client_idempotency UNIQUE (client_id, idempotency_key),
    CONSTRAINT uq_transactions_reversal_of UNIQUE (reversal_of),
    CONSTRAINT chk_transactions_request_hash CHECK (length(request_hash) = 64)
);

CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_reversal_of ON transactions (reversal_of) WHERE reversal_of IS NOT NULL;

-- ============================================================================
-- 3. ENTRIES TABLE (Append-Only Ledger)
-- ============================================================================
CREATE TABLE IF NOT EXISTS entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id  UUID         NOT NULL REFERENCES transactions(id) ON DELETE RESTRICT,
    account_id      UUID         NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    amount          BIGINT       NOT NULL,
    direction       VARCHAR(6)   NOT NULL,
    currency        CHAR(3)      NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_entries_amount CHECK (amount > 0),
    CONSTRAINT chk_entries_direction CHECK (direction IN ('DEBIT', 'CREDIT')),
    CONSTRAINT chk_entries_currency CHECK (currency ~ '^[A-Z]{3}$')
);

-- Fast balance queries and transaction entry lookups
CREATE INDEX IF NOT EXISTS idx_entries_account_created ON entries (account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_entries_transaction_id ON entries (transaction_id);

-- ============================================================================
-- 4. IMMUTABILITY TRIGGER: BLOCK UPDATE & DELETE
-- ============================================================================
CREATE OR REPLACE FUNCTION fn_block_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger immutability violation: % operation is prohibited on table %', TG_OP, TG_TABLE_NAME
        USING ERRCODE = '55000'; -- object_not_in_prerequisite_state
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_block_transactions_mutation
BEFORE UPDATE OR DELETE ON transactions
FOR EACH ROW EXECUTE FUNCTION fn_block_mutation();

CREATE TRIGGER trg_block_entries_mutation
BEFORE UPDATE OR DELETE ON entries
FOR EACH ROW EXECUTE FUNCTION fn_block_mutation();

-- ============================================================================
-- 5. CONSTRAINT TRIGGER: MINIMUM 2 ENTRIES PER TRANSACTION (DEFERRED)
-- ============================================================================
CREATE OR REPLACE FUNCTION fn_verify_transaction_min_entries()
RETURNS TRIGGER AS $$
DECLARE
    v_entry_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO v_entry_count
    FROM entries
    WHERE transaction_id = NEW.id;

    IF v_entry_count < 2 THEN
        RAISE EXCEPTION 'transaction % must have at least 2 entries, found %', NEW.id, v_entry_count
            USING ERRCODE = '23514'; -- check_violation
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_verify_transaction_min_entries
AFTER INSERT ON transactions
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fn_verify_transaction_min_entries();

-- ============================================================================
-- 6. CONSTRAINT TRIGGER: ZERO-SUM & SINGLE-CURRENCY INVARIANT (DEFERRED)
-- ============================================================================
CREATE OR REPLACE FUNCTION fn_verify_entries_balance_and_currency()
RETURNS TRIGGER AS $$
DECLARE
    v_balance_diff          BIGINT;
    v_currency_count        INTEGER;
    v_mismatched_accounts   INTEGER;
BEGIN
    -- 1. Verify sum(DEBIT) - sum(CREDIT) = 0
    -- 2. Verify single currency across all entries for the transaction
    SELECT 
        COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE -amount END), 0),
        COUNT(DISTINCT currency)
    INTO v_balance_diff, v_currency_count
    FROM entries
    WHERE transaction_id = NEW.transaction_id;

    IF v_balance_diff <> 0 THEN
        RAISE EXCEPTION 'transaction % is unbalanced: debit minus credit diff is % (must be 0)', NEW.transaction_id, v_balance_diff
            USING ERRCODE = '23514';
    END IF;

    IF v_currency_count <> 1 THEN
        RAISE EXCEPTION 'transaction % has % distinct currencies, exactly 1 allowed', NEW.transaction_id, v_currency_count
            USING ERRCODE = '23514';
    END IF;

    -- 3. Verify that entries.currency matches accounts.currency for all involved entries
    SELECT COUNT(*) INTO v_mismatched_accounts
    FROM entries e
    JOIN accounts a ON e.account_id = a.id
    WHERE e.transaction_id = NEW.transaction_id
      AND e.currency <> a.currency;

    IF v_mismatched_accounts > 0 THEN
        RAISE EXCEPTION 'transaction % contains entries with currency mismatching account currency', NEW.transaction_id
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_verify_entries_balance_and_currency
AFTER INSERT ON entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION fn_verify_entries_balance_and_currency();
