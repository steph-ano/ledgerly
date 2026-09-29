import React from 'react'
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
} from 'react-native'
import { Colors } from '../theme/colors'
import type { Order, Installment } from '../types'

interface HomeScreenProps {
  orders: Order[]
  onSelectInstallment: (inst: Installment, order: Order) => void
}

export const HomeScreen: React.FC<HomeScreenProps> = ({
  orders,
  onSelectInstallment,
}) => {
  // Find next upcoming pending installment
  let nextDue: { inst: Installment; order: Order } | null = null
  for (const ord of orders) {
    const pending = ord.installments.find((i) => i.status === 'pending' || i.status === 'retrying')
    if (pending) {
      if (!nextDue || new Date(pending.due_date) < new Date(nextDue.inst.due_date)) {
        nextDue = { inst: pending, order: ord }
      }
    }
  }

  // Calculate total customer remaining balance
  let totalRemainingCents = 0
  for (const ord of orders) {
    for (const inst of ord.installments) {
      if (inst.status !== 'paid') {
        totalRemainingCents += inst.amount
      }
    }
  }

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      {/* Header Account Summary */}
      <View style={styles.balanceCard}>
        <Text style={styles.balanceLabel}>TOTAL OUTSTANDING BNPL BALANCE</Text>
        <Text style={styles.balanceValue}>
          ${(totalRemainingCents / 100).toFixed(2)}
          <Text style={styles.currency}> USD</Text>
        </Text>
        <Text style={styles.balanceSubtext}>
          4 Bi-Weekly Installments • 0% APR • Zero Hidden Fees
        </Text>
      </View>

      {/* Next Upcoming Due Installment Banner */}
      {nextDue && (
        <View style={styles.nextDueCard}>
          <View style={styles.nextDueHeader}>
            <View style={styles.badgePulse}>
              <Text style={styles.badgePulseText}>UPCOMING</Text>
            </View>
            <Text style={styles.nextDueDate}>
              Due: {new Date(nextDue.inst.due_date).toLocaleDateString()}
            </Text>
          </View>

          <View style={styles.nextDueBody}>
            <View>
              <Text style={styles.nextDueTitle}>
                Cuota #{nextDue.inst.installment_number} • {nextDue.order.client_id}
              </Text>
              <Text style={styles.nextDueAmount}>
                ${(nextDue.inst.amount / 100).toFixed(2)}
              </Text>
            </View>

            <TouchableOpacity
              style={styles.payNowBtn}
              onPress={() => onSelectInstallment(nextDue!.inst, nextDue!.order)}
              activeOpacity={0.8}
            >
              <Text style={styles.payNowBtnText}>Pay Early</Text>
            </TouchableOpacity>
          </View>
        </View>
      )}

      {/* Active Purchases List */}
      <Text style={styles.sectionTitle}>Active Purchases (Pay-in-4)</Text>

      {orders.map((order) => {
        const paidCount = order.installments.filter((i) => i.status === 'paid').length

        return (
          <View key={order.id} style={styles.orderCard}>
            <View style={styles.orderTop}>
              <View>
                <Text style={styles.merchantTitle}>{order.client_id}</Text>
                <Text style={styles.orderIdText}>#{order.id}</Text>
              </View>
              <Text style={styles.orderTotalAmount}>
                ${(order.total_amount / 100).toFixed(2)}
              </Text>
            </View>

            <Text style={styles.progressText}>{paidCount} of 4 Installments Settled</Text>

            {/* Installment Nodes */}
            <View style={styles.installmentsRow}>
              {order.installments.map((inst) => {
                const isPaid = inst.status === 'paid'
                return (
                  <TouchableOpacity
                    key={inst.id}
                    style={[
                      styles.instNode,
                      isPaid ? styles.instNodePaid : styles.instNodePending,
                    ]}
                    onPress={() => !isPaid && onSelectInstallment(inst, order)}
                    disabled={isPaid}
                  >
                    <Text
                      style={[
                        styles.instNodeNum,
                        isPaid ? styles.textPaid : styles.textPending,
                      ]}
                    >
                      {inst.installment_number}
                    </Text>
                    <Text style={styles.instNodeAmount}>
                      ${(inst.amount / 100).toFixed(0)}
                    </Text>
                    <Text style={styles.instNodeStatus}>
                      {isPaid ? 'PAID' : 'DUE'}
                    </Text>
                  </TouchableOpacity>
                )
              })}
            </View>
          </View>
        )
      })}
    </ScrollView>
  )
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.bgDeep,
  },
  content: {
    padding: 18,
    paddingBottom: 40,
  },
  balanceCard: {
    backgroundColor: Colors.bgSurface,
    borderRadius: 18,
    padding: 20,
    borderWidth: 1,
    borderColor: Colors.border,
    marginBottom: 20,
  },
  balanceLabel: {
    color: Colors.textDim,
    fontSize: 11,
    fontWeight: '700',
    letterSpacing: 0.8,
    marginBottom: 6,
  },
  balanceValue: {
    color: Colors.textMain,
    fontSize: 34,
    fontWeight: '800',
  },
  currency: {
    fontSize: 16,
    color: Colors.cyan,
    fontWeight: '600',
  },
  balanceSubtext: {
    color: Colors.textMuted,
    fontSize: 12,
    marginTop: 6,
  },
  nextDueCard: {
    backgroundColor: Colors.bgElevated,
    borderRadius: 16,
    padding: 16,
    borderWidth: 1,
    borderColor: 'rgba(0, 242, 254, 0.35)',
    marginBottom: 24,
  },
  nextDueHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 10,
  },
  badgePulse: {
    backgroundColor: 'rgba(0, 242, 254, 0.15)',
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: 12,
  },
  badgePulseText: {
    color: Colors.cyan,
    fontSize: 10,
    fontWeight: '800',
  },
  nextDueDate: {
    color: Colors.textDim,
    fontSize: 12,
  },
  nextDueBody: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  nextDueTitle: {
    color: Colors.textMain,
    fontSize: 15,
    fontWeight: '700',
  },
  nextDueAmount: {
    color: Colors.cyan,
    fontSize: 22,
    fontWeight: '800',
    marginTop: 2,
  },
  payNowBtn: {
    backgroundColor: Colors.cyan,
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 10,
  },
  payNowBtnText: {
    color: Colors.bgDeep,
    fontWeight: '800',
    fontSize: 13,
  },
  sectionTitle: {
    color: Colors.textMain,
    fontSize: 18,
    fontWeight: '800',
    marginBottom: 14,
  },
  orderCard: {
    backgroundColor: Colors.bgCard,
    borderRadius: 16,
    padding: 16,
    borderWidth: 1,
    borderColor: Colors.border,
    marginBottom: 14,
  },
  orderTop: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  merchantTitle: {
    color: Colors.textMain,
    fontSize: 16,
    fontWeight: '700',
  },
  orderIdText: {
    color: Colors.textDim,
    fontSize: 11,
    marginTop: 2,
  },
  orderTotalAmount: {
    color: Colors.textMain,
    fontSize: 17,
    fontWeight: '800',
  },
  progressText: {
    color: Colors.textMuted,
    fontSize: 12,
    marginVertical: 12,
  },
  installmentsRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: 8,
  },
  instNode: {
    flex: 1,
    paddingVertical: 10,
    borderRadius: 12,
    alignItems: 'center',
    borderWidth: 1,
  },
  instNodePaid: {
    backgroundColor: 'rgba(16, 185, 129, 0.08)',
    borderColor: 'rgba(16, 185, 129, 0.3)',
  },
  instNodePending: {
    backgroundColor: Colors.bgSurface,
    borderColor: Colors.border,
  },
  instNodeNum: {
    fontWeight: '800',
    fontSize: 14,
  },
  textPaid: {
    color: Colors.emerald,
  },
  textPending: {
    color: Colors.textDim,
  },
  instNodeAmount: {
    color: Colors.textMain,
    fontSize: 12,
    fontWeight: '700',
    marginTop: 2,
  },
  instNodeStatus: {
    fontSize: 9,
    fontWeight: '800',
    color: Colors.textDim,
    marginTop: 2,
  },
})
