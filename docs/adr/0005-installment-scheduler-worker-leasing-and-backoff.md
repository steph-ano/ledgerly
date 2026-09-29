# ADR-0005: Installment Scheduler, Worker Leasing, and Exponential Backoff Retries

## Status
Accepted

## Context
In a BNPL system, the scheduler is responsible for periodically collecting due installments (e.g. bi-weekly payments). The scheduler must:
1. Prevent **double charging** customers under horizontally scaled worker instances.
2. Gracefully handle payment failures (e.g. temporary card declines or network hiccups) using exponential backoff without starving the queue.
3. Eliminate race conditions between automated scheduled payments and manual customer payments initiated via web or mobile apps.

## Decision

### 1. Worker Leasing via `SELECT ... FOR UPDATE SKIP LOCKED`
To distribute due installments across multiple parallel worker instances without centralized distributed lock coordinators (like Redis Redlock):
```sql
SELECT id, order_id, number, amount, currency, due_date, status, attempt_count
FROM installments
WHERE status IN ('pending', 'retrying')
  AND due_date <= NOW()
  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY due_date ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;
```

#### Why `SKIP LOCKED`?
- Standard `FOR UPDATE` would block other workers waiting on the locked rows.
- `SKIP LOCKED` instructs PostgreSQL to immediately skip rows already locked by another running worker and return only free due rows.
- Workers run fully in parallel with zero lock contention and zero duplicate processing.

### 2. State Transition & Concurrency Safety
1. Immediately upon leasing an installment inside the database transaction, its status transitions to `processing`.
2. The transaction commits, releasing the row lock, while holding an in-flight lease timestamp (`locked_at = NOW()`).
3. If a customer attempts to pay the installment manually via the mobile or web app while `status = 'processing'`, the manual attempt is rejected with `HTTP 409 Conflict: payment_in_progress`, eliminating race conditions between the customer and the scheduler.

### 3. Exponential Backoff Policy & Failure Handling
When a payment attempt fails due to a soft decline (e.g. insufficient funds, transient timeout):
- **Attempt 1**: Re-scheduled for $+2\text{ hours}$.
- **Attempt 2**: Re-scheduled for $+12\text{ hours}$.
- **Attempt 3**: Re-scheduled for $+24\text{ hours}$.
- A random jitter ($\pm 10\%$) is added to prevent thundering herd spikes on the payment gateway.
- Status is set to `retrying` with `next_retry_at` updated.
- If **Attempt 4** fails (max attempts exhausted):
  - Installment transitions to `failed`.
  - The parent `Order` transitions to `defaulted`.
  - A webhook event `order.defaulted` is queued in the transactional outbox.

### 4. Successful Payment Flow
When an attempt succeeds:
1. An idempotent ledger transaction is recorded in the Ledger service:
   - Debit: `platform` cash account (funds collected from customer card).
   - Credit: `customer` receivable account (reducing remaining loan debt).
2. Installment transitions to `paid`.
3. If all 4 installments for the order are `paid`, the parent `Order` transitions to `completed`.
4. A webhook event `installment.paid` (and `order.completed` if final) is written to the outbox.

## Consequences
- **Positive**: Guaranteed zero double charges; horizontal scalability across any number of Kubernetes worker pods; resilient automatic recovery from transient card declines.
- **Negative**: Workers must handle orphan `processing` rows if a pod crashes mid-execution (handled via a reaper routine resetting leases older than 10 minutes).
