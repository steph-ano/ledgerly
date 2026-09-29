import React from 'react'
import { ShieldCheck, Layers, ShoppingBag, BookOpen, Activity, GitBranch } from 'lucide-react'
import './Navbar.css'

export type NavTab = 'checkout' | 'orders' | 'ledger' | 'webhooks'

interface NavbarProps {
  currentTab: NavTab
  onSelectTab: (tab: NavTab) => void
  onOpenArchitecture: () => void
  health: { ledger: boolean; bnpl: boolean }
}

export const Navbar: React.FC<NavbarProps> = ({
  currentTab,
  onSelectTab,
  onOpenArchitecture,
  health,
}) => {
  return (
    <header className="navbar">
      <div className="navbar-container">
        <div className="navbar-brand">
          <div className="brand-icon">
            <Layers className="icon-cyan" size={22} />
          </div>
          <div className="brand-text">
            <span className="brand-name">Ledgerly</span>
            <span className="brand-tag">BNPL Core & Ledger</span>
          </div>
        </div>

        <nav className="navbar-links">
          <button
            className={`nav-item ${currentTab === 'checkout' ? 'active' : ''}`}
            onClick={() => onSelectTab('checkout')}
          >
            <ShoppingBag size={18} />
            <span>Pay-in-4 Checkout</span>
          </button>

          <button
            className={`nav-item ${currentTab === 'orders' ? 'active' : ''}`}
            onClick={() => onSelectTab('orders')}
          >
            <Activity size={18} />
            <span>Merchant Orders</span>
          </button>

          <button
            className={`nav-item ${currentTab === 'ledger' ? 'active' : ''}`}
            onClick={() => onSelectTab('ledger')}
          >
            <ShieldCheck size={18} />
            <span>Double-Entry Ledger</span>
          </button>

          <button
            className={`nav-item ${currentTab === 'webhooks' ? 'active' : ''}`}
            onClick={() => onSelectTab('webhooks')}
          >
            <GitBranch size={18} />
            <span>Outbox Webhooks</span>
          </button>
        </nav>

        <div className="navbar-actions">
          <div className="health-badges">
            <div className={`health-pill ${health.ledger ? 'online' : 'simulated'}`}>
              <span className="dot" />
              <span>Ledger :8080</span>
            </div>
            <div className={`health-pill ${health.bnpl ? 'online' : 'simulated'}`}>
              <span className="dot" />
              <span>BNPL :8081</span>
            </div>
          </div>

          <button className="arch-btn" onClick={onOpenArchitecture} title="View Architecture & Invariants">
            <BookOpen size={16} />
            <span>Architecture Specs</span>
          </button>
        </div>
      </div>
    </header>
  )
}
