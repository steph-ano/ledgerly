export type AccountType = 'customer' | 'merchant' | 'platform' | 'fees'

export interface Account {
  id: string
  client_id: string
  type: AccountType
  currency: string
  created_at: string
  balance?: {
    total_debits: number
    total_credits: number
    net_balance: number
    currency: string
  }
}

export type Direction = 'DEBIT' | 'CREDIT'

export interface Entry {
  id: string
  transaction_id: string
  account_id: string
  amount: number // cents
  direction: Direction
  currency: string
  created_at: string
}

export interface Transaction {
  id: string
  client_id: string
  idempotency_key: string
  description: string
  reversal_of?: string | null
  created_at: string
  entries: Entry[]
}

export type OrderStatus = 'pending' | 'active' | 'completed' | 'defaulted' | 'cancelled'
export type InstallmentStatus = 'pending' | 'retrying' | 'paid' | 'failed'

export interface Installment {
  id: string
  order_id: string
  installment_number: number
  amount: number // cents
  currency: string
  due_date: string
  status: InstallmentStatus
  attempt_count: number
  next_retry_at: string | null
  paid_at: string | null
  ledger_transaction_id: string | null
}

export interface Order {
  id: string
  client_id: string
  customer_account_id: string
  merchant_account_id: string
  total_amount: number // cents
  currency: string
  status: OrderStatus
  merchant_webhook_url: string
  created_at: string
  updated_at: string
  installments: Installment[]
}

export interface OutboxEvent {
  id: string
  event_type: 'order.created' | 'installment.paid' | 'order.completed' | 'order.defaulted'
  aggregate_type: string
  aggregate_id: string
  destination_url: string
  payload: Record<string, unknown>
  status: 'pending' | 'delivered' | 'failed'
  retry_count: number
  created_at: string
  delivered_at: string | null
}
