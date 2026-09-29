# ADR-0007: Web Dashboard Architecture, State Management, and Design System

## Status
Accepted

## Context
Ledgerly requires a frontend application to demonstrate the end-to-end BNPL lifecycle, provide operator observability into the immutable double-entry ledger, and simulate consumer checkout interactions. The target audience includes prospective employers, fintech interviewers (evaluating systems knowledge around concurrency, idempotency, and financial integrity), and engineers testing the platform.

The web application must fulfill three core user experiences:
1. **Interactive BNPL Checkout Simulator (Consumer Persona)**:
   - Dynamic 4-installment split calculation with calendar payment schedule.
   - Interactive card simulator selector (`pm_card_visa` for immediate success, `pm_card_insufficient_funds` for declined down payment, `pm_card_timeout` for gateway failure handling).
   - Real-time submission via `POST /v1/orders`.
2. **Merchant & Operator Console (Admin Persona)**:
   - Order pipeline monitoring (`active`, `completed`, `defaulted`).
   - Installment drill-down with visual retry backoff badges (+2h, +12h, +24h, terminal fail).
   - Installment manual payment trigger (`POST /v1/installments/{id}/pay`).
   - Transaction reversal trigger demonstrating ledger balancing entries.
3. **Double-Entry Ledger Audit Explorer**:
   - Live account balance monitor ($\sum \text{Debits} - \sum \text{Credits}$).
   - Balanced transaction ledger inspector displaying journal entries with strict debit/credit balance indicators.
4. **Webhook Event Log & Signature Inspector**:
   - Stream of Transactional Outbox events with HMAC-SHA256 signatures and timestamp verification.

## Decision
1. **Framework & Tooling**:
   - **React 18/19 with TypeScript** bootstrapped via **Vite** for sub-second hot module reloading and fast build times.
   - **Vanilla CSS with Custom Properties (Design System)**: No TailwindCSS dependencies; modular CSS tokens for typography, dark mode palettes, elevations, and glassmorphic surfaces.

2. **Design Aesthetic & Visual Language**:
   - **Fintech Dark Mode**: Curated deep slate `#090d16` and obsidian backgrounds, high-contrast borders (`rgba(255,255,255,0.08)`), and electric cyan/emerald accents (`#00f2fe`, `#10b981`, `#6366f1`).
   - **Typography**: Inter / Outfit modern geometric sans-serif for clean numerical clarity.
   - **Tabular Numerics**: Strict `font-variant-numeric: tabular-nums` for all financial figures, ensuring monetary values align cleanly.

3. **Backend Communication & Proxying**:
   - Development proxy configured in `vite.config.ts` mapping:
     - `/api/ledger/*` $\to$ `http://localhost:8080/`
     - `/api/bnpl/*` $\to$ `http://localhost:8081/`
   - Client API service layer with built-in fallback mock mode: If the backend Go containers are not currently running during static evaluation, the UI seamlessly falls back to a realistic in-browser simulated state so evaluators can inspect every screen without needing local Docker.

4. **State Management**:
   - Lightweight reactive state using React hooks and context (`useLedgerStore` / custom reducers) without heavyweight external libraries.
   - Optimistic UI updates with automatic reconciliation upon API response.

## Consequences
- Single cohesive web application that can run connected to the live Go microservices or autonomously in demonstration mode.
- Clear, interview-ready presentation of double-entry ledger mechanics and BNPL scheduling.
