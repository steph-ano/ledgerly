# ADR-0003: Idempotency Pattern and Concurrent Request Handling

## Status
Accepted

## Context
In financial systems and distributed networks, client timeouts, network drops, and automated retries inevitably cause identical requests to be sent multiple times. If an operation that moves funds is re-executed, it could result in double-charging customers, double-paying merchants, or corrupting ledger integrity.

We must define:
1. The storage mechanism for idempotency keys.
2. The atomic relationship between idempotency tracking and ledger state.
3. Behavior when identical requests arrive concurrently.
4. Behavior when the same idempotency key is reused with a different payload.

## Decision

### 1. Unified Atomic Transaction Storage
- The idempotency record is stored in the **exact same PostgreSQL database transaction** as the financial transaction and its ledger entries.
- We do not use an external cache (like Redis) as the source of truth for financial idempotency, because a two-phase failure between Redis and PostgreSQL could leave the system in an inconsistent state.
- The `transactions` table includes an `idempotency_key` column with a `UNIQUE` constraint:
  ```sql
  idempotency_key VARCHAR(255) NOT NULL,
  request_hash    CHAR(64)     NOT NULL, -- SHA-256 of canonical request payload
  ```
  Or a dedicated `idempotency_keys` table if multi-endpoint operations require it. In the ledger core, binding `idempotency_key` and `request_hash` directly into the `transactions` record ensures zero cross-table synchronization latency.

### 2. Request Processing Flow & `INSERT ... ON CONFLICT`
1. Compute the cryptographic SHA-256 hash of the normalized request payload:
   $$\text{Hash} = \text{SHA256}(\text{CanonicalJSON}(\text{Payload}))$$
2. Begin database transaction (`BEGIN`).
3. Attempt to insert or lock the idempotency record:
   ```sql
   INSERT INTO transactions (id, idempotency_key, request_hash, description, created_at)
   VALUES ($1, $2, $3, $4, NOW())
   ON CONFLICT (idempotency_key) DO UPDATE
   SET idempotency_key = EXCLUDED.idempotency_key -- no-op update to acquire row lock
   RETURNING id, request_hash, (xmin = pg_current_xact_id()) AS is_new_record;
   ```
4. **Evaluate Result**:
   - **Case A: New Transaction (`is_new_record = true`)**:
     The key has never been used. Lock accounts (`SELECT ... FOR UPDATE`), validate balances, insert entries, and commit. Return `HTTP 201 Created` with the newly created transaction.
   - **Case B: Completed Prior Transaction (`is_new_record = false`)**:
     - Compare the existing record's `request_hash` with the incoming request's hash:
       - **Same Hash**: The request is a legitimate retry. Retrieve the previously committed transaction and its entries, commit/rollback the read transaction, and return `HTTP 200 OK` with the original response data.
       - **Different Hash**: The caller attempted to reuse an existing idempotency key for a different operation. Rollback immediately and return `HTTP 409 Conflict` (with error code `IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`).

### 3. Handling Concurrent Requests with the Same Idempotency Key
What happens when two identical requests with the same idempotency key arrive at the exact same millisecond across two different instances or goroutines?

1. **Row-Level Lock Queuing**:
   Both requests execute the `INSERT ... ON CONFLICT DO UPDATE` statement inside their respective PostgreSQL transactions.
2. The first request acquires the row lock for the `idempotency_key`.
3. The second concurrent request is **blocked** by PostgreSQL's row-level lock on that specific key until the first transaction finishes (`COMMIT` or `ROLLBACK`).
4. **Resolution**:
   - If Request 1 succeeds and commits: Request 2 unblocks, observes the committed record (`is_new_record = false`), verifies the hash matches, and returns the committed transaction (`HTTP 200 OK`) without repeating any balance movement or entry creation.
   - If Request 1 fails (e.g. database error, rollback): Request 2 unblocks, observes the lock released by rollback, acquires the key as new (`is_new_record = true`), and processes the transaction cleanly.

## Consequences
- **Positive**: Complete atomic safety; absolute zero chance of double execution even under millisecond-level concurrent retries; no external distributed lock manager required.
- **Negative**: Concurrent identical requests wait for the active transaction to commit. Because ledger transactions are short-lived memory/database writes without external HTTP calls, lock wait times are negligible (<5ms).
