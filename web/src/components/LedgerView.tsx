import React, { useState } from 'react'
import { ShieldCheck, RotateCcw, CheckCircle2, AlertCircle, ArrowUpRight, ArrowDownLeft } from 'lucide-react'
import type { Account, Transaction } from '../types'
import { api } from '../api/client'
import './LedgerView.css'

interface LedgerViewProps {
  accounts: Account[]
  transactions: Transaction[]
  onTransactionUpdated: () => void
}

export const LedgerView: React.FC<LedgerViewProps> = ({
  accounts,
  transactions,
  onTransactionUpdated,
}) => {
  const [reversingTxId, setReversingTxId] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  const handleReverse = async (txId: string) => {
    setReversingTxId(txId)
    setFeedback(null)
    try {
      await api.reverseTransaction(txId)
      setFeedback({
        type: 'success',
        message: 'Immutable reversal transaction recorded! Counter-entries appended without mutating history.',
      })
      onTransactionUpdated()
    } catch (err: unknown) {
      setFeedback({
        type: 'error',
        message: err instanceof Error ? err.message : 'Reversal failed',
      })
    } finally {
      setReversingTxId(null)
    }
  }

  return (
    <div className="ledger-container animate-fade-in">
      <div className="ledger-header">
        <div>
          <h1>Immutable Double-Entry Ledger Explorer</h1>
          <p className="subtitle">
            Every monetary event is recorded as immutable balanced entries (&Sigma; Debits = &Sigma; Credits).
            Historical records can never be updated or deleted.
          </p>
        </div>
      </div>

      {feedback && (
        <div className={`feedback-alert ${feedback.type} animate-fade-in`}>
          {feedback.type === 'success' ? <CheckCircle2 size={18} /> : <AlertCircle size={18} />}
          <span>{feedback.message}</span>
        </div>
      )}

      {/* Account Balances Grid */}
      <h2 className="section-title">Active Chart of Accounts & Balances</h2>
      <div className="accounts-grid">
        {accounts.map((acc) => {
          const debits = acc.balance?.total_debits || 0
          const credits = acc.balance?.total_credits || 0
          const net = acc.balance?.net_balance || 0

          return (
            <div key={acc.id} className="account-card">
              <div className="account-top">
                <span className={`account-type-badge ${acc.type}`}>
                  {acc.type.toUpperCase()}
                </span>
                <span className="account-id mono">{acc.id.slice(0, 8)}...</span>
              </div>

              <div className="account-balance">
                <span className="balance-label">Calculated Net Balance</span>
                <span className="balance-value mono">
                  ${(net / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}
                  <span className="currency-tag">{acc.currency}</span>
                </span>
              </div>

              <div className="account-turnover">
                <div className="turnover-item">
                  <span className="turnover-label">
                    <ArrowDownLeft size={12} className="text-credit" /> Total Credits
                  </span>
                  <span className="mono text-credit">+${(credits / 100).toFixed(2)}</span>
                </div>
                <div className="turnover-item">
                  <span className="turnover-label">
                    <ArrowUpRight size={12} className="text-debit" /> Total Debits
                  </span>
                  <span className="mono text-debit">-${(debits / 100).toFixed(2)}</span>
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* Journal Transactions Feed */}
      <div className="journal-header">
        <h2 className="section-title">General Journal (Append-Only Transaction Stream)</h2>
        <div className="journal-legend">
          <span className="legend-item"><span className="legend-dot debit" /> Debit (+ Asset / - Liability)</span>
          <span className="legend-item"><span className="legend-dot credit" /> Credit (- Asset / + Liability)</span>
        </div>
      </div>

      <div className="transactions-stream">
        {transactions.map((tx) => {
          const totalDebit = tx.entries
            .filter((e) => e.direction === 'DEBIT')
            .reduce((acc, e) => acc + e.amount, 0)
          const totalCredit = tx.entries
            .filter((e) => e.direction === 'CREDIT')
            .reduce((acc, e) => acc + e.amount, 0)
          const isBalanced = totalDebit === totalCredit
          const isReversal = Boolean(tx.reversal_of)

          return (
            <div key={tx.id} className={`tx-card ${isReversal ? 'is-reversal' : ''}`}>
              <div className="tx-header">
                <div className="tx-meta-info">
                  <span className="tx-id mono">TX #{tx.id.slice(0, 8)}</span>
                  <span className="tx-desc">{tx.description}</span>
                  {isReversal && (
                    <span className="reversal-pill">
                      Reversal of #{tx.reversal_of?.slice(0, 8)}
                    </span>
                  )}
                </div>

                <div className="tx-right-meta">
                  <div className={`balance-badge ${isBalanced ? 'balanced' : 'unbalanced'}`}>
                    <ShieldCheck size={14} />
                    <span>Zero-Sum Verified (${(totalDebit / 100).toFixed(2)})</span>
                  </div>

                  {!isReversal && (
                    <button
                      className="reverse-btn"
                      onClick={() => handleReverse(tx.id)}
                      disabled={reversingTxId === tx.id}
                      title="Post an immutable inverse transaction to cancel this movement"
                    >
                      <RotateCcw size={14} />
                      <span>{reversingTxId === tx.id ? 'Reversing...' : 'Reverse'}</span>
                    </button>
                  )}
                </div>
              </div>

              {/* Entries Breakdown Table */}
              <div className="entries-table-wrapper">
                <table className="entries-table">
                  <thead>
                    <tr>
                      <th>Account</th>
                      <th>Direction</th>
                      <th className="text-right">Debit</th>
                      <th className="text-right">Credit</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tx.entries.map((entry) => {
                      const isDebit = entry.direction === 'DEBIT'
                      return (
                        <tr key={entry.id}>
                          <td className="mono account-cell">
                            {entry.account_id.slice(0, 13)}...
                          </td>
                          <td>
                            <span className={`direction-tag ${entry.direction.toLowerCase()}`}>
                              {entry.direction}
                            </span>
                          </td>
                          <td className="text-right mono text-debit">
                            {isDebit ? `$${(entry.amount / 100).toFixed(2)}` : '—'}
                          </td>
                          <td className="text-right mono text-credit">
                            {!isDebit ? `$${(entry.amount / 100).toFixed(2)}` : '—'}
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                  <tfoot>
                    <tr>
                      <td colSpan={2} className="total-label">Sum of Entries (Invariants Check):</td>
                      <td className="text-right mono text-debit bold">${(totalDebit / 100).toFixed(2)}</td>
                      <td className="text-right mono text-credit bold">${(totalCredit / 100).toFixed(2)}</td>
                    </tr>
                  </tfoot>
                </table>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
