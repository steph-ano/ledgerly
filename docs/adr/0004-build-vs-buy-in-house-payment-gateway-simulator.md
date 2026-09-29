# ADR-0004: Build vs Buy — In-House Payment Gateway Simulator

## Status
Accepted

## Context
A Buy Now, Pay Later (BNPL) platform relies fundamentally on charging consumer payment instruments (credit cards, debit cards, bank debits) through payment gateways for down payments and recurring installments.

During automated CI/CD testing, local development, and technical interview demonstrations, testing against third-party sandbox environments (such as Stripe Test Mode) presents significant drawbacks:
1. **Flakiness & Network Dependence**: Tests fail when external sandboxes experience latency or outages.
2. **Rate Limiting**: Automated parallel integration and load tests easily trip external sandbox rate limits.
3. **Impossibility of Deterministic Chaos Simulation**: Simulating edge cases like specific network timeouts during gateway capture, transient socket drops, or idempotent webhook replay is difficult or impossible to control deterministically via external test cards.
4. **Credential Leak Risks**: Requires managing API keys in CI/CD secrets.

Following our core engineering principle:
> *"Open source first and build before buy: do not use paid or external services if a reasonable version can be built. Whenever something is built instead of bought, document why in an ADR."*

## Decision
We will build a high-fidelity, in-house **Payment Gateway Simulator** embedded within the platform.

### Capabilities:
1. **Deterministic Response Triggers**:
   - The simulator inspects payment method tokens, custom request headers (e.g., `X-Simulate-Result: decline_insufficient_funds`), or amount thresholds to simulate:
     - `SUCCESS`
     - `DECLINED_INSUFFICIENT_FUNDS`
     - `DECLINED_CARD_EXPIRED`
     - `GATEWAY_TIMEOUT` (transient 504 / network fault)
     - `CARD_BLOCKED`
2. **Strict Idempotency**:
   - The gateway maintains an internal idempotency store to verify that re-charging an installment with the same idempotency key returns the original charge ID and does not create duplicate debit attempts.
3. **Pluggable Interface**:
   - The domain depends exclusively on an abstract `PaymentGateway` interface:
     ```go
     type PaymentGateway interface {
         Charge(ctx context.Context, req ChargeRequest) (*ChargeResult, error)
     }
     ```
   - In production, this can be swapped with a real Stripe, Adyen, or Sezzle adapter without altering a single line of domain or scheduler logic.

## Consequences
- **Positive**: 100% offline, lightning-fast test execution (<10ms per charge); deterministic chaos testing for retry backoff and double-charge prevention; zero external API dependencies or secret leakage.
- **Negative**: The simulator must accurately model real-world gateway behaviors (charge IDs, failure codes, and timeout semantics).
