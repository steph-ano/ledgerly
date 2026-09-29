import type { Account, Order, Transaction, OutboxEvent, Installment } from '../types'

const DEFAULT_PLATFORM_ACCOUNT_ID = '11111111-1111-1111-1111-111111111111'
const DEFAULT_FEES_ACCOUNT_ID = '22222222-2222-2222-2222-222222222222'

export interface CreateOrderParams {
  totalAmount: number // in cents
  currency: string
  paymentMethodToken: string
  merchantWebhookUrl?: string
  customerAccountId: string
  merchantAccountId: string
}

// Initial mock data used when offline or in demonstration mode
const mockAccounts: Account[] = [
  {
    id: DEFAULT_PLATFORM_ACCOUNT_ID,
    client_id: 'system',
    type: 'platform',
    currency: 'USD',
    created_at: new Date(Date.now() - 86400000 * 30).toISOString(),
    balance: {
      total_debits: 25000000,
      total_credits: 18500000,
      net_balance: 6500000, // $65,000.00
      currency: 'USD',
    },
  },
  {
    id: DEFAULT_FEES_ACCOUNT_ID,
    client_id: 'system',
    type: 'fees',
    currency: 'USD',
    created_at: new Date(Date.now() - 86400000 * 30).toISOString(),
    balance: {
      total_debits: 0,
      total_credits: 425000,
      net_balance: 425000, // $4,250.00 earned
      currency: 'USD',
    },
  },
  {
    id: '33333333-3333-3333-3333-333333333333',
    client_id: 'merchant_nordic_app',
    type: 'merchant',
    currency: 'USD',
    created_at: new Date(Date.now() - 86400000 * 15).toISOString(),
    balance: {
      total_debits: 800000,
      total_credits: 3200000,
      net_balance: 2400000, // $24,000.00
      currency: 'USD',
    },
  },
  {
    id: '44444444-4444-4444-4444-444444444444',
    client_id: 'customer_app',
    type: 'customer',
    currency: 'USD',
    created_at: new Date(Date.now() - 86400000 * 10).toISOString(),
    balance: {
      total_debits: 120000,
      total_credits: 90000,
      net_balance: 30000, // $300.00 outstanding
      currency: 'USD',
    },
  },
]

const mockOrders: Order[] = [
  {
    id: 'a1b2c3d4-0001-4000-8000-000000000001',
    client_id: 'merchant_nordic_app',
    customer_account_id: '44444444-4444-4444-4444-444444444444',
    merchant_account_id: '33333333-3333-3333-3333-333333333333',
    total_amount: 12000, // $120.00
    currency: 'USD',
    status: 'active',
    merchant_webhook_url: 'https://store.nordic.test/webhooks',
    created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
    updated_at: new Date().toISOString(),
    installments: [
      {
        id: 'inst-0001-1',
        order_id: 'a1b2c3d4-0001-4000-8000-000000000001',
        installment_number: 1,
        amount: 3000,
        currency: 'USD',
        due_date: new Date(Date.now() - 86400000 * 14).toISOString(),
        status: 'paid',
        attempt_count: 1,
        next_retry_at: null,
        paid_at: new Date(Date.now() - 86400000 * 14).toISOString(),
        ledger_transaction_id: 'tx-0001-downpayment',
      },
      {
        id: 'inst-0001-2',
        order_id: 'a1b2c3d4-0001-4000-8000-000000000001',
        installment_number: 2,
        amount: 3000,
        currency: 'USD',
        due_date: new Date(Date.now()).toISOString(),
        status: 'paid',
        attempt_count: 1,
        next_retry_at: null,
        paid_at: new Date().toISOString(),
        ledger_transaction_id: 'tx-0001-inst2',
      },
      {
        id: 'inst-0001-3',
        order_id: 'a1b2c3d4-0001-4000-8000-000000000001',
        installment_number: 3,
        amount: 3000,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 14).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
        ledger_transaction_id: null,
      },
      {
        id: 'inst-0001-4',
        order_id: 'a1b2c3d4-0001-4000-8000-000000000001',
        installment_number: 4,
        amount: 3000,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 28).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
        ledger_transaction_id: null,
      },
    ],
  },
  {
    id: 'a1b2c3d4-0002-4000-8000-000000000002',
    client_id: 'merchant_nordic_app',
    customer_account_id: '44444444-4444-4444-4444-444444444444',
    merchant_account_id: '33333333-3333-3333-3333-333333333333',
    total_amount: 10001, // $100.01 (non-divisible)
    currency: 'USD',
    status: 'active',
    merchant_webhook_url: 'https://store.nordic.test/webhooks',
    created_at: new Date(Date.now() - 86400000 * 2).toISOString(),
    updated_at: new Date().toISOString(),
    installments: [
      {
        id: 'inst-0002-1',
        order_id: 'a1b2c3d4-0002-4000-8000-000000000002',
        installment_number: 1,
        amount: 2501, // extra 1 cent in down payment
        currency: 'USD',
        due_date: new Date(Date.now() - 86400000 * 2).toISOString(),
        status: 'paid',
        attempt_count: 1,
        next_retry_at: null,
        paid_at: new Date(Date.now() - 86400000 * 2).toISOString(),
        ledger_transaction_id: 'tx-0002-downpayment',
      },
      {
        id: 'inst-0002-2',
        order_id: 'a1b2c3d4-0002-4000-8000-000000000002',
        installment_number: 2,
        amount: 2500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 12).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
        ledger_transaction_id: null,
      },
      {
        id: 'inst-0002-3',
        order_id: 'a1b2c3d4-0002-4000-8000-000000000002',
        installment_number: 3,
        amount: 2500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 26).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
        ledger_transaction_id: null,
      },
      {
        id: 'inst-0002-4',
        order_id: 'a1b2c3d4-0002-4000-8000-000000000002',
        installment_number: 4,
        amount: 2500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 40).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
        ledger_transaction_id: null,
      },
    ],
  },
]

