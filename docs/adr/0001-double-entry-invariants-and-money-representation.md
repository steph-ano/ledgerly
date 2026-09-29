# ADR-0001: Double-Entry Invariants, Money Representation, and Immutability

## Status
Accepted

## Context
Ledgerly is a Buy Now, Pay Later (BNPL) financial platform requiring strict financial integrity, zero data loss, and an indisputable audit trail. In financial ledgers, software defects such as floating-point rounding errors, unbalanced transactions, or modified historical records destroy regulatory compliance and financial consistency.

We must define:
1. Representation of monetary values.
2. Invariants of the double-entry accounting system and their database enforcement.
3. Immutability guarantees via database triggers.
4. Calculation of account balances and negative balance policies by account type, including behavior under reversals.

## Decision

### 1. Money Representation
- Monetary amounts are strictly represented as 64-bit integers (`int64`) in the currency's smallest atomic unit (e.g., cents for USD/EUR: $10.50 is represented as `1050`).
- Floating-point types (`float32`, `float64`) are **strictly prohibited** in domain entities, storage models, and database columns.
- All monetary operations (addition, subtraction, multiplication) must be performed using explicit overflow-checked arithmetic (testing against `math.MaxInt64` and `math.MinInt64`) returning `ErrAmountOverflow` on violation.
- Every monetary amount is paired with an ISO 4217 three-letter currency code (e.g., `USD`).

### 2. Double-Entry Accounting Core Invariants & Database Enforcement
- A financial movement is recorded as a `Transaction` consisting of two or more `Entry` records.
- Each `Entry` specifies:
  - `account_id`: The target account.
  - `amount`: A positive integer (`amount > 0`).
  - `direction`: An enum with values `DEBIT` or `CREDIT`.
  - `currency`: Matching the transaction and account currency.
- **Zero-Sum Transaction Invariant**:
  $$\sum \text{Debits} - \sum \text{Credits} = 0$$
  Every transaction must balance to zero across all its entries.
- **Single Currency Invariant**: All entries in a single transaction must share the identical currency code.
- **Database-Level Constraint Triggers**:
  In addition to domain-layer validation, PostgreSQL guarantees integrity at `COMMIT` time using two `DEFERRABLE INITIALLY DEFERRED` constraint triggers:
  1. **Balance & Currency Trigger on `entries`**: Executes on `entries` changes; verifies at transaction commit that $\sum \text{Debits} = \sum \text{Credits}$ for every affected `transaction_id`, and that all entries share a single distinct `currency`.
  2. **Minimum Entries Trigger on `transactions`**: Executes `AFTER INSERT ON transactions`; verifies at transaction commit that the newly inserted transaction has at least 2 entries associated with it (`COUNT(entries) >= 2`), preventing orphaned or empty transaction records.

### 3. Append-Only Immutability
- The `transactions` and `entries` tables are strictly **append-only**.
- Records in `transactions` and `entries` must **never** be updated or deleted.
- PostgreSQL database triggers (`BEFORE UPDATE OR DELETE`) explicitly raise an exception (`RAISE EXCEPTION 'ledger records are immutable'`) if any modification or deletion is attempted.
- Corrections, disputes, or cancellations are recorded exclusively as new balancing **reversal transactions**.
- **Reversal Uniqueness**: A transaction can be reversed at most once. The column `transactions.reversal_of` is constrained with a `UNIQUE` index (allowing nulls), preventing duplicate reversals at the database constraint level.

### 4. Account Balance Calculation & Snapshot Roadmap
- **Current Strategy (MVP)**: Account balances are computed dynamically using `SUM(entries)` grouped by account ID:
  $$\text{Balance} = \sum \text{Debits} - \sum \text{Credits}$$
- **Future Improvement (Snapshotting)**: Periodic balance snapshots (e.g., daily checkpoint tables or materialized balance projections) will be introduced as entry volume grows. The append-only invariant on `transactions` and `entries` remains unchanged.

### 5. Negative Balance Policy Verification & Explicit Limits
The negative balance policy is **enforced in the application service layer** inside the database transaction, immediately after acquiring pessimistic row locks (`SELECT ... FOR UPDATE`) on the involved accounts and computing the projected post-transaction balances, prior to committing.

Policies by account type:
1. **`customer` (Customer Receivables / Loan Principal - Asset)**:
   - Normal Balance: `DEBIT` (outstanding loan balance owed to platform).
   - Policy: Debit balance $\ge 0$. A customer cannot have a negative receivable balance (over-crediting beyond the debt is rejected).
2. **`merchant` (Merchant Payables - Liability)**:
   - Normal Balance: `CREDIT` (funds owed by platform to merchant).
   - Policy: Credit balance $\ge 0$. Payouts or debits exceeding the merchant's accrued credit balance are rejected (`ErrInsufficientFunds`).
3. **`platform` (Operational Liquidity / Settlement Cash - Asset)**:
   - Normal Balance: `DEBIT` (platform cash reserves).
   - Policy: Explicit operational liquidity floor. Default limit: `PLATFORM_MIN_BALANCE = 0 cents` (no unauthorized overdraft). If an overdraft credit line is configured in platform settings, the floor is explicitly bounded by `-PLATFORM_OVERDRAFT_LIMIT_CENTS`. Any transaction attempting to breach this floor is rejected.
4. **`fees` (Platform Revenue - Revenue/Income)**:
   - Normal Balance: `CREDIT` (earned fee revenue).
   - Policy: Credit balance $\ge 0$. Fee reversals cannot reduce earned revenue below zero.

### 6. Policy Behavior Under Reversals
- Reversal transactions are subject to the **exact same negative balance policy checks** as standard transactions.
- If reversing an original transaction would cause any affected account to violate its non-negative balance policy (for example, attempting to reverse a customer payment after merchant funds have already been disbursed, leaving the merchant with a negative payable balance), the reversal is **rejected** with `ErrInsufficientFundsForReversal`.
- This guarantees that a reversal never creates an unbacked deficit or hidden insolvency in any account.

## Consequences
- **Positive**: Cryptographically and mathematically sound financial records; defense-in-depth via deferred triggers and service-level pre-commit checks; explicit guardrails against unauthorized overdrafts.
- **Negative**: Dynamic balance calculation queries require indexing on `(account_id, created_at)`. Reversals may be blocked if counterparty accounts lack sufficient funds to cover the refund.
