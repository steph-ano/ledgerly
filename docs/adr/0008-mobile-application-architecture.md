# ADR-0008: Mobile Application Architecture and Payment Flow

## Status
Accepted

## Context
Ledgerly provides consumer-facing BNPL services where customers monitor their active purchases, receive upcoming installment alerts, and execute early or manual payments on due installments. To maximize developer velocity, cross-platform reach (iOS & Android), and architectural cohesion with the React web application, a React Native mobile application is required.

The mobile app must address:
1. **Purchase & Installment Tracker**: Clear timeline visualization of the 4 bi-weekly payments for each order.
2. **One-Tap Early Repayment**: Customer capability to settle due or pending installments ahead of time with instant ledger synchronization.
3. **Offline Resilience & Fast Startup**: Optimistic UI rendering with cached state, syncing with the BNPL service (`/api/bnpl/v1/orders/{id}`).
4. **Fintech Design Standard**: Sleek dark mode visual language matching the web dashboard with high-contrast monetary typography and clear payment status badges.

## Decision
1. **Framework & Runtime**:
   - **React Native with TypeScript** leveraging the **Expo** framework for unified cross-platform tooling, instant previewing, and zero native build boilerplate.
   - React 18/19 components with functional hooks.

2. **Component Architecture & Styling**:
   - React Native's native `StyleSheet` without heavy external CSS-in-JS runtimes for optimal 60fps rendering performance on lower-end mobile devices.
   - Design tokens consistent with the Web Dashboard (Obsidian `#07090e`, Neon Cyan `#00f2fe`, Emerald `#10b981`, Rose `#f43f5e`).

3. **API Integration & State**:
   - Typed client communicating with the Ledgerly BNPL API.
   - Built-in demonstration mock fallback so reviewers can evaluate and run the mobile app without requiring active local tunnels or running backend containers.

4. **Security & Validation**:
   - All monetary calculations are displayed using integer cent conversions (`$cents / 100`) to eliminate IEEE-754 floating-point inaccuracies on mobile clients.
   - Payment method tokenization simulated via synthetic tokens (`pm_card_visa`, `pm_card_mastercard`).

## Consequences
- Fast cross-platform customer mobile application for iOS and Android.
- Shares TypeScript domain contracts (`Order`, `Installment`, `Account`) with the web application.