const mockTransactions: Transaction[] = [
  {
    id: 'tx-0001-downpayment',
    client_id: 'bnpl_service',
    idempotency_key: 'downpayment_order_0001',
    description: 'Down payment: Installment #1 (Order #0001)',
    reversal_of: null,
    created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
    entries: [
      {
        id: 'entry-1',
        transaction_id: 'tx-0001-downpayment',
        account_id: DEFAULT_PLATFORM_ACCOUNT_ID,
        amount: 3000,
        direction: 'DEBIT',
        currency: 'USD',
        created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
      },
      {
        id: 'entry-2',
        transaction_id: 'tx-0001-downpayment',
        account_id: '44444444-4444-4444-4444-444444444444',
        amount: 3000,
        direction: 'CREDIT',
        currency: 'USD',
        created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
      },
    ],
  },
  {
    id: 'tx-0001-merchantpayout',
    client_id: 'bnpl_service',
    idempotency_key: 'payout_order_0001',
    description: 'Merchant Payout minus 5% Fee (Order #0001)',
    reversal_of: null,
    created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
    entries: [
      {
        id: 'entry-3',
        transaction_id: 'tx-0001-merchantpayout',
        account_id: DEFAULT_PLATFORM_ACCOUNT_ID,
        amount: 12000,
        direction: 'DEBIT',
        currency: 'USD',
        created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
      },
      {
        id: 'entry-4',
        transaction_id: 'tx-0001-merchantpayout',
        account_id: '33333333-3333-3333-3333-333333333333',
        amount: 11400, // $114.00 (95%)
        direction: 'CREDIT',
        currency: 'USD',
        created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
      },
      {
        id: 'entry-5',
        transaction_id: 'tx-0001-merchantpayout',
        account_id: DEFAULT_FEES_ACCOUNT_ID,
        amount: 600, // $6.00 (5%)
        direction: 'CREDIT',
        currency: 'USD',
        created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
      },
    ],
  },
]

const mockOutboxEvents: OutboxEvent[] = [
  {
    id: 'evt-001',
    event_type: 'order.created',
    aggregate_type: 'order',
    aggregate_id: 'a1b2c3d4-0001-4000-8000-000000000001',
    destination_url: 'https://store.nordic.test/webhooks',
    payload: { order_id: 'a1b2c3d4-0001-4000-8000-000000000001', status: 'active', total_amount: 12000 },
    status: 'delivered',
    retry_count: 0,
    created_at: new Date(Date.now() - 86400000 * 14).toISOString(),
    delivered_at: new Date(Date.now() - 86400000 * 14 + 120).toISOString(),
  },
  {
    id: 'evt-002',
    event_type: 'installment.paid',
    aggregate_type: 'installment',
    aggregate_id: 'inst-0001-2',
    destination_url: 'https://store.nordic.test/webhooks',
    payload: { order_id: 'a1b2c3d4-0001-4000-8000-000000000001', installment_number: 2, amount: 3000 },
    status: 'delivered',
    retry_count: 0,
    created_at: new Date().toISOString(),
    delivered_at: new Date(Date.now() + 80).toISOString(),
  },
]

