# ADR-0002: Concurrency Control, Isolation Strategy, and Race Validation

## Status
Accepted

## Context
A financial ledger must prevent concurrent anomalies such as:
1. **Double spending / Overdraft**: Multiple concurrent requests debiting an account past its permitted limit.
2. **Deadlocks**: Two concurrent transactions locking the same set of accounts in opposite order (e.g., Account A -> Account B vs Account B -> Account A).
3. **Lost Updates / Inconsistent Reads**: Reading uncommitted or obsolete account states during balance checks.

We need to establish:
- The database isolation level.
- The concurrency locking mechanism.
- The validation strategy differentiating memory races in Go from database races in PostgreSQL.

## Decision

### 1. Selected Approach: `READ COMMITTED` with Deterministic `SELECT ... FOR UPDATE`
We select **`READ COMMITTED` isolation level combined with explicit row-level pessimistic locking (`SELECT ... FOR UPDATE`) on the involved accounts, ordered deterministically by `account_id` ASC**.

#### Mechanism:
1. When a transaction request arrives affecting accounts $[A_1, A_2, \dots, A_n]$, the list of unique account IDs is sorted in lexicographical order (e.g., `A < B < C`).
2. Inside an atomic PostgreSQL transaction (`BEGIN`), the service acquires exclusive row locks on these accounts:
   ```sql
   SELECT id FROM accounts 
   WHERE id = ANY($1) 
   ORDER BY id ASC 
   FOR UPDATE;
   ```
3. Current account balances are calculated (or verified) while holding the row lock.
4. If balance constraints are satisfied (e.g., non-negative customer or merchant limits), `transactions` and `entries` are inserted.
5. The transaction is committed (`COMMIT`), releasing the locks.

#### Deadlock Freedom:
Because all concurrent database transactions acquire locks on accounts in the exact same deterministic order (`id ASC`), cycle dependencies in the lock graph are mathematically impossible. Deadlocks between ledger transactions are eliminated.

### 2. Discarded Alternative: `SERIALIZABLE` Isolation Level
We considered setting PostgreSQL's transaction isolation level to `SERIALIZABLE` without explicit row locking.
- **Reason for Discarding**:
  Under `SERIALIZABLE`, PostgreSQL uses predicate locks (SIREAD) and aborts any transaction that exhibits a serialization anomaly with SQLSTATE `40001` (`serialization_failure`).
  In a BNPL system, shared accounts (such as the `platform` clearing account or `fees` revenue account) experience high write contention. Under high throughput, `SERIALIZABLE` causes massive transaction abort storms, requiring complex exponential backoff retry loops at the application layer, degrading latency and throughput.
  In contrast, `READ COMMITTED` with deterministic `SELECT ... FOR UPDATE` queues concurrent access cleanly on the locked rows without aborting transactions or generating serialization errors.

### 3. Concurrency Validation Strategy: Memory Races vs Database Races
It is critical to distinguish between two completely distinct classes of concurrency defects:

1. **In-Memory Data Races (Go runtime level)**:
   - **Definition**: Two or more goroutines accessing the same shared memory location concurrently where at least one write occurs without synchronization (mutex, channels, atomics).
   - **Detection**: Validated using Go's built-in race detector flag (`go test -race`).
   - **Scope**: Internal Go structures, caching layers, goroutine worker pools.

2. **Database Race Conditions & Concurrency Anomalies (PostgreSQL level)**:
   - **Definition**: Interleaved database operations that could produce lost updates, phantom reads, double withdrawals, or deadlocks across independent HTTP requests/goroutines.
   - **Detection**: The Go `-race` flag **cannot** detect database races because database connections communicate via independent TCP network sockets or connection pools.
   - **Scope & Testing Strategy**: Validated through **concurrent integration tests against real PostgreSQL instances**. These tests launch high-concurrency workloads (e.g., 50–100 parallel goroutines attempting to debit the same account simultaneously) and assert that:
     - Total debits never exceed the starting balance.
     - No deadlocks occur.
     - The sum of all ledger entries precisely equals expected balance.

## Consequences
- **Positive**: Predictable FIFO queuing on hot accounts, deterministic deadlock prevention, no transaction abort storms, clear separation of testing concerns.
- **Negative**: Accounts involved in a transaction remain locked for the duration of that database transaction; database transactions must remain concise (no external network I/O while holding locks).
