import React from 'react'
import { X, ShieldAlert, Cpu, Lock, GitCommit, CheckCircle2, Layers } from 'lucide-react'
import './ArchitectureModal.css'

interface ArchitectureModalProps {
  isOpen: boolean
  onClose: () => void
}

export const ArchitectureModal: React.FC<ArchitectureModalProps> = ({ isOpen, onClose }) => {
  if (!isOpen) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="arch-modal-card animate-fade-in" onClick={(e) => e.stopPropagation()}>
        <div className="arch-modal-header">
          <div className="arch-header-title">
            <Layers className="icon-cyan" size={24} />
            <div>
              <h2>Ledgerly Architecture & Invariants</h2>
              <p>Foundational principles designed for high-concurrency fintech resilience</p>
            </div>
          </div>
          <button className="close-btn" onClick={onClose}>
            <X size={20} />
          </button>
        </div>

        <div className="arch-modal-body">
          <div className="invariants-list">
            <div className="invariant-box">
              <div className="inv-icon">
                <ShieldAlert className="text-cyan" size={20} />
              </div>
              <div className="inv-content">
                <h3>1. Strict Integer Money (Zero Floating Point)</h3>
                <p>
                  Floating point numbers (<code>float64</code>) are forbidden across models, database schemas, and APIs.
                  All amounts are 64-bit signed integers in minor currency units (cents). Arithmetic operations implement
                  overflow boundary checking (<code>math.MaxInt64</code>).
                </p>
              </div>
            </div>

            <div className="invariant-box">
              <div className="inv-icon">
                <Lock className="text-credit" size={20} />
              </div>
              <div className="inv-content">
                <h3>2. Double-Entry Zero-Sum Invariant & Deferred Triggers</h3>
                <p>
                  Every financial movement requires &Sigma; Debits &minus; &Sigma; Credits = 0 in a single currency.
                  PostgreSQL enforces this through a <code>DEFERRABLE INITIALLY DEFERRED</code> constraint trigger verified at <code>COMMIT</code> time.
                </p>
              </div>
            </div>

            <div className="invariant-box">
              <div className="inv-icon">
                <GitCommit className="text-purple" size={20} />
              </div>
              <div className="inv-content">
                <h3>3. Append-Only Immutability</h3>
                <p>
                  Database triggers block <code>UPDATE</code> and <code>DELETE</code> operations on <code>transactions</code> and <code>entries</code> (raising SQLSTATE <code>55000</code>).
                  Modifications occur exclusively via balancing reversal transactions.
                </p>
              </div>
            </div>

            <div className="invariant-box">
              <div className="inv-icon">
                <Cpu className="text-cyan" size={20} />
              </div>
              <div className="inv-content">
                <h3>4. Concurrency: Lexicographical Row Locks & SKIP LOCKED</h3>
                <p>
                  Deadlocks are mathematically eliminated by sorting account IDs in ascending order (<code>SELECT ... FOR UPDATE ORDER BY account_id ASC</code>) under <code>READ COMMITTED</code>.
                  The scheduler leases due installments with <code>SKIP LOCKED</code> across multi-instance worker nodes.
                </p>
              </div>
            </div>

            <div className="invariant-box">
              <div className="inv-icon">
                <CheckCircle2 className="text-emerald" size={20} />
              </div>
              <div className="inv-content">
                <h3>5. Atomic Idempotency & Exponential Backoff</h3>
                <p>
                  Idempotency keys and SHA-256 canonical request hashes are persisted in the same transaction using
                  <code>INSERT ... ON CONFLICT (client_id, idempotency_key) DO NOTHING</code>. Declined installments retry exponentially (+2h, +12h, +24h) before terminal default.
                </p>
              </div>
            </div>
          </div>
        </div>

        <div className="arch-modal-footer">
          <span className="mono footer-note">Validated with Go race detector (-race) and Rapid property-based testing</span>
          <button className="done-btn" onClick={onClose}>Done</button>
        </div>
      </div>
    </div>
  )
}