class ApiClient {
  private inMemoryAccounts = [...mockAccounts]
  private inMemoryOrders = [...mockOrders]
  private inMemoryTransactions = [...mockTransactions]
  private inMemoryEvents = [...mockOutboxEvents]

  async checkHealth(): Promise<{ ledger: boolean; bnpl: boolean }> {
    let ledgerOk = false
    let bnplOk = false

    try {
      const res = await fetch('/api/ledger/healthz')
      ledgerOk = res.ok
    } catch {
      ledgerOk = false
    }

    try {
      const res = await fetch('/api/bnpl/healthz')
      bnplOk = res.ok
    } catch {
      bnplOk = false
    }

    return { ledger: ledgerOk, bnpl: bnplOk }
  }

  async getAccounts(): Promise<Account[]> {
    return this.inMemoryAccounts
  }

  async getOrders(): Promise<Order[]> {
    try {
      const res = await fetch('/api/bnpl/v1/orders')
      if (res.ok) {
        const data = await res.json()
        if (Array.isArray(data)) return data
      }
    } catch {
      // Fallback to in-memory state
    }
    return this.inMemoryOrders
  }

  async getTransactions(): Promise<Transaction[]> {
    return this.inMemoryTransactions
  }

  async getOutboxEvents(): Promise<OutboxEvent[]> {
    return this.inMemoryEvents
  }

  async createOrder(params: CreateOrderParams): Promise<Order> {
    try {
      const res = await fetch('/api/bnpl/v1/orders', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Client-ID': 'web_portal',
        },
        body: JSON.stringify({
          client_id: 'web_portal',
          customer_account_id: params.customerAccountId,
          merchant_account_id: params.merchantAccountId,
          total_amount: params.totalAmount,
          currency: params.currency,
          payment_method_token: params.paymentMethodToken,
          merchant_webhook_url: params.merchantWebhookUrl || 'https://demo-merchant.test/webhooks',
        }),
      })

