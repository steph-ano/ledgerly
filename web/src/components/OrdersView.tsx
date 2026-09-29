import React, { useState } from 'react'
import { Calendar, CheckCircle2, Clock, AlertTriangle, ChevronDown, ChevronUp, DollarSign, Layers, ShieldCheck, Zap } from 'lucide-react'
import type { Order, Installment } from '../types'
import { api } from '../api/client'
import './OrdersView.css'

interface OrdersViewProps {
  orders: Order[]
  onOrderUpdated: () => void
}

export const OrdersView: React.FC<OrdersViewProps> = ({ orders, onOrderUpdated }) => {
  const [expandedOrderId, setExpandedOrderId] = useState<string | null>(orders[0]?.id || null)
  const [payingInstallment, setPayingInstallment] = useState<Installment | null>(null)
  const [paymentCard, setPaymentCard] = useState<string>('pm_card_visa')
  const [isProcessing, setIsProcessing] = useState<boolean>(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [actionSuccess, setActionSuccess] = useState<string | null>(null)

  const totalVolumeCents = orders.reduce((acc, o) => acc + o.total_amount, 0)
  const activeOrdersCount = orders.filter((o) => o.status === 'active').length
  const completedOrdersCount = orders.filter((o) => o.status === 'completed').length

  const handlePayInstallment = async (inst: Installment) => {
    setIsProcessing(true)
    setActionError(null)
    setActionSuccess(null)

    try {
      await api.payInstallment(inst.id, paymentCard)
      setActionSuccess(`Cuota #${inst.installment_number} successfully paid! Double-entry ledger settlement recorded.`)
      setPayingInstallment(null)
      onOrderUpdated()
    } catch (err: unknown) {
      if (err instanceof Error) {
        setActionError(err.message)
      } else {
        setActionError('Failed to pay installment')
      }
    } finally {
      setIsProcessing(false)
    }
  }

  const toggleExpand = (id: string) => {
    setExpandedOrderId(expandedOrderId === id ? null : id)
  }

  return (
    <div className="orders-container animate-fade-in">
      <div className="orders-header">
        <div>
          <h1>Merchant Orders & Installment Pipeline</h1>
          <p className="subtitle">
            Observe the lifecycle of Pay-in-4 contracts, automated scheduler leases, and manual customer settlements.
          </p>
        </div>
      </div>

      {/* Metrics Row */}
      <div className="metrics-grid">
        <div className="metric-card">
          <div className="metric-icon cyan">
            <DollarSign size={20} />
          </div>
          <div className="metric-content">
            <span className="metric-label">Total Contract Volume</span>
            <span className="metric-value mono">${(totalVolumeCents / 100).toFixed(2)}</span>
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-icon purple">
            <Layers size={20} />
          </div>
          <div className="metric-content">
            <span className="metric-label">Active Portfolios</span>
            <span className="metric-value mono">{activeOrdersCount}</span>
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-icon emerald">
            <ShieldCheck size={20} />
          </div>
          <div className="metric-content">
            <span className="metric-label">Completed Pay-in-4</span>
            <span className="metric-value mono">{completedOrdersCount}</span>
          </div>
        </div>

        <div className="metric-card">
          <div className="metric-icon rose">
            <Zap size={20} />
          </div>
          <div className="metric-content">
            <span className="metric-label">Default Rate</span>
            <span className="metric-value mono">0.00%</span>
          </div>
        </div>
      </div>

      {actionSuccess && (
        <div className="alert-banner success animate-fade-in">
          <CheckCircle2 size={18} />
          <span>{actionSuccess}</span>
        </div>
      )}

      {actionError && (
        <div className="alert-banner error animate-fade-in">
          <AlertTriangle size={18} />
          <span>{actionError}</span>
        </div>
      )}

      {/* Orders List */}
      <div className="orders-list">
        {orders.map((order) => {
          const isExpanded = expandedOrderId === order.id
          const paidCount = order.installments.filter((i) => i.status === 'paid').length
          const progressPercent = (paidCount / 4) * 100

          return (
            <div key={order.id} className={`order-card ${isExpanded ? 'expanded' : ''}`}>
              <div className="order-summary" onClick={() => toggleExpand(order.id)}>
                <div className="order-main-info">
                  <span className="order-id mono">#{order.id.slice(0, 8)}</span>
                  <span className={`status-pill ${order.status}`}>
                    {order.status}
                  </span>
                  <span className="client-id mono">{order.client_id}</span>
                </div>

                <div className="order-progress-section">
                  <div className="progress-label">
                    <span>Installments: {paidCount} / 4 Paid</span>
                    <span className="mono">{progressPercent.toFixed(0)}%</span>
                  </div>
                  <div className="progress-bar">
                    <div className="progress-fill" style={{ width: `${progressPercent}%` }} />
                  </div>
                </div>

                <div className="order-amount-section">
                  <span className="amount-label">Contract Amount</span>
                  <span className="amount-value mono">${(order.total_amount / 100).toFixed(2)}</span>
                </div>

                <button className="expand-btn">
                  {isExpanded ? <ChevronUp size={20} /> : <ChevronDown size={20} />}
                </button>
              </div>

              {/* Expanded Installments Details */}
              {isExpanded && (
                <div className="installments-detail animate-fade-in">
                  <h3 className="detail-heading">Installment Schedule & Concurrency Leasing Status</h3>
                  <div className="installments-grid">
                    {order.installments.map((inst) => {
                      const isPaid = inst.status === 'paid'
                      return (
                        <div key={inst.id} className={`installment-item ${inst.status}`}>
                          <div className="inst-header">
                            <span className="inst-number">Cuota #{inst.installment_number}</span>
                            <span className={`inst-status-badge ${inst.status}`}>
                              {inst.status}
                            </span>
                          </div>

                          <div className="inst-amount mono">
                            ${(inst.amount / 100).toFixed(2)} {inst.currency}
                          </div>

                          <div className="inst-meta">
                            <div className="meta-line">
                              <Calendar size={13} />
                              <span>Due: {new Date(inst.due_date).toLocaleDateString()}</span>
                            </div>
                            <div className="meta-line">
                              <Clock size={13} />
                              <span>Attempts: {inst.attempt_count}</span>
                            </div>
                            {inst.paid_at && (
                              <div className="meta-line text-credit">
                                <CheckCircle2 size={13} />
                                <span>Paid: {new Date(inst.paid_at).toLocaleDateString()}</span>
                              </div>
                            )}
                          </div>

                          {!isPaid && (
                            <button
                              className="pay-cuota-btn"
                              onClick={() => setPayingInstallment(inst)}
                            >
                              Simulate Early Payment
                            </button>
                          )}
                        </div>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Payment Modal */}
      {payingInstallment && (
        <div className="modal-backdrop">
          <div className="modal-card animate-fade-in">
            <h2 className="modal-title">
              Pay Cuota #{payingInstallment.installment_number}
            </h2>
            <p className="modal-subtitle">
              Settle ${(payingInstallment.amount / 100).toFixed(2)} {payingInstallment.currency} through the simulator.
            </p>

            <div className="modal-body">
              <label className="section-label">Select Simulation Payment Card</label>
              <div className="modal-card-options">
                <label className={`modal-option ${paymentCard === 'pm_card_visa' ? 'active' : ''}`}>
                  <input
                    type="radio"
                    name="modal_card"
                    value="pm_card_visa"
                    checked={paymentCard === 'pm_card_visa'}
                    onChange={(e) => setPaymentCard(e.target.value)}
                  />
                  <span>Visa (Success - Ledger Entry Posted)</span>
                </label>
                <label className={`modal-option ${paymentCard === 'pm_card_insufficient_funds' ? 'active' : ''}`}>
                  <input
                    type="radio"
                    name="modal_card"
                    value="pm_card_insufficient_funds"
                    checked={paymentCard === 'pm_card_insufficient_funds'}
                    onChange={(e) => setPaymentCard(e.target.value)}
                  />
                  <span>Decline Card (Triggers Scheduler Backoff)</span>
                </label>
              </div>
            </div>

            <div className="modal-actions">
              <button
                className="cancel-btn"
                onClick={() => setPayingInstallment(null)}
                disabled={isProcessing}
              >
                Cancel
              </button>
              <button
                className="confirm-btn"
                onClick={() => handlePayInstallment(payingInstallment)}
                disabled={isProcessing}
              >
                {isProcessing ? 'Processing Settlement...' : 'Confirm Payment'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
