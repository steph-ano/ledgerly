import { useState, useEffect, useCallback } from 'react'
import { Navbar, type NavTab } from './components/Navbar'
import { CheckoutSimulator } from './components/CheckoutSimulator'
import { OrdersView } from './components/OrdersView'
import { LedgerView } from './components/LedgerView'
import { WebhooksView } from './components/WebhooksView'
import { ArchitectureModal } from './components/ArchitectureModal'
import { api } from './api/client'
import type { Account, Order, Transaction, OutboxEvent } from './types'
import './App.css'

export function App() {
  const [currentTab, setCurrentTab] = useState<NavTab>('checkout')
  const [isArchModalOpen, setIsArchModalOpen] = useState<boolean>(false)
  const [health, setHealth] = useState<{ ledger: boolean; bnpl: boolean }>({ ledger: false, bnpl: false })

  const [accounts, setAccounts] = useState<Account[]>([])
  const [orders, setOrders] = useState<Order[]>([])
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [outboxEvents, setOutboxEvents] = useState<OutboxEvent[]>([])

  const refreshData = useCallback(async () => {
    try {
      const [accs, ords, txs, evts, h] = await Promise.all([
        api.getAccounts(),
        api.getOrders(),
        api.getTransactions(),
        api.getOutboxEvents(),
        api.checkHealth(),
      ])
      setAccounts(accs)
      setOrders(ords)
      setTransactions(txs)
      setOutboxEvents(evts)
      setHealth(h)
    } catch {
      // Handled silently
    }
  }, [])

  useEffect(() => {
    refreshData()
    const interval = setInterval(refreshData, 10000)
    return () => clearInterval(interval)
  }, [refreshData])

  const handleOrderCreated = (newOrder: Order) => {
    setOrders((prev) => [newOrder, ...prev])
    refreshData()
  }

  return (
    <div className="app-layout">
      <Navbar
        currentTab={currentTab}
        onSelectTab={(tab) => setCurrentTab(tab)}
        onOpenArchitecture={() => setIsArchModalOpen(true)}
        health={health}
      />

      <main className="app-content">
        {currentTab === 'checkout' && (
          <CheckoutSimulator
            onOrderCreated={handleOrderCreated}
            onViewOrders={() => setCurrentTab('orders')}
          />
        )}

        {currentTab === 'orders' && (
          <OrdersView
            orders={orders}
            onOrderUpdated={refreshData}
          />
        )}

        {currentTab === 'ledger' && (
          <LedgerView
            accounts={accounts}
            transactions={transactions}
            onTransactionUpdated={refreshData}
          />
        )}

        {currentTab === 'webhooks' && (
          <WebhooksView
            events={outboxEvents}
          />
        )}
      </main>

      <footer className="app-footer">
        <div className="footer-container">
          <div className="footer-left">
            <span className="brand-footer mono">Ledgerly v1.0.0</span>
            <span className="footer-sep">•</span>
            <span>Immutable Double-Entry Ledger Core & BNPL Engine</span>
          </div>

          <div className="footer-right">
            <span className="footer-badge mono">Go 1.24+</span>
            <span className="footer-badge mono">PostgreSQL 16 (READ COMMITTED)</span>
            <span className="footer-badge mono">React 19 + TypeScript</span>
          </div>
        </div>
      </footer>

      <ArchitectureModal
        isOpen={isArchModalOpen}
        onClose={() => setIsArchModalOpen(false)}
      />
    </div>
  )
}

export default App
