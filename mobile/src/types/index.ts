export type OrderStatus = 'pending' | 'active' | 'completed' | 'defaulted' | 'cancelled'
export type InstallmentStatus = 'pending' | 'retrying' | 'paid' | 'failed'

export interface Installment {
  id: string
  order_id: string
  installment_number: number
  amount: number // in cents
  currency: string
  due_date: string
  status: InstallmentStatus
  attempt_count: number
  next_retry_at: string | null
  paid_at: string | null
}

export interface Order {
  id: string
  client_id: string
  customer_account_id: string
  merchant_account_id: string
  total_amount: number // in cents
  currency: string
  status: OrderStatus
  created_at: string
  installments: Installment[]
}

export interface CustomerStatementEntry {
  id: string
  description: string
  amount: number // cents
  direction: 'DEBIT' | 'CREDIT'
  currency: string
  date: string
}
