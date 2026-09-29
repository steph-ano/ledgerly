import React, { useState, useMemo } from 'react'
import { Calendar, CreditCard, CheckCircle2, AlertTriangle, ArrowRight, Zap, RefreshCw, ShoppingCart } from 'lucide-react'
import type { Order } from '../types'
import { api } from '../api/client'
import './CheckoutSimulator.css'

interface CheckoutSimulatorProps {
  onOrderCreated: (order: Order) => void
  onViewOrders: () => void
}

export const CheckoutSimulator: React.FC<CheckoutSimulatorProps> = ({
  onOrderCreated,
  onViewOrders,
}) => {
  const [totalDollars, setTotalDollars] = useState<number>(100.01)
  const [cardToken, setCardToken] = useState<string>('pm_card_visa')
  const [isLoading, setIsLoading] = useState<boolean>(false)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const [createdOrder, setCreatedOrder] = useState<Order | null>(null)

  const totalCents = Math.round(totalDollars * 100)

  // Calculate the 4 installments down to the exact cent
  const installmentsPreview = useMemo(() => {
    const base = Math.floor(totalCents / 4)
    const remainder = totalCents % 4
    const today = new Date()

    return [1, 2, 3, 4].map((num) => {
      const amountCents = num === 1 ? base + remainder : base
      const dueDate = new Date(today.getTime() + (num - 1) * 14 * 86400000)
      return {
        number: num,
        amountCents,
        date: dueDate.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' }),
        isDownPayment: num === 1,
      }
    })
  }, [totalCents])

  const merchantFeeCents = Math.round(totalCents * 0.05)
  const merchantPayoutCents = totalCents - merchantFeeCents

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setErrorMsg(null)
    setCreatedOrder(null)

    try {
      const order = await api.createOrder({
        totalAmount: totalCents,
        currency: 'USD',
        paymentMethodToken: cardToken,
        customerAccountId: '44444444-4444-4444-4444-444444444444',
        merchantAccountId: '33333333-3333-3333-3333-333333333333',
        merchantWebhookUrl: 'https://demo-store.nordic.test/webhooks/ledgerly',
      })
      setCreatedOrder(order)
      onOrderCreated(order)
    } catch (err: unknown) {
      if (err instanceof Error) {
        setErrorMsg(err.message)
      } else {
        setErrorMsg('An unexpected error occurred')
      }
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="checkout-container animate-fade-in">
      <div className="checkout-header">
        <div className="badge-tag">
          <Zap size={14} /> Interactive Consumer Simulator
        </div>
        <h1>Buy Now, Pay in 4 Bi-Weekly Installments</h1>
        <p className="subtitle">
          Experience the complete lifecycle: deterministic cent division, instant down payment debit,
          merchant payout, and immutable zero-sum ledger settlement.
        </p>
      </div>

      <div className="checkout-grid">
        {/* Left column: Checkout form */}
        <div className="checkout-card main-form">
          <form onSubmit={handleSubmit}>
            <div className="form-section">
              <label className="section-label">1. Enter Purchase Amount</label>
              <div className="amount-input-wrapper">
                <span className="currency-prefix">$</span>
                <input
                  type="number"
                  step="0.01"
                  min="10.00"
                  max="5000.00"
                  value={totalDollars}
                  onChange={(e) => setTotalDollars(parseFloat(e.target.value) || 0)}
                  className="amount-input mono"
                  required
                />
              </div>

              <div className="amount-presets">
                {[60, 100.01, 149.99, 320].map((val) => (
                  <button
                    key={val}
                    type="button"
                    className={`preset-btn ${totalDollars === val ? 'active' : ''}`}
                    onClick={() => setTotalDollars(val)}
                  >
                    ${val.toFixed(2)}
                    {val === 100.01 && <span className="preset-note">(Odd Cents)</span>}
                  </button>
                ))}
              </div>
            </div>

            <div className="form-section">
              <label className="section-label">2. Select Payment Method (Gateway Simulator)</label>
              <p className="helper-text">Test how the platform handles successful charges, card declines, or gateway drops.</p>

              <div className="card-selector">
                <label className={`card-option ${cardToken === 'pm_card_visa' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="card"
                    value="pm_card_visa"
                    checked={cardToken === 'pm_card_visa'}
                    onChange={(e) => setCardToken(e.target.value)}
                  />
                  <div className="card-option-content">
                    <div className="card-brand">
                      <CreditCard size={18} className="icon-success" />
                      <span className="card-title">Visa Classic (Synthetic)</span>
                    </div>
                    <span className="status-badge success">Approved Immediately</span>
                  </div>
                </label>

                <label className={`card-option ${cardToken === 'pm_card_insufficient_funds' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="card"
                    value="pm_card_insufficient_funds"
                    checked={cardToken === 'pm_card_insufficient_funds'}
                    onChange={(e) => setCardToken(e.target.value)}
                  />
                  <div className="card-option-content">
                    <div className="card-brand">
                      <CreditCard size={18} className="icon-danger" />
                      <span className="card-title">Debit Card (No Funds)</span>
                    </div>
                    <span className="status-badge danger">HTTP 402 Decline</span>
                  </div>
                </label>

                <label className={`card-option ${cardToken === 'pm_card_timeout' ? 'selected' : ''}`}>
                  <input
                    type="radio"
                    name="card"
                    value="pm_card_timeout"
                    checked={cardToken === 'pm_card_timeout'}
                    onChange={(e) => setCardToken(e.target.value)}
                  />
                  <div className="card-option-content">
                    <div className="card-brand">
                      <CreditCard size={18} className="icon-warning" />
                      <span className="card-title">Network Timeout Simulator</span>
                    </div>
                    <span className="status-badge warning">Simulated Drop</span>
                  </div>
                </label>
              </div>
            </div>

            {errorMsg && (
              <div className="feedback-banner error animate-fade-in">
                <AlertTriangle size={20} />
                <div>
                  <strong>Order Creation Failed</strong>
                  <p>{errorMsg}</p>
                </div>
              </div>
            )}

            {createdOrder && (
              <div className="feedback-banner success animate-fade-in">
                <CheckCircle2 size={24} />
                <div className="banner-content">
                  <strong>Order #{createdOrder.id.slice(0, 8)} Approved & Active!</strong>
                  <p>
                    Down payment of <strong>${(createdOrder.installments[0].amount / 100).toFixed(2)}</strong> charged.
                    Remaining 3 installments scheduled on calendar.
                  </p>
                  <button type="button" className="inline-link-btn" onClick={onViewOrders}>
                    View in Merchant Orders <ArrowRight size={14} />
                  </button>
                </div>
              </div>
            )}

            <button type="submit" className="submit-btn" disabled={isLoading || totalDollars <= 0}>
              {isLoading ? (
                <>
                  <RefreshCw className="spin" size={18} />
                  <span>Processing Double-Entry Settlement...</span>
                </>
              ) : (
                <>
                  <ShoppingCart size={18} />
                  <span>Confirm Pay-in-4 Purchase (${totalDollars.toFixed(2)})</span>
                </>
              )}
            </button>
          </form>
        </div>

        {/* Right column: Dynamic breakdown & schedule preview */}
        <div className="checkout-card preview-card">
          <h2 className="preview-title">Installment Schedule (Pay-in-4)</h2>
          <p className="preview-subtitle">
            0% Interest • No Hidden Fees • Exact Cents Guaranteed
          </p>

          <div className="schedule-timeline">
            {installmentsPreview.map((item) => (
              <div key={item.number} className={`timeline-item ${item.isDownPayment ? 'highlight' : ''}`}>
                <div className="timeline-node">
                  {item.number}
                </div>
                <div className="timeline-details">
                  <div className="timeline-header">
                    <span className="cuota-title">
                      Cuota #{item.number}
                      {item.isDownPayment && <span className="downpayment-tag">Due Today</span>}
                    </span>
                    <span className="cuota-amount mono">
                      ${(item.amountCents / 100).toFixed(2)}
                    </span>
                  </div>
                  <div className="timeline-date">
                    <Calendar size={13} /> {item.date}
                  </div>
                </div>
              </div>
            ))}
          </div>

          <div className="financial-breakdown">
            <h3 className="breakdown-heading">Financial Mechanics (Under the Hood)</h3>
            <div className="breakdown-row">
              <span>Customer Total Due</span>
              <span className="mono bold">${totalDollars.toFixed(2)}</span>
            </div>
            <div className="breakdown-row">
              <span>Merchant Gross Settlement</span>
              <span className="mono">${totalDollars.toFixed(2)}</span>
            </div>
            <div className="breakdown-row">
              <span>Merchant Fee (5.00% BPS)</span>
              <span className="mono text-credit">+${(merchantFeeCents / 100).toFixed(2)}</span>
            </div>
            <div className="breakdown-row total">
              <span>Merchant Net Payout</span>
              <span className="mono text-cyan">${(merchantPayoutCents / 100).toFixed(2)}</span>
            </div>
          </div>

          <div className="security-notice">
            <CheckCircle2 size={16} className="icon-emerald" />
            <span>
              All movements post to immutable PostgreSQL tables with deferred zero-sum balance triggers.
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
