# ADR-0003: Idempotency Pattern, Atomic SQL Transactions, and Concurrent Retries

## Status
Accepted

## Context
In financial systems and distributed networks, client timeouts, network drops, and automated retries inevitably cause identical requests to be sent multiple times. If an operation that moves funds is re-executed, it could result in double-charging customers, double-paying merchants, or corrupting ledger integrity.

Furthermore, because `transactions` and `entries` tables have strict database-level append-only triggers preventing any `UPDATE`, idempotency handling must **never** execute an `UPDATE` on existing rows (e.g. `ON CONFLICT DO UPDATE` would fail against append-only triggers).

We must define:
1. The storage mechanism and scope for idempotency keys.
2. The canonical payload hashing algorithm.
3. The atomic database transaction ordering.
4. The exact handling of concurrent requests sharing the same idempotency key.

## Decision

### 1. Key Scope and Database Storage
- The idempotency key is scoped per API client/tenant using a composite unique constraint:
  ```sql
  client_id       VARCHAR(64)  NOT NULL,
  idempotency_key VARCHAR(255) NOT NULL,
  request_hash    CHAR(64)     NOT NULL, -- SHA-256 of canonical request payload
  CONSTRAINT uq_transactions_client_idempotency UNIQUE (client_id, idempotency_key)
  ```
- Storing the idempotency constraint directly within the `transactions` table guarantees that recording the transaction and recording the idempotency key occur in the exact same atomic PostgreSQL transaction without distributed coordination.

### 2. Canonical Payload Hashing
To prevent false mismatches caused by JSON formatting (key ordering, whitespace), the request payload is normalized before hashing:
1. Parse the request body into domain DTOs.
2. Serialize into canonical JSON with lexicographically sorted object keys and no superfluous whitespace.
3. Compute the SHA-256 hex digest:
   $$\text{request\_hash} = \text{SHA256}(\text{CanonicalJSON}(\text{Payload}))$$

### 3. Execution Order Within the Database Transaction
All steps occur within a single database transaction (`READ COMMITTED`):

```text
[HTTP Request]
       │
       ▼
 1. Compute Canonical SHA-256 Hash
       │
       ▼
 2. BEGIN SQL Transaction
       │
       ▼
 3. INSERT INTO transactions (id, client_id, idempotency_key, request_hash, ...)
    ON CONFLICT (client_id, idempotency_key) DO NOTHING
    RETURNING id;
       │
       ├─────────────────────────────────┐
       ▼ [Row Returned: New Key]         ▼ [No Row Returned: Existing Key]
 4a. Lock accounts deterministically: 4b. SELECT id, request_hash FROM transactions
     SELECT ... FOR UPDATE                WHERE client_id = $1 AND idempotency_key = $2;
     ORDER BY account_id ASC;            (PostgreSQL row lock waits if in-flight)
       │                                  │
 5a. Verify balance limits               5b. Compare request_hash:
     (negative balance policy)                ├── MATCH: Fetch entries, COMMIT read,
       │                                      │   return HTTP 200 OK (original response)
 6a. INSERT INTO entries ...                  └── MISMATCH: ROLLBACK,
       │                                          return HTTP 409 Conflict
 7a. COMMIT (triggers verify invariants)
       │
       ▼
 Return HTTP 201 Created
```

#### Detailed Execution Steps:
1. **Insert Idempotency First (`ON CONFLICT DO NOTHING`)**:
   ```sql
   INSERT INTO transactions (id, client_id, idempotency_key, request_hash, description, created_at)
   VALUES ($1, $2, $3, $4, $5, NOW())
   ON CONFLICT (client_id, idempotency_key) DO NOTHING
   RETURNING id;
   ```
   Because `DO NOTHING` does not perform an `UPDATE`, it completely respects the database-level append-only triggers.
2. **Branch on Insertion Result**:
   - **Case A: Row Returned (New Transaction)**:
     - The idempotency key is newly claimed.
     - Acquire row locks on affected accounts ordered by `account_id ASC`:
       ```sql
       SELECT id FROM accounts WHERE id = ANY($1) ORDER BY id ASC FOR UPDATE;
       ```
     - Compute post-transaction balances and enforce negative balance policies.
     - Insert all associated `entries`.
     - Commit the transaction (`COMMIT`), returning `HTTP 201 Created`.
   - **Case B: No Row Returned (Conflict / Existing Transaction)**:
     - Execute:
       ```sql
       SELECT id, request_hash, description, created_at 
       FROM transactions 
       WHERE client_id = $1 AND idempotency_key = $2;
       ```
     - Compare the retrieved `request_hash` with the current request's hash:
       - **Matching Hash**: Legitimate client retry. Retrieve the transaction's entries, commit the read transaction, and return `HTTP 200 OK` with the existing transaction data.
       - **Mismatched Hash**: The caller reused an idempotency key with different payload parameters. Abort (`ROLLBACK`) and return `HTTP 409 Conflict` with error code `IDEMPOTENCY_KEY_PAYLOAD_MISMATCH`.

### 4. Concurrent Requests with the Same Key
When two identical requests with the same `(client_id, idempotency_key)` arrive at the exact same millisecond:

1. **Conflict Lock Queuing**:
   Both concurrent database transactions execute the `INSERT ... ON CONFLICT DO NOTHING`.
   - The first transaction successfully inserts the row and holds an uncommitted insertion lock on that unique index entry.
   - The second concurrent transaction encounters the unique index conflict. PostgreSQL's internal index locking pauses the second transaction until the first transaction concludes (`COMMIT` or `ROLLBACK`).
2. **Resolution After First Transaction Concludes**:
   - **If Transaction 1 Commits**:
     The second transaction's `INSERT ... ON CONFLICT DO NOTHING` completes by doing nothing and returning 0 rows. Transaction 2 proceeds to Step 4b (`SELECT`), reads the committed transaction, verifies the hash matches, and returns `HTTP 200 OK` without duplicating any ledger entries.
   - **If Transaction 1 Rolls Back**:
     The lock is released upon rollback. The second transaction's `INSERT` executes freshly, succeeds, returns the new row ID, and processes the transaction as new.

## Consequences
- **Positive**: 100% compliant with append-only triggers; completely immune to race conditions or duplicate entries under concurrent retries; zero external lock dependencies.
- **Negative**: Reused keys with matching hashes perform a secondary `SELECT` to read existing entries, which is an acceptable O(1) query on an indexed primary key.
