# ADR-0001: Double-Entry Invariants, Money Representation, and Immutability

## Status
Accepted

## Context
Ledgerly is a Buy Now, Pay Later (BNPL) financial platform requiring strict financial integrity, zero data loss, and an indisputable audit trail. In financial ledgers, software defects such as floating-point rounding errors, unbalanced transactions, or modified historical records destroy regulatory compliance and financial consistency.

We must define:
1. Representation of monetary values.
2. Invariants of the double-entry accounting system.
3. Immutability guarantees.
4. Calculation of account balances and negative balance policies by account type.

## Decision

### 1. Money Representation
- Monetary amounts are strictly represented as signed or unsigned 64-bit integers (`int64`) in the currency's smallest atomic unit (e.g., cents for USD/EUR, meaning $10.50 is represented as `1050`).
- Floating-point types (`float32`, `float64`) are **strictly prohibited** in domain entities, storage models, and database columns.
- All monetary operations (addition, subtraction, multiplication) must be performed using explicit overflow-checked arithmetic (e.g., testing against `math.MaxInt64` and `math.MinInt64`) returning `ErrAmountOverflow` on violation.
- Every monetary amount is paired with an ISO 4217 three-letter currency code (e.g., `USD`).

### 2. Double-Entry Accounting Core Invariant
- A financial movement is recorded as a `Transaction` consisting of two or more `Entry` records.
- Each `Entry` specifies:
  - `account_id`: The target account.
  - `amount`: A positive integer (`amount > 0`).
  - `direction`: An enum with values `DEBIT` or `CREDIT`.
  - `currency`: Matching the transaction and account currency.
- **Zero-Sum Transaction Invariant**:
  $$\sum \text{Debits} - \sum \text{Credits} = 0$$
  Every transaction must balance to zero across all its entries. A transaction with unequal debits and credits is invalid and rejected.
- **Single Currency per Transaction Invariant**: All entries in a single transaction must share the identical currency code. Multi-currency transactions are out of scope for the MVP.
- **Database-Level Enforcement**: In addition to application-level domain validation, PostgreSQL will enforce these invariants using a `CONSTRAINT TRIGGER ... DEFERRABLE INITIALLY DEFERRED` on the `entries` table, executed at `COMMIT` time.

### 3. Append-Only Immutability
- The `transactions` and `entries` tables are strictly **append-only**.
- Records in `transactions` and `entries` must **never** be updated or deleted.
- PostgreSQL database triggers (`BEFORE UPDATE OR DELETE`) will explicitly raise an exception if any update or deletion is attempted on these tables.
- Corrections, disputes, or cancellations are recorded exclusively as new balancing **reversal transactions**.
- **Reversal Uniqueness**: A transaction can be reversed at most once. The column `transactions.reversal_of` is constrained with a `UNIQUE` index (allowing nulls), preventing double reversals.

### 4. Account Balance Calculation & Snapshot Roadmap
- **Current Strategy (MVP)**: Account balances are computed dynamically using `SUM(entries)` grouped by account ID:
  $$\text{Balance} = \sum \text{Debits} - \sum \text{Credits}$$
  (or inverted for liability/revenue accounts depending on their normal balance).
- **Future Improvement (Snapshotting)**: As ledger entry volume grows into millions, periodic balance snapshots (e.g., daily checkpoint tables or materialized balance projection tables) will be introduced. The append-only invariant on `transactions` and `entries` will remain unchanged.

### 5. Negative Balance Policy by Account Type
Accounts have distinct economic semantics and balance constraints:
1. **`customer` (Customer Receivables / Loan Account - Asset)**:
   - Normal Balance: `DEBIT` (represents the outstanding principal owed by the customer).
   - Policy: Debit balance $\ge 0$. A customer cannot have a negative receivable balance (over-crediting beyond the debt is rejected or routed to customer credit liabilities).
2. **`merchant` (Merchant Payables - Liability)**:
   - Normal Balance: `CREDIT` (represents funds owed by the platform to the merchant).
   - Policy: Credit balance $\ge 0$. The platform cannot pay out more than what is owed to the merchant; overdrafts are rejected.
3. **`platform` (Operational / Settlement Cash - Asset)**:
   - Normal Balance: `DEBIT` (platform cash reserves).
   - Policy: Controlled deficit allowed during settlement windows, but flagged if exceeding operational overdraft limits.
4. **`fees` (Platform Revenue - Revenue/Income)**:
   - Normal Balance: `CREDIT` (cumulative earned fees).
   - Policy: Credit balance $\ge 0$. Fee reversals cannot reduce earned revenue below zero.

## Consequences
- **Positive**: Absolute auditability; mathematically verified ledger balance; zero floating-point discrepancies; compliance with standard accounting practices.
- **Negative**: Dynamic balance queries via `SUM(entries)` require database indexes on `(account_id, created_at)` and will eventually require snapshot aggregation for high-volume accounts.
