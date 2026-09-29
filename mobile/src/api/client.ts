import type { Order, CustomerStatementEntry } from '../types'

const initialOrders: Order[] = [
  {
    id: 'ord-mob-001',
    client_id: 'nordic_store',
    customer_account_id: 'cust-uuid-4444',
    merchant_account_id: 'merch-uuid-3333',
    total_amount: 10001, // $100.01 (demonstrating exact cents)
    currency: 'USD',
    status: 'active',
    created_at: new Date(Date.now() - 86400000 * 7).toISOString(),
    installments: [
      {
        id: 'inst-mob-1',
        order_id: 'ord-mob-001',
        installment_number: 1,
        amount: 2501, // downpayment with 1 cent remainder
        currency: 'USD',
        due_date: new Date(Date.now() - 86400000 * 7).toISOString(),
        status: 'paid',
        attempt_count: 1,
        next_retry_at: null,
        paid_at: new Date(Date.now() - 86400000 * 7).toISOString(),
      },
      {
        id: 'inst-mob-2',
        order_id: 'ord-mob-001',
        installment_number: 2,
        amount: 2500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 7).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
      },
      {
        id: 'inst-mob-3',
        order_id: 'ord-mob-001',
        installment_number: 3,
        amount: 2500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 21).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
      },
      {
        id: 'inst-mob-4',
        order_id: 'ord-mob-001',
        installment_number: 4,
        amount: 2500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 35).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
      },
    ],
  },
  {
    id: 'ord-mob-002',
    client_id: 'tech_gear_direct',
    customer_account_id: 'cust-uuid-4444',
    merchant_account_id: 'merch-uuid-3333',
    total_amount: 18000, // $180.00
    currency: 'USD',
    status: 'active',
    created_at: new Date(Date.now() - 86400000 * 20).toISOString(),
    installments: [
      {
        id: 'inst-mob-2-1',
        order_id: 'ord-mob-002',
        installment_number: 1,
        amount: 4500,
        currency: 'USD',
        due_date: new Date(Date.now() - 86400000 * 20).toISOString(),
        status: 'paid',
        attempt_count: 1,
        next_retry_at: null,
        paid_at: new Date(Date.now() - 86400000 * 20).toISOString(),
      },
      {
        id: 'inst-mob-2-2',
        order_id: 'ord-mob-002',
        installment_number: 2,
        amount: 4500,
        currency: 'USD',
        due_date: new Date(Date.now() - 86400000 * 6).toISOString(),
        status: 'paid',
        attempt_count: 1,
        next_retry_at: null,
        paid_at: new Date(Date.now() - 86400000 * 6).toISOString(),
      },
      {
        id: 'inst-mob-2-3',
        order_id: 'ord-mob-002',
        installment_number: 3,
        amount: 4500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 8).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
      },
      {
        id: 'inst-mob-2-4',
        order_id: 'ord-mob-002',
        installment_number: 4,
        amount: 4500,
        currency: 'USD',
        due_date: new Date(Date.now() + 86400000 * 22).toISOString(),
        status: 'pending',
        attempt_count: 0,
        next_retry_at: null,
        paid_at: null,
      },
    ],
  },
]

const initialStatements: CustomerStatementEntry[] = [
  {
    id: 'stmt-001',
    description: 'Down payment: Nordic Store #ord-mob-001',
    amount: 2501,
    direction: 'DEBIT',
    currency: 'USD',
    date: new Date(Date.now() - 86400000 * 7).toISOString(),
  },
  {
    id: 'stmt-002',
    description: 'Cuota #2: Tech Gear #ord-mob-002',
    amount: 4500,
    direction: 'DEBIT',
    currency: 'USD',
    date: new Date(Date.now() - 86400000 * 6).toISOString(),
  },
]

class MobileApiClient {
  private orders: Order[] = [...initialOrders]
  private statements: CustomerStatementEntry[] = [...initialStatements]

  async getCustomerOrders(): Promise<Order[]> {
    return this.orders
  }

  async getStatement(): Promise<CustomerStatementEntry[]> {
    return this.statements
  }

  async payInstallment(installmentId: string): Promise<void> {
    for (const ord of this.orders) {
      const inst = ord.installments.find((i) => i.id === installmentId)
      if (inst) {
        inst.status = 'paid'
        inst.paid_at = new Date().toISOString()
        inst.attempt_count += 1

        this.statements.unshift({
          id: `stmt-${Date.now()}`,
          description: `Cuota #${inst.installment_number}: Order #${ord.id.slice(0, 8)}`,
          amount: inst.amount,
          direction: 'DEBIT',
          currency: inst.currency,
          date: new Date().toISOString(),
        })

        if (ord.installments.every((i) => i.status === 'paid')) {
          ord.status = 'completed'
        }
        return
      }
    }
  }
}

export const mobileApi = new MobileApiClient()