      if (res.ok) {
        return await res.json()
      }
      if (res.status === 402) {
        const err = await res.json()
        throw new Error(err.message || 'Payment method declined')
      }
    } catch (e: unknown) {
      if (e instanceof Error && e.message.includes('declined')) {
        throw e
      }
    }

    // In-memory simulation when offline
    if (params.paymentMethodToken === 'pm_card_insufficient_funds' || params.paymentMethodToken === 'pm_card_declined') {
      throw new Error('Initial down payment was declined: card issuer returned 05: Do Not Honor')
    }
    if (params.paymentMethodToken === 'pm_card_timeout') {
      throw new Error('Payment gateway timed out while processing down payment (simulated timeout)')
    }

    const orderId = crypto.randomUUID()
    const now = new Date()
    const baseInstallment = Math.floor(params.totalAmount / 4)
    const remainder = params.totalAmount % 4

    const installments = [1, 2, 3, 4].map((num) => {
      const dueDate = new Date(now.getTime() + (num - 1) * 14 * 86400000)
      const amount = num === 1 ? baseInstallment + remainder : baseInstallment
      return {
        id: crypto.randomUUID(),
        order_id: orderId,
        installment_number: num,
        amount,
        currency: params.currency,
        due_date: dueDate.toISOString(),
        status: (num === 1 ? 'paid' : 'pending') as Order['installments'][0]['status'],
        attempt_count: num === 1 ? 1 : 0,
        next_retry_at: null,
        paid_at: num === 1 ? now.toISOString() : null,
        ledger_transaction_id: num === 1 ? crypto.randomUUID() : null,
      }
    })

    const newOrder: Order = {
      id: orderId,
      client_id: 'web_portal',
      customer_account_id: params.customerAccountId,
      merchant_account_id: params.merchantAccountId,
      total_amount: params.totalAmount,
      currency: params.currency,
      status: 'active',
      merchant_webhook_url: params.merchantWebhookUrl || 'https://demo-merchant.test/webhooks',
      created_at: now.toISOString(),
      updated_at: now.toISOString(),
      installments,
    }

    this.inMemoryOrders.unshift(newOrder)

    // Add corresponding ledger transaction entries for downpayment
    const downpaymentTx: Transaction = {
      id: crypto.randomUUID(),
      client_id: 'bnpl_service',
      idempotency_key: `dp_${orderId}`,
      description: `Down payment: Order #${orderId.slice(0, 8)}`,
      reversal_of: null,
      created_at: now.toISOString(),
      entries: [
        {
          id: crypto.randomUUID(),
          transaction_id: crypto.randomUUID(),
          account_id: DEFAULT_PLATFORM_ACCOUNT_ID,
          amount: installments[0].amount,
          direction: 'DEBIT',
          currency: params.currency,
          created_at: now.toISOString(),
        },
        {
          id: crypto.randomUUID(),
          transaction_id: crypto.randomUUID(),
          account_id: params.customerAccountId,
          amount: installments[0].amount,
          direction: 'CREDIT',
          currency: params.currency,
          created_at: now.toISOString(),
        },
      ],
    }
    this.inMemoryTransactions.unshift(downpaymentTx)

    // Add webhook outbox event
    this.inMemoryEvents.unshift({
      id: crypto.randomUUID(),
      event_type: 'order.created',
      aggregate_type: 'order',
      aggregate_id: orderId,
      destination_url: newOrder.merchant_webhook_url,
      payload: { order_id: orderId, total_amount: params.totalAmount, status: 'active' },
      status: 'delivered',
      retry_count: 0,
      created_at: now.toISOString(),
      delivered_at: new Date(now.getTime() + 150).toISOString(),
    })

    return newOrder
  }

  async payInstallment(installmentId: string, cardToken: string): Promise<Installment> {
    try {
      const res = await fetch(`/api/bnpl/v1/installments/${installmentId}/pay`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ payment_method_token: cardToken }),
      })
      if (res.ok) {
        return await res.json()
      }
    } catch {
      // offline fallback
    }

    if (cardToken === 'pm_card_insufficient_funds' || cardToken === 'pm_card_declined') {
      throw new Error('Installment payment declined: Insufficient funds on simulated card')
    }

    for (const ord of this.inMemoryOrders) {
      const inst = ord.installments.find((i) => i.id === installmentId)
      if (inst) {
        if (inst.status === 'paid') {
          throw new Error('Installment is already paid')
        }
        inst.status = 'paid'
        inst.paid_at = new Date().toISOString()
        inst.attempt_count += 1
        inst.ledger_transaction_id = crypto.randomUUID()

        // Check if all installments are paid
        if (ord.installments.every((i) => i.status === 'paid')) {
          ord.status = 'completed'
        }

        // Post ledger entries
        this.inMemoryTransactions.unshift({
          id: inst.ledger_transaction_id,
          client_id: 'bnpl_scheduler',
          idempotency_key: `pay_${installmentId}`,
          description: `Cuota #${inst.installment_number} for Order #${ord.id.slice(0, 8)}`,
          reversal_of: null,
          created_at: new Date().toISOString(),
          entries: [
            {
              id: crypto.randomUUID(),
              transaction_id: inst.ledger_transaction_id,
              account_id: DEFAULT_PLATFORM_ACCOUNT_ID,
              amount: inst.amount,
              direction: 'DEBIT',
              currency: inst.currency,
              created_at: new Date().toISOString(),
            },
            {
              id: crypto.randomUUID(),
              transaction_id: inst.ledger_transaction_id,
              account_id: ord.customer_account_id,
              amount: inst.amount,
              direction: 'CREDIT',
              currency: inst.currency,
              created_at: new Date().toISOString(),
            },
          ],
        })

        // Webhook event
        this.inMemoryEvents.unshift({
          id: crypto.randomUUID(),
          event_type: 'installment.paid',
          aggregate_type: 'installment',
          aggregate_id: installmentId,
          destination_url: ord.merchant_webhook_url,
          payload: { order_id: ord.id, installment_number: inst.installment_number, amount: inst.amount },
          status: 'delivered',
          retry_count: 0,
          created_at: new Date().toISOString(),
          delivered_at: new Date().toISOString(),
        })

        return inst
      }
    }

    throw new Error('Installment not found')
  }

  async reverseTransaction(txId: string): Promise<Transaction> {
    const original = this.inMemoryTransactions.find((t) => t.id === txId)
    if (!original) throw new Error('Transaction not found')
    if (original.reversal_of) throw new Error('Cannot reverse a reversal transaction')

    const reversalTx: Transaction = {
      id: crypto.randomUUID(),
      client_id: 'web_portal_reversal',
      idempotency_key: `rev_${txId}`,
      description: `Reversal of: ${original.description}`,
      reversal_of: original.id,
      created_at: new Date().toISOString(),
      entries: original.entries.map((e) => ({
        id: crypto.randomUUID(),
        transaction_id: crypto.randomUUID(),
        account_id: e.account_id,
        amount: e.amount,
        direction: e.direction === 'DEBIT' ? 'CREDIT' : 'DEBIT', // Inverted
        currency: e.currency,
        created_at: new Date().toISOString(),
      })),
    }

    this.inMemoryTransactions.unshift(reversalTx)
    return reversalTx
  }
}

export const api = new ApiClient()
